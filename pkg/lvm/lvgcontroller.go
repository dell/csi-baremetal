package lvm

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

type LVGController struct {
	k8sClient *base.KubeClient
	node      string

	log *logrus.Entry
}

func NewLVGController(k8sClient *base.KubeClient, node string, log *logrus.Logger) *LVGController {
	return &LVGController{
		k8sClient: k8sClient,
		node:      node,
		log:       log.WithField("component", "LVGController"),
	}
}

func (l *LVGController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelFn()

	ll := l.log.WithFields(logrus.Fields{
		"method":  "Reconcile",
		"LVGName": req.Name,
	})

	lvg := &lvgcrd.LVG{}

	err := l.k8sClient.ReadCR(ctx, req.Name, lvg)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Here we need to check that this LVG CR corresponds to this node
	// because we deploy LVG CR Controller as DaemonSet
	if lvg.Spec.Node != l.node {
		ll.Info("Skip ...")
		return ctrl.Result{}, nil
	}

	ll.Infof("Reconciling LVG: %v", lvg)
	return ctrl.Result{}, nil
}

func (l *LVGController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lvgcrd.LVG{}).
		Complete(l)
}
