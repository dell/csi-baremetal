package node

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

// constants for state monitoring
const (
	// when initializing or unable to detect state
	Unknown = 0
	// pod is ready
	Ready = 1
	// pod is in unready state for > X seconds, but < Y seconds
	Unready = 2
	// pod is in unready state for > Y seconds
	PermanentDown = 3
)

// timeout constant seconds
const (
	UnreadyTimeout       = 60
	PermanentDownTimeout = 120
	SleepBeforeNextPoll  = 30
	// StartupProtectionMultiplier will be applied to Timeouts if POD is under startup protection
	StartupProtectionMultiplier = 5
)

const (
	// todo need to use app label instead
	svcPodsMask = "baremetal-csi-node"
)

// ServicesStateMonitor contains methods to get Node service health state
type ServicesStateMonitor struct {
	// client to periodically obtain list of node services
	client *k8s.KubeClient
	// logger. Can't we initialize it inside?
	log *logrus.Entry
	// helper to work with custom resource definition
	crHelper *k8s.CRHelper
	// nodeX ID -> <Ready/Unready/PermanentDown>
	nodeHealthMap map[string]*serviceState
	// mutex to protect map access
	lock *sync.RWMutex
}

// serviceState keeps current state of the node service and last timestamp when its changed
type serviceState struct {
	status int
	time   time.Time
	// if true, then POD was seen in ready state some time ago, we should not apply startupProteciton to it
	wasReady bool
}

type stateComponents struct {
	node *coreV1.Node
	pod  *coreV1.Pod
}

// NewNodeServicesStateMonitor instantiates health monitor
func NewNodeServicesStateMonitor(client *k8s.KubeClient, logger *logrus.Logger) *ServicesStateMonitor {
	return &ServicesStateMonitor{
		client:        client,
		log:           logger.WithField("component", "ServicesStateMonitor"),
		crHelper:      k8s.NewCRHelper(client, logger),
		nodeHealthMap: make(map[string]*serviceState),
		lock:          &sync.RWMutex{},
	}
}

// Run inits map node id -> health
func (n *ServicesStateMonitor) Run() {
	// spawn routine to watch for node service status
	go n.pollPodsStatus()
	// spawn routine to update custom resources
	go n.updateCRs()
}

// GetUnreadyPods obtains list of Unready pods. Blocking for read
func (n *ServicesStateMonitor) GetUnreadyPods() []string {
	unready := make([]string, 0)

	// read lock
	n.lock.RLock()
	for name, state := range n.nodeHealthMap {
		if state.status != Unready {
			unready = append(unready, name)
		}
	}
	n.lock.RUnlock()

	return unready
}

// GetReadyPods obtains list of Ready pods. Blocking for read
func (n *ServicesStateMonitor) GetReadyPods() []string {
	ready := make([]string, 0)

	n.lock.RLock()
	for name, state := range n.nodeHealthMap {
		if state.status == Ready {
			ready = append(ready, name)
		}
	}
	n.lock.RUnlock()

	return ready
}

// UpdateNodeHealthCache check if node service pods are ready and update nodeHealthMap
func (n *ServicesStateMonitor) UpdateNodeHealthCache() {
	log := n.log.WithFields(logrus.Fields{"method": "UpdateNodeHealthCache"})
	podToNodeMap, err := n.getPodToNodeList()
	// obtain write lock
	n.lock.Lock()
	defer n.lock.Unlock()
	currentTime := time.Now()
	if err == nil {
		for nodeID, podAndNode := range podToNodeMap {
			// check pod status
			isReady := isPodReady(podAndNode)
			var (
				state   *serviceState
				isExist bool
			)
			// todo when node is removed from cluster?
			if state, isExist = n.nodeHealthMap[nodeID]; !isExist {
				state = &serviceState{status: Unknown, time: currentTime}
				// add pod to the map - no need to print warning message here since this is cache initialization
				n.nodeHealthMap[nodeID] = state
			}
			if isReady {
				state.wasReady = true
			}
			// calculate new status
			timePassed := currentTime.Sub(state.time).Seconds()
			newStatus := calculatePodStatus(nodeID, isReady, state.status, timePassed, podIsUnderStartupProtection(*state, podAndNode), n.log)
			// update when status changed
			if newStatus != state.status {
				state.status = newStatus
				state.time = currentTime
			}
		}
	} else {
		log.Errorf("Unable to obtain list of the pods. Change health to Unknown for all pods")
		for _, state := range n.nodeHealthMap {
			state.status = Unknown
			state.time = currentTime
		}
	}
}

// todo how will this method scale up with hundreds of nodes?
// todo instead of polling we need to watch for events once liveness probes are ready
func (n *ServicesStateMonitor) pollPodsStatus() {
	// infinite loop
	for {
		n.UpdateNodeHealthCache()
		// sleep before next poll
		time.Sleep(SleepBeforeNextPoll * time.Second)
	}
}

// getPodToNodeList generates map: nodeID -> {node, pod}
func (n *ServicesStateMonitor) getPodToNodeList() (map[string]stateComponents, error) {
	log := n.log.WithFields(logrus.Fields{"method": "getPodToNodeList"})

	// todo should be configured
	timeout := 30 * time.Second
	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	log.Trace("Obtaining list of pods and nodes...")
	// get list of the pods
	pods, err := n.client.GetPods(ctx, svcPodsMask)
	if err != nil {
		return nil, errors.New("unable to obtain list of the pods")
	}

	// get list of nodes
	nodes, err := n.client.GetNodes(ctx)
	if err != nil {
		return nil, errors.New("unable to obtain list of the nodes")
	}

	stateComponentsMap := make(map[string]stateComponents)
	// expect to have on pod per node
	for _, pod := range pods {
		// pod nodeID is a key in map
		// should be node UUID
		var podNode *coreV1.Node
		for _, node := range nodes {
			for _, address := range node.Status.Addresses {
				// match pod to node by host name
				if address.Type == coreV1.NodeHostName {
					if address.Address == pod.Spec.NodeName {
						// nolint: scopelint
						podNode = &node
						break
					}
				}
			}
			if podNode != nil {
				// todo need to remove node from the list here
				break
			}
		}

		if podNode == nil {
			log.Fatalf("Unable to find node for pod %s", pod.Name)
		}
		nodeID := string(podNode.ObjectMeta.UID)
		stateComponentsMap[nodeID] = stateComponents{podNode, pod}
	}

	return stateComponentsMap, nil
}

// calculate pod status based on current, previous state and timestamp
func calculatePodStatus(name string, isReady bool, status int, timePassed float64,
	startupProtection bool, logger *logrus.Entry) int {
	log := logger.WithFields(logrus.Fields{"method": "calculatePodStatus"})
	if isReady {
		// return to Ready state right away
		if status != Ready {
			log.Infof("Node service %s is ready", name)
		}
		return Ready
	}
	multiplier := 1
	if startupProtection {
		multiplier = StartupProtectionMultiplier
	}
	// pod is unready. need to decide whether it's unready or permanent down
	switch status {
	case Unknown, Ready:
		// todo how to be with polling interval?
		// todo this is not fair
		if timePassed > float64(UnreadyTimeout*multiplier) {
			log.Warningf("Node service %s is unready", name)
			return Unready
		}
	case Unready:
		if timePassed > float64(PermanentDownTimeout*multiplier) {
			log.Errorf("Node service %s is in PermanentDown state", name)
			return PermanentDown
		}
	case PermanentDown:
		// do nothing
	}

	// return current status otherwise
	return status
}

// updateCRs updates corresponding CRs when node service is Unready/PermanentDown
// blocking for read access
func (n *ServicesStateMonitor) updateCRs() {
	log := n.log.WithFields(logrus.Fields{"method": "updateCRs"})
	for {
		unready := make([]string, 0)
		permanentDown := make([]string, 0)

		// obtain read lock
		n.lock.RLock()
		for id, state := range n.nodeHealthMap {
			switch state.status {
			case Unready:
				unready = append(unready, id)
			case PermanentDown:
				// add to unready to remove AC if they still exist
				unready = append(unready, id)
				// update drives and volumes statuses
				permanentDown = append(permanentDown, id)
			}
		}
		n.lock.RUnlock()

		// todo possible issue with racing on bootstrap - AK8S-1129
		// delete AC for unready
		for _, id := range unready {
			err := n.crHelper.DeleteACsByNodeID(id)
			if err != nil {
				log.Tracef("Error occurred during AC deletion: %s", err)
			}
		}
		// mark disks as OFFLINE when permanentDown
		// mark volumes as MISSING
		for _, id := range permanentDown {
			err := n.crHelper.UpdateDrivesStatusOnNode(id, apiV1.DriveStatusOffline)
			if err != nil {
				log.Tracef("Error occurred during drives status update: %s", err)
			}
			// todo create JIRA to return volume back to OPERATIVE state when node is up
			err = n.crHelper.UpdateVolumesOpStatusOnNode(id, apiV1.OperationalStatusMissing)
			if err != nil {
				log.Tracef("Error occurred during volumes status update: %s", err)
			}
		}

		// sleep before next poll
		time.Sleep(SleepBeforeNextPoll * time.Second)
	}
}

// isPodReady checks pod and node states
// Receives references to pod and node
// Returns true when pod is ready and false otherwise
func isPodReady(components stateComponents) bool {
	pod := components.pod
	// check readiness first
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready {
			// if one container not ready the whole pod is unready
			return false
		}
	}

	node := components.node
	for _, condition := range node.Status.Conditions {
		if condition.Type == coreV1.NodeReady {
			return condition.Status == coreV1.ConditionTrue
		}
	}
	// return false if corresponding condition not found
	return false
}

// podIsUnderStartupProtection checks if startupProtection can be applied to the node's POD
// If POD on node was never in "Ready" state during the controller's POD lifetime, then we will increase timeouts to
// give the node's POD additional time to boot and create all resources.
func podIsUnderStartupProtection(state serviceState, podNode stateComponents) bool {
	// if we detect that POD already was online we should handle it as usual
	if state.wasReady {
		return false
	}
	// only PODs in Pending and Running state can be under startup protection
	if !(podNode.pod.Status.Phase == coreV1.PodPending ||
		podNode.pod.Status.Phase == coreV1.PodRunning) {
		return false
	}
	// POD should have no terminated containers to be under startup protection
	for _, container := range podNode.pod.Status.ContainerStatuses {
		if container.State.Terminated != nil {
			return false
		}
	}
	return true
}
