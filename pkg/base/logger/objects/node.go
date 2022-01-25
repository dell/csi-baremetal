package objects

import (
	"fmt"

	"github.com/dell/csi-baremetal/api/v1/nodecrd"
)

type node struct{}

func (l *node) Log(object *nodecrd.Node) (str string) {
	return fmt.Sprintf("Labels: %+v, Annotations: %+v, Spec: %+v",
		object.ObjectMeta.Labels, object.ObjectMeta.Annotations, object.Spec)
}

func newNode() *node {
	return &node{}
}
