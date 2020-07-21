package node

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	coreV1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	logger = logrus.New().WithField("component", "test")

	testNode = &coreV1.Node{
		ObjectMeta: k8smetav1.ObjectMeta{Name: "testNode"},
		Status: coreV1.NodeStatus{Conditions: []coreV1.NodeCondition{
			{Type: coreV1.NodeReady, Status: coreV1.ConditionTrue},
		}},
	}

	testPod = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: "testPod"},
		Status: coreV1.PodStatus{
			Phase: coreV1.PodRunning,
			PodIP: "10.10.10.1",
			ContainerStatuses: []coreV1.ContainerStatus{
				{Name: "container-1", Ready: true},
				{Name: "container-2", Ready: true},
				{Name: "container-3", Ready: true},
			},
		},
	}

	nodeID = "node-uuid"
)

func TestIsNodeServiceReady(t *testing.T) {
	isReady := isPodReady(stateComponents{testNode, testPod})
	assert.Equal(t, true, isReady)
}

func TestIsNodeServiceUnreadyPod(t *testing.T) {
	testPod.Status.ContainerStatuses[2].Ready = false
	isReady := isPodReady(stateComponents{testNode, testPod})
	assert.Equal(t, false, isReady)
	// restore
	testPod.Status.ContainerStatuses[2].Ready = false
}

func TestIsNodeServiceUnready(t *testing.T) {
	testNode.Status.Conditions[0].Status = coreV1.ConditionFalse
	isReady := isPodReady(stateComponents{testNode, testPod})
	assert.Equal(t, false, isReady)
	// restore
	testNode.Status.Conditions[0].Status = coreV1.ConditionTrue
}

func TestCalculatePodStatusReady(t *testing.T) {
	status := calculatePodStatus(nodeID, true, Ready, 0, false, logger)
	assert.Equal(t, Ready, status)
}

func TestCalculatePodStatusPermanentDownToReady(t *testing.T) {
	status := calculatePodStatus(nodeID, true, PermanentDown, 0, false, logger)
	assert.Equal(t, Ready, status)
}

func TestCalculatePodStatusUnready(t *testing.T) {
	status := calculatePodStatus(nodeID, false, Ready, UnreadyTimeout+1, false, logger)
	assert.Equal(t, Unready, status)
}

func TestCalculatePodStatusUnreadyTimeout(t *testing.T) {
	status := calculatePodStatus(nodeID, false, Unready, PermanentDownTimeout-1, false, logger)
	assert.Equal(t, Unready, status)
}

func TestCalculatePodStatusUnreadyToPermanentDown(t *testing.T) {
	status := calculatePodStatus(nodeID, false, Unready, PermanentDownTimeout+1, false, logger)
	assert.Equal(t, PermanentDown, status)
}

func TestCalculatePodStatusPermanentDown(t *testing.T) {
	status := calculatePodStatus(nodeID, false, PermanentDown, 0, false, logger)
	assert.Equal(t, PermanentDown, status)
}

func TestPodIsUnderStartupProtection(t *testing.T) {
	components := stateComponents{testNode, testPod}
	assert.False(t, podIsUnderStartupProtection(
		serviceState{Unready, time.Now(), true},
		components))
	assert.True(t, podIsUnderStartupProtection(
		serviceState{Unready, time.Now(), false},
		components))
}
