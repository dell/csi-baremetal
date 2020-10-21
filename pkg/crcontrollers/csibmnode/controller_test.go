package csibmnode

import (
	"context"
	"testing"

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

	testNode1 = coreV1.Node{ObjectMeta: metaV1.ObjectMeta{Name: "node-1", Namespace: testNS, Annotations: map[string]string{}}}
	testNode2 = coreV1.Node{ObjectMeta: metaV1.ObjectMeta{Name: "node-2", Namespace: testNS, Annotations: map[string]string{}}}
)

func TestNewCSIBMController(t *testing.T) {
	k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)

	c, err := NewController(k8sClient, testLogger)
	assert.Nil(t, err)
	assert.NotNil(t, c)
	assert.NotNil(t, c.cache)
	assert.NotNil(t, c.cache.bmToK8sNode)
	assert.NotNil(t, c.cache.k8sToBMNode)
}

func TestReconcile(t *testing.T) {
	t.Run("Reconcile for k8s node. Success", func(t *testing.T) {
		var (
			c      = setup(t)
			node   = testNode1
			bmNode = testCSIBMNode1
		)

		node.Annotations[NodeIDAnnotationKey] = bmNode.Spec.UUID
		createObjects(t, c.k8sClient, &bmNode, &node)

		res, err := c.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Name: node.Name, Namespace: testNS}})
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)
	})

	t.Run("Reconcile for CSIBMNode. Success", func(t *testing.T) {
		var (
			c      = setup(t)
			node   = testNode1 // annotation should be set for that object
			bmNode = testCSIBMNode1
		)

		node.Status.Addresses = convertCSIBMNodeAddrsToK8sNodeAddrs(bmNode.Spec.Addresses)
		createObjects(t, c.k8sClient, &bmNode, &node)

		res, err := c.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Name: bmNode.Name, Namespace: testNS}})
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		// read node obj
		nodeObj := new(coreV1.Node)
		assert.Nil(t, c.k8sClient.Get(testCtx, k8sCl.ObjectKey{Name: node.Name}, nodeObj))
		val, ok := nodeObj.GetAnnotations()[NodeIDAnnotationKey]
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

}

func Test_reconcileForCSIBMNode(t *testing.T) {

}

func Test_checkAnnotation(t *testing.T) {
	t.Run("Node has needed annotation with goal value", func(t *testing.T) {

	})

	t.Run("Node has needed annotation with another value", func(t *testing.T) {

	})

	t.Run("Node doesnt have needed annotation", func(t *testing.T) {

	})
}

func Test_constructAddresses(t *testing.T) {
	t.Run("Empty addresses", func(t *testing.T) {
		var (
			c    = setup(t)
			node = testNode1
			res  map[string]string
		)

		res = c.constructAddresses(&node)
		assert.NotNil(t, res)
		assert.Equal(t, 0, len(res))
	})

	t.Run("Converted successfully", func(t *testing.T) {
		var (
			c     = setup(t)
			node  = testNode1
			res   map[string]string
			key   = "Hostname"
			value = "10.10.10.10"
		)

		node.Status.Addresses = []coreV1.NodeAddress{
			{Type: coreV1.NodeAddressType(key), Address: value},
		}

		res = c.constructAddresses(&node)
		assert.Equal(t, 1, len(res))
		curr, ok := res[key]
		assert.True(t, ok)
		assert.Equal(t, value, curr)
	})
}

func setup(t *testing.T) *Controller {
	k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)

	c, err := NewController(k8sClient, testLogger)
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
