package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

const (
	// HelmInstallCSICmdTmpl is a template for helm command
	HelmInstallCSICmdTmpl = "helm install csi-baremetal /csi-baremetal-driver --set image.tag=%s --set drivemgr.type=%s"
	// KernelValue is csi driver kernel version value in charts
	KernelValue = " --set kernel.version=%s"
)

// Observer is an interface encapsulated method for notification
type Observer interface {
	Notify(string)
}

// CSIInstaller represents CSI installation process with helm
type CSIInstaller struct {
	version    string
	drivemgr   string
	updated    chan string
	kubeClient *k8s.KubeClient
	log        *logrus.Entry
	sync.Once
}

// NewCSIInstaller is a constructor for CSIInstaller
func NewCSIInstaller(version string, drivemgr string, kubeClient *k8s.KubeClient, log *logrus.Logger) *CSIInstaller {
	return &CSIInstaller{
		version:    version,
		drivemgr:   drivemgr,
		kubeClient: kubeClient,
		updated:    make(chan string),
		log:        log.WithField("component", "CSIInstaller"),
	}
}

// Notify send version to CSIInstaller channel
// Receive string
func (c *CSIInstaller) Notify(version string) {
	c.Do(func() {
		c.updated <- version
	})
}

// Install tries to install CSI in go routine and wait until it send value to error channel
// Return error
func (c *CSIInstaller) Install() error {
	var (
		ctxWithTimeout, cancel = context.WithTimeout(context.Background(), time.Minute*5)
		chanErr                = make(chan error)
	)
	defer cancel()
	defer close(chanErr)
	defer close(c.updated)

	go func() {
		chanErr <- c.install(ctxWithTimeout)
	}()

	return <-chanErr
}

// install waits until value occurs in updated channel or context id done
// Receive context
// Return error
func (c *CSIInstaller) install(ctx context.Context) error {
	ll := c.log.WithFields(logrus.Fields{
		"method": "install",
	})
	select {
	case version := <-c.updated:
		ll.Infof("Receive kernel version: %s", version)
		return c.installWithHelm(version)
	case <-ctx.Done():
		return fmt.Errorf("context is done: %v", ctx.Err())
	}
}

// installWithHelm tris to install helm with kernel version or without it if parameter kernelVersion is empty
// Receive string
// Return error
func (c *CSIInstaller) installWithHelm(kernelVersion string) error {
	cmd := fmt.Sprintf(HelmInstallCSICmdTmpl, c.version, c.drivemgr) + fmt.Sprintf(KernelValue, kernelVersion)
	executor := command.NewExecutor(c.log.Logger)
	if _, _, err := executor.RunCmd(cmd); err != nil {
		return err
	}
	return nil
}
