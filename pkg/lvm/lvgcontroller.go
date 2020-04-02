package lvm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

const lvgFinalizer = "dell.emc.csi/lvg-cleanup"

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
	if lvg.ObjectMeta.DeletionTimestamp.IsZero() {
		// append finalizer if LVG doesn't contain it
		if !base.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer) {
			lvg.ObjectMeta.Finalizers = append(lvg.ObjectMeta.Finalizers, lvgFinalizer)
			if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
				ll.Errorf("Unable to append finalizer %s to LVG, error: %v.", lvgFinalizer, err)
				return ctrl.Result{Requeue: true}, err
			}
		}
	} else {
		if base.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer) {
			// remove AC that point on that LVG
			c.removeChildAC(lvg.Name)
			// cleanup LVM artifacts
			if err := c.removeLVGArtifacts(lvg.Name); err != nil {
				ll.Errorf("Unable to cleanup LVM artifacts: %v", err)
				return ctrl.Result{}, err
			}

			lvg.ObjectMeta.Finalizers = base.RemoveString(lvg.ObjectMeta.Finalizers, lvgFinalizer)
			if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
				ll.Errorf("Unable to update LVG's finalizers: %v", err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if lvg.Spec.Status == apiV1.Creating {
		newStatus := apiV1.Created
		var err error
		var locations []string
		if locations, err = c.createSystemLVG(lvg); err != nil {
			ll.Errorf("Unable to create system LVG: %v", err)
			newStatus = apiV1.Failed
		}

		lvg.Spec.Status = newStatus
		lvg.Spec.Locations = locations
		if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
			ll.Errorf("Unable to update LVG status to %s, error: %v.", newStatus, err)
		}
	}

	return ctrl.Result{}, nil
}

func (c *LVGController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lvgcrd.LVG{}).
		Complete(c)
}

// createSystemLVG creates LVG in the system and put all drives from lvg.Spec.Location in that LVG
// if some drive doesn't read that drive will not pass in lvg.Location
// return list of drives in LVG that should be used as a locations for this LVG
func (c *LVGController) createSystemLVG(lvg *lvgcrd.LVG) (locations []string, err error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "createSystemLVG",
		"lvgName": lvg.Name,
	})
	ll.Info("Processing ...")

	var deviceFiles = make([]string, 0) // device files of each drive in LVG
	for _, driveUUID := range lvg.Spec.Locations {
		drive := &drivecrd.Drive{}
		if err := c.k8sClient.ReadCR(context.Background(), driveUUID, drive); err != nil {
			// that drive will not be in LVG location
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
		locations = append(locations, driveUUID)
		deviceFiles = append(deviceFiles, dev)
	}
	if len(deviceFiles) == 0 {
		return locations, errors.New("no one PVs were created")
	}
	// create vg
	if err = c.linuxUtils.VGCreate(lvg.Name, deviceFiles...); err != nil {
		ll.Errorf("Unable to create VG: %v", err)
		return locations, err
	}
	return locations, nil
}

// removeLVGArtifacts removes LVG and PVs that doesn't correspond to particular LVG
// when LVG is removed all PVs that were in that LVG becomes orphans
func (c *LVGController) removeLVGArtifacts(lvgName string) error {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "removeLVGArtifacts",
		"lvgName": lvgName,
	})
	ll.Info("Processing ...")

	if c.linuxUtils.IsVGContainsLVs(lvgName) {
		ll.Errorf("There are LVs in LVG. Unable to remove it.")
		return fmt.Errorf("there are LVs in LVG %s", lvgName)
	}

	var err error
	if err = c.linuxUtils.VGRemove(lvgName); err != nil {
		return fmt.Errorf("unable to remove LVG %s: %v", lvgName, err)
	}
	_ = c.linuxUtils.RemoveOrphanPVs() // ignore error since LVG was removed successfully
	return nil
}

// removeChildAC removes AC CR that points (AC.Spec.Location) on LVG CR with name lvgName
func (c *LVGController) removeChildAC(lvgName string) {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "removeChildAC",
		"lvgName": lvgName,
	})
	acList := &accrd.AvailableCapacityList{}
	if err := c.k8sClient.ReadList(context.Background(), acList); err != nil {
		ll.Errorf("Unable to list ACs: %v", err)
	}
	for _, ac := range acList.Items {
		if ac.Spec.Location == lvgName {
			acToRemove := ac
			if err := c.k8sClient.DeleteCR(context.Background(), &acToRemove); err != nil {
				ll.Errorf("Unable to remove AC %v, error: %v", ac, err)
			}
		}
	}
}
