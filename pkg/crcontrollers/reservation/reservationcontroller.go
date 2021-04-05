package reservation

import (
	"context"
	v1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	fc "github.com/dell/csi-baremetal/pkg/base/featureconfig"
	annotations "github.com/dell/csi-baremetal/pkg/crcontrollers/operator/common"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1api "github.com/dell/csi-baremetal/api/generated/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	metrics "github.com/dell/csi-baremetal/pkg/metrics/common"
)

const (
	contextTimeoutSeconds = 60
)

// Controller to reconcile aviliablecapacityreservation custom resource
type Controller struct {
	client *k8s.KubeClient
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
		featureChecker: featureConfig,
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
		log.Errorf("Failed to read available capacity reservation %s CR", name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Infof("Reservation changed: %v", reservation)
	switch reservation.Spec.Status{
	case v1.ReservationRequested:
		// not an error - reservation requested
		// convert to volumes
		volumes := make([]*v1api.Volume, len(reservation.Spec.Requests))
		for i, request := range reservation.Spec.Requests {
			volumes[i] = &v1api.Volume{Id: request.Name, Size: request.Size, StorageClass: request.StorageClass}
		}

		// convert to nodes
		/*nodes := make([]*corev1.Node, len(reservation.Spec.Nodes))
		for i, node := range reservation.Spec.Nodes {
			if err := c.client.Get(ctx, client.ObjectKey{Name: node.Id}, nodes[i]); err != nil {
				log.Errorf("Failed to read node %s: %s", node.Id, err)
				return ctrl.Result{Requeue: true}, err
			}
		}*/

		// TODO: do not read all ACs and ACRs for each request: https://github.com/dell/csi-baremetal/issues/89
		acReader := capacityplanner.NewACReader(c.client, c.log, true)
		acrReader := capacityplanner.NewACRReader(c.client, c.log, true)
		reservedCapReader := capacityplanner.NewUnreservedACReader(c.log, acReader, acrReader)
		capManager := c.capacityManagerBuilder.GetCapacityManager(c.log, reservedCapReader)

		nodeList := &corev1.NodeList{}
		if err := c.client.ReadList(ctx, nodeList); err != nil {
			log.Errorf("Failed to get list of the nodes: %s", err)
			return ctrl.Result{Requeue: true}, err
		}

		// todo it might have negative impact on scheduling... need to think about this...
		idToNodeMap := map[string]*corev1.Node{}
		for _, node := range nodeList.Items {
			nodeId, err := annotations.GetNodeID(&node, c.annotationKey, c.featureChecker)
			if err != nil {
				c.log.Errorf("failed to get NodeID: %s", err)
			}
			idToNodeMap[nodeId] = &node
		}

		// todo pass requested nodes to the capacity manager for placing
		placingPlan, err := capManager.PlanVolumesPlacing(ctx, volumes, idToNodeMap)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		var matchedNodes []corev1.Node
		for id, node := range idToNodeMap {
			if placingPlan == nil {
				continue
			}

			placingForNode := placingPlan.GetVolumesToACMapping(id)
			if placingForNode == nil {
				continue
			}
			matchedNodes = append(matchedNodes, *node)
		}
		if len(matchedNodes) != 0 {
			reservationHelper := capacityplanner.NewReservationHelper(c.log, c.client, acReader, acrReader)
			if err = reservationHelper.CreateReservation(ctx, placingPlan, matchedNodes, reservation); err != nil {
				c.log.Errorf("failed to create reservation: %s", err.Error())
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
