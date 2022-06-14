package common

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	fc "github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

var (
	testNS     = ""
	testCtx    = context.Background()
	testLogger = logrus.New()

	nodeName = "node"
	nodeUID  = "11-22"
	testNode = coreV1.Node{
		ObjectMeta: metaV1.ObjectMeta{
			Name:        nodeName,
			UID:         types.UID(nodeUID),
			Namespace:   testNS,
			Annotations: map[string]string{},
		},
	}
	bmNode = nodecrd.Node{
		ObjectMeta: metaV1.ObjectMeta{
			Name:        nodeName,
			UID:         types.UID(nodeUID),
			Namespace:   testNS,
			Annotations: map[string]string{},
		},
	}

	annotationKey   = "example/uuid"
	annotationValue = "1111-2222-3333-4444"
)

func TestObtainNodeIDWithRetries(t *testing.T) {
	t.Run("Not found", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		annotationSrv := New(
			k8sClient,
			fc.NewFeatureConfig(),
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)
		nodeID, err := annotationSrv.ObtainNodeID(nodeName, "app=baremetal-csi")
		assert.NotNil(t, err)
		assert.Equal(t, nodeID, "")
	})

	t.Run("Obtained", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		featureConf := fc.NewFeatureConfig()
		featureConf.Update(fc.FeatureNodeIDFromAnnotation, true)
		featureConf.Update(fc.FeatureExternalAnnotationForNode, true)
		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)
		node := bmNode.DeepCopy()
		node.Annotations[annotationKey] = annotationValue
		node.SetLabels(map[string]string{"app": "baremetal-csi"})

		assert.Nil(t, k8sClient.Create(testCtx, node))

		nodeID, err := annotationSrv.ObtainNodeID(nodeName, annotationKey)
		assert.Equal(t, annotationValue, nodeID)
		assert.Nil(t, err)
	})
}

func TestGetNodeID(t *testing.T) {
	t.Run("All features disabled", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		annotationSrv := New(
			k8sClient,
			fc.NewFeatureConfig(),
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)

		nodeID, err := annotationSrv.GetNodeID(&bmNode, annotationKey, "")
		assert.Equal(t, nodeUID, nodeID)
		assert.Nil(t, err)
	})

	t.Run("Default annotation feature", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		featureConf := fc.NewFeatureConfig()
		featureConf.Update(fc.FeatureNodeIDFromAnnotation, true)
		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)

		node := bmNode.DeepCopy()
		node.SetLabels(map[string]string{"app": "baremetal-csi"})
		node.Annotations[DeafultNodeIDAnnotationKey] = annotationValue

		nodeID, err := annotationSrv.GetNodeID(node, annotationKey, "app=baremetal-csi")
		assert.Equal(t, annotationValue, nodeID)
		assert.Nil(t, err)
	})

	t.Run("Default annotation feature wrong labels", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		featureConf := fc.NewFeatureConfig()
		featureConf.Update(fc.FeatureNodeIDFromAnnotation, true)
		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)

		node := bmNode.DeepCopy()
		node.SetLabels(map[string]string{"app": "baremetal-csi"})
		node.Annotations[DeafultNodeIDAnnotationKey] = annotationValue

		nodeID, err := annotationSrv.GetNodeID(node, annotationKey, "app=csi-baremetal")
		assert.Equal(t, "", nodeID)
		assert.Nil(t, err)
	})

	t.Run("Custom annotation feature", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		featureConf := fc.NewFeatureConfig()
		featureConf.Update(fc.FeatureNodeIDFromAnnotation, true)
		featureConf.Update(fc.FeatureExternalAnnotationForNode, true)
		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)

		node := bmNode.DeepCopy()
		node.Annotations[annotationKey] = annotationValue
		node.SetLabels(map[string]string{"app": "baremetal-csi"})

		nodeID, err := annotationSrv.GetNodeID(node, annotationKey, "app=baremetal-csi")
		assert.Equal(t, annotationValue, nodeID)
		assert.Nil(t, err)
	})

	t.Run("Annotation is not set", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		featureConf := fc.NewFeatureConfig()
		featureConf.Update(fc.FeatureNodeIDFromAnnotation, true)
		featureConf.Update(fc.FeatureExternalAnnotationForNode, true)
		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)
		node := bmNode.DeepCopy()
		node.SetLabels(map[string]string{"app": "baremetal-csi"})

		_, err = annotationSrv.GetNodeID(node, annotationKey, "app=baremetal-csi")
		assert.NotNil(t, err)
	})

	t.Run("Custom annotation feature is enabled, but annotationKey is empty", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		featureConf := fc.NewFeatureConfig()
		featureConf.Update(fc.FeatureNodeIDFromAnnotation, true)
		featureConf.Update(fc.FeatureExternalAnnotationForNode, true)
		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)

		node := bmNode.DeepCopy()
		node.Annotations[annotationKey] = annotationValue
		node.SetLabels(map[string]string{"app": "baremetal-csi"})

		_, err = annotationSrv.GetNodeID(node, "", "app=baremetal-csi")
		assert.NotNil(t, err)
	})
}

func TestGetNodeIDFromCRD(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		featureConf := fc.NewFeatureConfig()
		featureConf.Update(fc.FeatureNodeIDFromAnnotation, true)
		featureConf.Update(fc.FeatureExternalAnnotationForNode, true)
		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)

		node := bmNode.DeepCopy()
		node.Annotations[annotationKey] = annotationValue
		node.SetLabels(map[string]string{"app": "baremetal-csi"})

		assert.Nil(t, k8sClient.Create(testCtx, node))

		nodeID, err := annotationSrv.GetNodeIDFromCRD(testCtx, nodeName, annotationKey, "app=baremetal-csi")
		assert.Equal(t, annotationValue, nodeID)
		assert.Nil(t, err)
	})

	t.Run("Node is not exist", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		annotationSrv := New(
			k8sClient,
			fc.NewFeatureConfig(),
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)

		_, err = annotationSrv.GetNodeIDFromCRD(testCtx, nodeName, annotationKey, "app=baremetal-csi")
		assert.NotNil(t, err)
	})
}

func TestGetNodeIDFromK8s(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)

		featureConf := fc.NewFeatureConfig()
		featureConf.Update(fc.FeatureNodeIDFromAnnotation, true)
		featureConf.Update(fc.FeatureExternalAnnotationForNode, true)

		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)

		node := testNode.DeepCopy()
		node.Annotations[annotationKey] = annotationValue
		node.SetLabels(map[string]string{"app": "baremetal-csi"})

		assert.Nil(t, k8sClient.Create(testCtx, node))

		nodeID, err := annotationSrv.GetNodeIDFromK8s(testCtx, nodeName, annotationKey, "app=baremetal-csi")
		assert.Equal(t, annotationValue, nodeID)
		assert.Nil(t, err)
	})

	t.Run("Node is not exist", func(t *testing.T) {
		k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
		assert.Nil(t, err)
		featureConf := fc.NewFeatureConfig()
		annotationSrv := New(
			k8sClient,
			featureConf,
			testLogger,
			WithRetryDelay(1*time.Second),
			WithRetryNumber(1),
		)
		_, err = annotationSrv.GetNodeIDFromK8s(testCtx, nodeName, annotationKey, "app=baremetal-csi")
		assert.NotNil(t, err)
	})
}

func TestLabelStringToKV(t *testing.T) {
	for _, tt := range []struct {
		name    string
		payload string
		result  map[string]string
	}{
		{
			name:    "valid",
			payload: "app=baremetal",
			result:  map[string]string{"key": "app", "value": "baremetal"},
		},
		{
			name:    "invalid",
			payload: "",
			result:  map[string]string{"key": "", "value": ""},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			key, value := labelStringToKV(tt.payload)
			assert.Equal(t, tt.result["key"], key)
			assert.Equal(t, tt.result["value"], value)
		})
	}
}
