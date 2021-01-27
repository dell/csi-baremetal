package csibmnode

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	crdV1 "github.com/dell/csi-baremetal/api/v1"
	nodecrd "github.com/dell/csi-baremetal/api/v1/csibmnodecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/crcontrollers/csibmnode/common"
)

var (
	testNS     = "default"
	testCtx    = context.Background()
	testLogger = logrus.New()

	testCSIBMNode1 = nodecrd.CSIBMNode{
		TypeMeta: metaV1.TypeMeta{
			Kind:       crdV1.CSIBMNodeKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "csibmnode-1",
			Namespace: testNS,
		},
		Spec: api.CSIBMNode{
			UUID: "ffff-aaaa-bbbb",
			Addresses: map[string]string{
				string(coreV1.NodeHostName):   "node-1",
				string(coreV1.NodeInternalIP): "10.10.10.1",
			},
		},
	}

	testCSIBMNode2 = nodecrd.CSIBMNode{
		TypeMeta: metaV1.TypeMeta{
			Kind:       crdV1.CSIBMNodeKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "csibmnode-2",
			Namespace: testNS,
		},
		Spec: api.CSIBMNode{
			UUID: "1111-2222-3333",
			Addresses: map[string]string{
				string(coreV1.NodeHostName):   "node-2",
				string(coreV1.NodeInternalIP): "10.10.10.2",
			},
		},
	}

	osName    = "ubuntu"
	osVersion = "18.04"
	testNode1 = coreV1.Node{
		ObjectMeta: metaV1.ObjectMeta{
			Name:        "node-1",
			Namespace:   testNS,
			Annotations: map[string]string{},
			Labels:      map[string]string{}},
		Status: coreV1.NodeStatus{
			Addresses: convertCSIBMNodeAddrsToK8sNodeAddrs(testCSIBMNode1.Spec.Addresses),
		},
	}
	testNode2 = coreV1.Node{
		ObjectMeta: metaV1.ObjectMeta{
			Name:        "node-2",
			Namespace:   testNS,
			Annotations: map[string]string{},
			Labels:      map[string]string{}},
		Status: coreV1.NodeStatus{
			Addresses: convertCSIBMNodeAddrsToK8sNodeAddrs(testCSIBMNode2.Spec.Addresses),
			//NodeInfo: coreV1.NodeSystemInfo{OSImage: "Ubuntu 19.10"},
		},
	}
)

func TestNewCSIBMController(t *testing.T) {
	k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)

	t.Run("Node selector is empty", func(t *testing.T) {
		c, err := NewController("", k8sClient, testLogger)
		assert.Nil(t, err)
		assert.Nil(t, c.nodeSelector)
		assert.NotNil(t, c)
		assert.NotNil(t, c.cache)
		assert.NotNil(t, c.cache.bmToK8sNode)
		assert.NotNil(t, c.cache.k8sToBMNode)
	})

	t.Run("Node selector is provided", func(t *testing.T) {
		var (
			key   = "key"
			value = "value"
		)

		c, err := NewController("key:value", k8sClient, testLogger)
		assert.Nil(t, err)
		assert.NotNil(t, c)
		assert.NotNil(t, c.cache)
		assert.NotNil(t, c.cache.bmToK8sNode)
		assert.NotNil(t, c.cache.k8sToBMNode)
		assert.NotNil(t, c.nodeSelector)
		assert.Equal(t, label{key, value}, *c.nodeSelector)
	})

	t.Run("Node selector is wrong", func(t *testing.T) {
		c, err := NewController("key:dfdf:value", k8sClient, testLogger)
		assert.Nil(t, c)
		assert.NotNil(t, err)
	})

}

func Test_nodesCache(t *testing.T) {
	c := &nodesMapping{
		k8sToBMNode: make(map[string]string),
		bmToK8sNode: make(map[string]string),
	}

	k8sNode := "k8s-node"
	bmNode := "bm-node"

	c.put(k8sNode, bmNode)
	curr, ok := c.getK8sNodeName(bmNode)
	assert.True(t, ok)
	assert.Equal(t, k8sNode, curr)

	curr, ok = c.getCSIBMNodeName(k8sNode)
	assert.True(t, ok)
	assert.Equal(t, bmNode, curr)
}

func TestReconcile(t *testing.T) {
	t.Run("Reconcile for k8s node. Success", func(t *testing.T) {
		var (
			c      = setup(t)
			node   = testNode1.DeepCopy()
			bmNode = testCSIBMNode1.DeepCopy()
		)

		createObjects(t, c.k8sClient, bmNode, node)

		res, err := c.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Name: node.Name, Namespace: testNS}})
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		nodeCR := new(coreV1.Node)
		assert.Nil(t, c.k8sClient.ReadCR(testCtx, node.Name, "", nodeCR))

		val, ok := nodeCR.GetAnnotations()[nodeIDAnnotationKey]
		assert.True(t, ok)
		assert.Equal(t, bmNode.Spec.UUID, val)
	})

	t.Run("Reconcile for CSIBMNode. Success", func(t *testing.T) {
		var (
			c      = setup(t)
			node   = testNode1.DeepCopy() // annotation should be set for that object
			bmNode = testCSIBMNode1.DeepCopy()
		)

		createObjects(t, c.k8sClient, bmNode, node)

		res, err := c.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Name: bmNode.Name, Namespace: testNS}})
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		// read node obj
		nodeObj := new(coreV1.Node)
		assert.Nil(t, c.k8sClient.Get(testCtx, k8sCl.ObjectKey{Name: node.Name}, nodeObj))
		val, ok := nodeObj.GetAnnotations()[nodeIDAnnotationKey]
		assert.True(t, ok)
		assert.Equal(t, bmNode.Spec.UUID, val)
	})

	t.Run("Reconcile for nonexistent object", func(t *testing.T) {
		c := setup(t)
		res, err := c.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Name: "", Namespace: ""}})
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)
	})
}

func Test_reconcileForK8sNode(t *testing.T) {
	t.Run("CSIBMNode was created and annotation was set", func(t *testing.T) {
		var (
			c       = setup(t)
			k8sNode = testNode1.DeepCopy()
		)

		createObjects(t, c.k8sClient, k8sNode)

		res, err := c.reconcileForK8sNode(k8sNode)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		bmNodesList := &nodecrd.CSIBMNodeList{}
		assert.Nil(t, c.k8sClient.ReadList(testCtx, bmNodesList))
		assert.Equal(t, 1, len(bmNodesList.Items))
		bmNode := bmNodesList.Items[0]
		assert.Equal(t, len(bmNode.Spec.Addresses), c.matchedAddressesCount(&bmNode, k8sNode))

		assert.Nil(t, c.k8sClient.ReadCR(testCtx, k8sNode.Name, "", k8sNode))
		val, ok := k8sNode.GetAnnotations()[nodeIDAnnotationKey]
		assert.True(t, ok)
		assert.Equal(t, bmNode.Spec.UUID, val)
	})

	t.Run("K8s node addresses length is 0", func(t *testing.T) {
		var (
			c       = setup(t)
			k8sNode = testNode1.DeepCopy()
		)

		k8sNode.Status.Addresses = nil
		createObjects(t, c.k8sClient, k8sNode)

		res, err := c.reconcileForK8sNode(k8sNode)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "addresses are missing")
		assert.Equal(t, ctrl.Result{Requeue: false}, res)
	})

	t.Run("Unable to read corresponding CSIBMNode CR", func(t *testing.T) {
		var (
			c       = setup(t)
			k8sNode = testNode1.DeepCopy()
		)

		createObjects(t, c.k8sClient, k8sNode)
		c.cache.put(k8sNode.Name, "")

		res, err := c.reconcileForK8sNode(k8sNode)
		assert.NotNil(t, err)
		assert.Equal(t, ctrl.Result{Requeue: true}, res)
	})

	t.Run("There is CSIBMNode that partially match k8s node", func(t *testing.T) {
		var (
			c       = setup(t)
			k8sNode = testNode1.DeepCopy()
			bmNode  = testCSIBMNode1.DeepCopy()
		)

		k8sNode.Status.Addresses = []coreV1.NodeAddress{k8sNode.Status.Addresses[0]}
		createObjects(t, c.k8sClient, k8sNode, bmNode)

		res, err := c.reconcileForK8sNode(k8sNode)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		// read node obj
		nodeObj := new(coreV1.Node)
		assert.Nil(t, c.k8sClient.ReadCR(testCtx, k8sNode.Name, "", nodeObj))
		_, ok := nodeObj.GetAnnotations()[nodeIDAnnotationKey]
		assert.False(t, ok)
	})

	t.Run("More then one CSIBMNode CR match k8s node", func(t *testing.T) {
		var (
			c       = setup(t)
			k8sNode = testNode1.DeepCopy()
			bmNode1 = testCSIBMNode1.DeepCopy()
			bmNode2 = testCSIBMNode2.DeepCopy()
		)

		bmNode2.Spec.Addresses = bmNode1.Spec.Addresses
		createObjects(t, c.k8sClient, k8sNode, bmNode1, bmNode2)

		res, err := c.reconcileForK8sNode(k8sNode)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		// read node obj
		nodeObj := new(coreV1.Node)
		assert.Nil(t, c.k8sClient.ReadCR(testCtx, k8sNode.Name, "", nodeObj))
		_, ok := nodeObj.GetAnnotations()[nodeIDAnnotationKey]
		assert.False(t, ok)
	})
}

func Test_reconcileForCSIBMNode(t *testing.T) {
	t.Run("CSIBMNode is being deleted. Annotation was removed.", func(t *testing.T) {
		var (
			c       = setup(t)
			bmNode  = testCSIBMNode1.DeepCopy()
			k8sNode = testNode1.DeepCopy()
		)

		k8sNode.Annotations[nodeIDAnnotationKey] = "aaaa-bbbb-cccc-dddd"
		bmNode.DeletionTimestamp = &metaV1.Time{Time: time.Now()}

		createObjects(t, c.k8sClient, bmNode, k8sNode)

		res, err := c.reconcileForCSIBMNode(bmNode)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		nodeObj := new(coreV1.Node)
		assert.Nil(t, c.k8sClient.ReadCR(testCtx, k8sNode.Name, "", nodeObj))
		_, ok := nodeObj.GetAnnotations()[nodeIDAnnotationKey]
		assert.False(t, ok)
		enabled := c.isEnabledForNode(nodeObj.Name)
		assert.False(t, enabled)
	})

	t.Run("CSIBMNode addresses length is 0", func(t *testing.T) {
		var (
			c      = setup(t)
			bmNode = testCSIBMNode1.DeepCopy()
		)

		bmNode.Spec.Addresses = map[string]string{}
		createObjects(t, c.k8sClient, bmNode)

		res, err := c.reconcileForCSIBMNode(bmNode)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "addresses are missing")
		assert.Equal(t, ctrl.Result{Requeue: false}, res)
	})

	t.Run("Unable to read k8s node", func(t *testing.T) {
		var (
			c           = setup(t)
			k8sNodeName = "k8s-node"
			bmNode      = testCSIBMNode1.DeepCopy()
		)

		c.cache.put(k8sNodeName, bmNode.Name)

		res, err := c.reconcileForCSIBMNode(bmNode)
		assert.NotNil(t, err)
		assert.Equal(t, ctrl.Result{Requeue: true}, res)
	})

	t.Run("There is CSIBMNode that partially match k8s node", func(t *testing.T) {
		var (
			c       = setup(t)
			k8sNode = testNode1.DeepCopy()
			bmNode  = testCSIBMNode1.DeepCopy()
		)

		k8sNode.Status.Addresses = []coreV1.NodeAddress{k8sNode.Status.Addresses[0]}
		createObjects(t, c.k8sClient, k8sNode, bmNode)

		res, err := c.reconcileForCSIBMNode(bmNode)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		// read node obj
		nodeObj := new(coreV1.Node)
		assert.Nil(t, c.k8sClient.ReadCR(testCtx, k8sNode.Name, "", nodeObj))
		_, ok := nodeObj.GetAnnotations()[nodeIDAnnotationKey]
		assert.False(t, ok)
	})

	t.Run("More then one k8s node match CSIBMNode CR", func(t *testing.T) {
		var (
			c        = setup(t)
			k8sNode1 = testNode1.DeepCopy()
			k8sNode2 = testNode2.DeepCopy()
			bmNode   = testCSIBMNode1.DeepCopy()
		)

		k8sNode2.Status.Addresses = k8sNode1.Status.Addresses
		createObjects(t, c.k8sClient, k8sNode1, k8sNode2, bmNode)

		res, err := c.reconcileForCSIBMNode(bmNode)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		// read node obj
		nodeObj := new(coreV1.Node)
		assert.Nil(t, c.k8sClient.ReadCR(testCtx, k8sNode1.Name, "", nodeObj))
		_, ok := nodeObj.GetAnnotations()[nodeIDAnnotationKey]
		assert.False(t, ok)
		assert.Nil(t, c.k8sClient.ReadCR(testCtx, k8sNode2.Name, "", nodeObj))
		_, ok = nodeObj.GetAnnotations()[nodeIDAnnotationKey]
		assert.False(t, ok)
	})
}

func Test_checkAnnotationAndLabels(t *testing.T) {
	testCases := []struct {
		description                string
		currentAnnotationValue     string
		targetAnnotationValue      string
		currentOsNameLabelValue    string
		targetOsNameLabelValue     string
		currentOsVersionLabelValue string
		targetOsVersionLabelValue  string
	}{
		{
			description:                "Node has required annotation and labels",
			currentAnnotationValue:     "aaaa-bbbb",
			targetAnnotationValue:      "aaaa-bbbb",
			currentOsNameLabelValue:    osName,
			targetOsNameLabelValue:     osName,
			currentOsVersionLabelValue: osVersion,
			targetOsVersionLabelValue:  osVersion,
		},
		{
			description:                "Node has required annotation and labels with wrong values",
			currentAnnotationValue:     "aaaa-bbbb",
			targetAnnotationValue:      "ffff-dddd",
			currentOsNameLabelValue:    osName,
			targetOsNameLabelValue:     osName,
			currentOsVersionLabelValue: osVersion,
			targetOsVersionLabelValue:  "19.10",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			var (
				c    = setup(t)
				node = testNode1.DeepCopy()
			)

			// set annotation
			node.Annotations[nodeIDAnnotationKey] = testCase.currentAnnotationValue
			// set OS image and labels
			node.Status.NodeInfo.OSImage = testCase.targetOsNameLabelValue + " " + testCase.targetOsVersionLabelValue
			node.Labels[common.NodeOSNameLabelKey] = testCase.currentOsNameLabelValue
			node.Labels[common.NodeOSVersionLabelKey] = testCase.currentOsVersionLabelValue

			createObjects(t, c.k8sClient, node)
			res, err := c.updateNodeLabelsAndAnnotation(node, testCase.targetAnnotationValue)
			assert.Nil(t, err)
			assert.Equal(t, ctrl.Result{}, res)

			// read node obj
			nodeObj := new(coreV1.Node)
			assert.Nil(t, c.k8sClient.ReadCR(testCtx, node.Name, "", nodeObj))
			// check annotations
			val, ok := nodeObj.GetAnnotations()[nodeIDAnnotationKey]
			assert.True(t, ok)
			assert.Equal(t, testCase.targetAnnotationValue, val)
			// check os name label
			val, ok = nodeObj.GetLabels()[common.NodeOSNameLabelKey]
			assert.True(t, ok)
			assert.Equal(t, testCase.targetOsNameLabelValue, val)
			// check os version label
			val, ok = nodeObj.GetLabels()[common.NodeOSVersionLabelKey]
			assert.True(t, ok)
			assert.Equal(t, testCase.targetOsVersionLabelValue, val)
		})
	}
}

func Test_constructAddresses(t *testing.T) {
	t.Run("Empty addresses", func(t *testing.T) {
		var (
			c    = setup(t)
			node = testNode1.DeepCopy()
			res  map[string]string
		)

		node.Status.Addresses = nil
		res = c.constructAddresses(node)
		assert.NotNil(t, res)
		assert.Equal(t, 0, len(res))
	})

	t.Run("Converted successfully", func(t *testing.T) {
		var (
			c     = setup(t)
			node  = testNode1.DeepCopy()
			res   map[string]string
			key   = "Hostname"
			value = "10.10.10.10"
		)

		node.Status.Addresses = []coreV1.NodeAddress{
			{Type: coreV1.NodeAddressType(key), Address: value},
		}

		res = c.constructAddresses(node)
		assert.Equal(t, 1, len(res))
		curr, ok := res[key]
		assert.True(t, ok)
		assert.Equal(t, value, curr)
	})
}

func setup(t *testing.T) *Controller {
	k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)

	c, err := NewController("", k8sClient, testLogger)
	assert.Nil(t, err)
	return c
}

func createObjects(t *testing.T, c *k8s.KubeClient, objs ...runtime.Object) {
	for _, obj := range objs {
		assert.Nil(t, c.Create(testCtx, obj))
	}
}

func convertCSIBMNodeAddrsToK8sNodeAddrs(bmNodeAddrs map[string]string) []coreV1.NodeAddress {
	res := make([]coreV1.NodeAddress, len(bmNodeAddrs))
	i := 0
	for t, v := range bmNodeAddrs {
		res[i] = coreV1.NodeAddress{
			Type:    coreV1.NodeAddressType(t),
			Address: v,
		}
		i++
	}

	return res
}
