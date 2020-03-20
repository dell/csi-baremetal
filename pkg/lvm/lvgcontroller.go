package lvm

import (
	"context"
	"errors"
	"time"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"

	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

type LVGController struct {
	k8sClient  *base.KubeClient
	node       string
	linuxUtils *base.LinuxUtils
	e          base.CmdExecutor

	log *logrus.Entry
}

func NewLVGController(k8sClient *base.KubeClient, nodeID string, log *logrus.Logger) *LVGController {
	e := &base.Executor{}
	e.SetLogger(log)
	return &LVGController{
		k8sClient:  k8sClient,
		node:       nodeID,
		log:        log.WithField("component", "LVGController"),
		e:          e,
		linuxUtils: base.NewLinuxUtils(e, log),
	}
}

func (c *LVGController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelFn()

	ll := c.log.WithFields(logrus.Fields{
		"method":  "Reconcile",
		"LVGName": req.Name,
	})

	lvg := &lvgcrd.LVG{}

	if err := c.k8sClient.ReadCR(ctx, req.Name, lvg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Here we need to check that this LVG CR corresponds to this node
	// because we deploy LVG CR Controller as DaemonSet
	if lvg.Spec.Node != c.node {
		ll.Info("Skip ...")
		return ctrl.Result{}, nil
	}

	ll.Infof("Reconciling LVG: %v", lvg)
	if lvg.Spec.Status == api.OperationalStatus_Creating {
		ll.Info("Creating LVG")
		var deviceFiles = make([]string, 0) // device files of each drive in LVG
		for _, driveUUID := range lvg.Spec.Locations {
			drive := &drivecrd.Drive{}
			if err := c.k8sClient.ReadCR(context.Background(), driveUUID, drive); err != nil {
				ll.Errorf("Unable to read drive %s, error: %v", driveUUID, err)
				continue
			}
			sn := drive.Spec.SerialNumber
			dev, err := c.linuxUtils.SearchDrivePathBySN(sn)
			if err != nil {
				ll.Error(err)
				continue
			}
			// create PV
			if err := c.linuxUtils.PVCreate(dev); err != nil {
				ll.Errorf("Unable to create PV for device %s: %v", dev, err)
				continue
			}
			ll.Infof("PV for device %s (drive serial %s) was created.", dev, sn)
			deviceFiles = append(deviceFiles, dev)
		}
		if len(deviceFiles) == 0 {
			err := errors.New("no one PVs were created")
			ll.Error(err)
			return ctrl.Result{}, err
		}
		// create vg
		if err := c.linuxUtils.VGCreate(req.Name, deviceFiles...); err != nil {
			ll.Errorf("Unable to create vg: %v", err)
			return ctrl.Result{}, err
		}
		ll.Info("LVG was created on the node, changing LVG CR status")
		lvg.Spec.Status = api.OperationalStatus_Created
		if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
			ll.Errorf("Unable to update LVG status: %v. But vg was created successfully.", err)
			return ctrl.Result{}, err
		}
	} else {
		ll.Infof("LVG was reconciled successfully (nothing to do)")
	}

	return ctrl.Result{}, nil
}

func (c *LVGController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lvgcrd.LVG{}).
		Complete(c)
}
