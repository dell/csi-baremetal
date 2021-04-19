package reservation

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1api "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	fc "github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	metrics "github.com/dell/csi-baremetal/pkg/metrics/common"
)

const (
	contextTimeoutSeconds = 60
)

// Controller to reconcile aviliablecapacityreservation custom resource
type Controller struct {
	client                 *k8s.KubeClient
	log                    *logrus.Entry
	capacityManagerBuilder capacityplanner.CapacityManagerBuilder
	featureChecker         fc.FeatureChecker
	annotationKey          string
}

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient, node ID and logrus logger
// Returns an instance of Controller
func NewController(client *k8s.KubeClient, log *logrus.Logger) *Controller {
	featureConfig := fc.NewFeatureConfig()
	// todo get rid of hard code
	featureConfig.Update(fc.FeatureNodeIDFromAnnotation, true)
	return &Controller{
		client: client,
		//crHelper: k8s.NewCRHelper(client, log),
		log:                    log.WithField("component", "ReservationController"),
		capacityManagerBuilder: &capacityplanner.DefaultCapacityManagerBuilder{},
		featureChecker:         featureConfig,
		// todo pass annotation key
		annotationKey: "",
	}
}

// SetupWithManager registers Controller to ControllerManager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&acrcrd.AvailableCapacityReservation{}).
		Complete(c)
}

// Reconcile reconciles ACR custom resources
func (c *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	defer metrics.ReconcileDuration.EvaluateDurationForType("reservation_controller")()

	ctx, cancelFn := context.WithTimeout(context.Background(), contextTimeoutSeconds*time.Second)
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
	case v1.ReservationRequested:
		// handle reservation request
		// convert to volumes
		volumes := make([]*v1api.Volume, len(reservationSpec.ReservationRequests))
		for i, request := range reservationSpec.ReservationRequests {
			capacity := request.CapacityRequest
			volumes[i] = &v1api.Volume{Id: capacity.Name, Size: capacity.Size, StorageClass: capacity.StorageClass}
		}

		// TODO: do not read all ACs and ACRs for each request: https://github.com/dell/csi-baremetal/issues/89
		acReader := capacityplanner.NewACReader(c.client, c.log, true)
		acrReader := capacityplanner.NewACRReader(c.client, c.log, true)
		reservedCapReader := capacityplanner.NewUnreservedACReader(c.log, acReader, acrReader)
		capManager := c.capacityManagerBuilder.GetCapacityManager(c.log, reservedCapReader)

		requestedNodes := reservationSpec.NodeRequests.Requested
		placingPlan, err := capManager.PlanVolumesPlacing(ctx, volumes, requestedNodes)
		if err != nil {
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
				c.log.Infof("Matched node Id: %s", id)
			}
		}

		if len(matchedNodes) != 0 {
			reservationHelper := capacityplanner.NewReservationHelper(c.log, c.client, acReader, acrReader)
			if err = reservationHelper.UpdateReservation(ctx, placingPlan, matchedNodes, reservation); err != nil {
				c.log.Errorf("Failed to update reservation: %s", err.Error())
				return ctrl.Result{Requeue: true}, err
			}
		} else {
			// reject reservation
			reservation.Spec.Status = v1.ReservationRejected
			if err := c.client.UpdateCR(ctx, reservation); err != nil {
				c.log.Errorf("Unable to reject reservation %s: %v", reservation.Name, err)
				return ctrl.Result{Requeue: true}, err
			}
		}
		return ctrl.Result{}, nil
	case v1.ReservationConfirmed:
		// todo handle
		return ctrl.Result{}, nil
	case v1.ReservationRejected:
		// todo handle
		return ctrl.Result{}, nil
	case v1.ReservationCancelled:
		// todo handle
		return ctrl.Result{}, nil
	default:
		// todo handle
		return ctrl.Result{}, nil
	}
}
