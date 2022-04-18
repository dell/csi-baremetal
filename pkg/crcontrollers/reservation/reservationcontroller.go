package reservation

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	v1api "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	baseerr "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	metrics "github.com/dell/csi-baremetal/pkg/metrics/common"
)

const (
	contextTimeoutSeconds = 60

	// reservation parameters
	fastDelayEnv       = "RESERVATION_FAST_DELAY"
	slowDelayEnv       = "RESERVATION_SLOW_DELAY"
	maxFastAttemptsEnv = "RESERVATION_MAX_FAST_ATTEMPTS"

	defaulFastDelay       = 1500 * time.Millisecond
	defaulSlowDelay       = 12 * time.Second
	defaulMaxFastAttempts = 30
)

// Controller to reconcile aviliablecapacityreservation custom resource
type Controller struct {
	client                 *k8s.KubeClient
	log                    *logrus.Entry
	capacityManagerBuilder capacityplanner.CapacityManagerBuilder
	fastDelay              time.Duration
	slowDelay              time.Duration
	maxFastAttempts        uint64
}

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient, node ID and logrus logger
// Returns an instance of Controller
func NewController(client *k8s.KubeClient, log *logrus.Logger, sequentialLVGReservation bool) *Controller {
	c := &Controller{
		client:                 client,
		log:                    log.WithField("component", "ReservationController"),
		capacityManagerBuilder: &capacityplanner.DefaultCapacityManagerBuilder{SequentialLVGReservation: sequentialLVGReservation},
	}
	c.setReservationParameters()
	return c
}

// SetupWithManager registers Controller to ControllerManager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&acrcrd.AvailableCapacityReservation{}).
		WithOptions(controller.Options{
			// Rater controls timeout between reconcile attempts (if result.Requeue is true)
			// Default RateLimiter is exponential, which leads to increasing timeout after 100 retries to 5+ mins
			// Many attempts are expected during processing ACRs with LVG Volumes, so we need to use linear RateLimiter
			// Here we make first maxAttempts retries with fastTimeout, and then it will be slowTimeout forever
			RateLimiter: workqueue.NewItemFastSlowRateLimiter(
				c.fastDelay,
				c.slowDelay,
				int(c.maxFastAttempts),
			),
		}).
		Complete(c)
}

// Reconcile reconciles ACR custom resources
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	defer metrics.ReconcileDuration.EvaluateDurationForType("reservation_controller")()

	ctx, cancelFn := context.WithTimeout(ctx, contextTimeoutSeconds*time.Second)
	defer cancelFn()

	// read name
	name := req.Name
	// customize logging
	log := c.log.WithFields(logrus.Fields{"method": "Reconcile", "name": name})

	// obtain corresponding reservation
	reservation := &acrcrd.AvailableCapacityReservation{}
	if err := c.client.ReadCR(ctx, name, "", reservation); err != nil {
		log.Warningf("Failed to read available capacity reservation %s CR", name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Debugf("Reservation changed: %v", reservation)
	return c.handleReservationUpdate(ctx, log, reservation)
}

func (c *Controller) handleReservationUpdate(ctx context.Context, log *logrus.Entry,
	reservation *acrcrd.AvailableCapacityReservation) (ctrl.Result, error) {
	reservationSpec := &reservation.Spec
	// check status
	status := reservationSpec.Status
	log.Infof("Reservation status: %s", status)

	switch status {
	case v1.MatchReservationStatus(v1.ReservationRequested):
		// handle reservation request
		// convert to volumes
		volumes := make([]*v1api.Volume, len(reservationSpec.ReservationRequests))
		for i, request := range reservationSpec.ReservationRequests {
			capacity := request.CapacityRequest
			volumes[i] = &v1api.Volume{Id: capacity.Name, Size: capacity.Size, StorageClass: capacity.StorageClass}
		}

		// TODO: do not read all ACs and ACRs for each request: https://github.com/dell/csi-baremetal/issues/89
		acReader := capacityplanner.NewACReader(c.client, log, true)
		acrReader := capacityplanner.NewACRReader(c.client, log, true)
		capManager := c.capacityManagerBuilder.GetCapacityManager(log, acReader, acrReader)

		requestedNodes := reservationSpec.NodeRequests.Requested
		placingPlan, err := capManager.PlanVolumesPlacing(ctx, volumes, requestedNodes)
		if err == baseerr.ErrorRejectReservationRequest {
			log.Warningf("Reservation request rejected due to another ACR in RESERVED state has request based on LVG")
			return ctrl.Result{Requeue: true}, nil
		}
		if err != nil {
			log.Errorf("Failed to create placing plan: %s", err.Error())
			return ctrl.Result{Requeue: true}, err
		}

		var matchedNodes []string
		if placingPlan != nil {
			for _, id := range requestedNodes {
				placingForNode := placingPlan.GetVolumesToACMapping(id)
				if placingForNode == nil {
					continue
				}
				matchedNodes = append(matchedNodes, id)
				log.Infof("Matched node Id: %s", id)
			}
		}

		if len(matchedNodes) != 0 {
			reservationHelper := capacityplanner.NewReservationHelper(c.log, c.client, acReader)
			if err = reservationHelper.UpdateReservation(ctx, placingPlan, matchedNodes, reservation); err != nil {
				log.Errorf("Failed to update reservation: %s", err.Error())
				return ctrl.Result{Requeue: true}, err
			}
		} else {
			// reject reservation
			reservation.Spec.Status = v1.MatchReservationStatus(v1.ReservationRejected)
			if err := c.client.UpdateCR(ctx, reservation); err != nil {
				log.Errorf("Unable to reject reservation %s: %v", reservation.Name, err)
				return ctrl.Result{Requeue: true}, err
			}
		}
		log.Infof("CR obtained")
		return ctrl.Result{}, nil
	default:
		log.Infof("CR is not in %s state", v1.ReservationRequested)
		return ctrl.Result{}, nil
	}
}

func (c *Controller) setReservationParameters() {
	var (
		fastDelayStr       = os.Getenv(fastDelayEnv)
		slowDelayStr       = os.Getenv(slowDelayEnv)
		maxFastAttemptsStr = os.Getenv(maxFastAttemptsEnv)

		fastDelay       time.Duration
		slowDelay       time.Duration
		maxFastAttempts uint64
		err             error
	)

	fastDelay, err = time.ParseDuration(fastDelayStr)
	if err != nil {
		c.log.Errorf("passed fastTimeout parameter %s is not parsable as time.Duration. Used defaul - %s", fastDelayStr, defaulFastDelay)
		fastDelay = defaulFastDelay
	}

	slowDelay, err = time.ParseDuration(slowDelayStr)
	if err != nil {
		c.log.Errorf("passed slowTimeout parameter %s is not parsable as time.Duration. Used defaul - %s", slowDelayStr, defaulSlowDelay)
		slowDelay = defaulSlowDelay
	}

	maxFastAttempts, err = strconv.ParseUint(maxFastAttemptsStr, 10, 64)
	if err != nil {
		c.log.Errorf("passed maxAttempts parameter %s is not parsable as uint. Used defaul - %d", maxFastAttemptsStr, defaulMaxFastAttempts)
		maxFastAttempts = defaulMaxFastAttempts
	}

	c.fastDelay = fastDelay
	c.slowDelay = slowDelay
	c.maxFastAttempts = maxFastAttempts
	c.log.Infof("Reservation controller parameters: fastTimeout - %s, slowTimeout - %s, maxAttempts - %d", fastDelay, slowDelay, maxFastAttempts)
}
