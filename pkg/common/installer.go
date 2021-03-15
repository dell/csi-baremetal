package common

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	v1 "github.com/dell/csi-baremetal/api/v1"
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
	e          command.CmdExecutor
	sync.Once
}

// NewCSIInstaller is a constructor for CSIInstaller
func NewCSIInstaller(version string, drivemgr string, kubeClient *k8s.KubeClient, e command.CmdExecutor, log *logrus.Logger) *CSIInstaller {
	return &CSIInstaller{
		version:    version,
		drivemgr:   drivemgr,
		kubeClient: kubeClient,
		updated:    make(chan string),
		e:          e,
		log:        log.WithField("component", "CSIInstaller"),
	}
}

// Notify send version to CSIInstaller channel
// Receive string
func (c *CSIInstaller) Notify(version string) {
	c.log.WithField("method", "Notify").Info("In notify methods")
	c.Do(func() {
		c.log.WithField("method", "Notify").Info("update channel")
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

// installWithHelm tries to install helm with kernel version
// Receive string
// Return error
func (c *CSIInstaller) installWithHelm(kernelVersion string) error {
	version, err := c.convertKernelVersion(kernelVersion)
	if err != nil {
		return fmt.Errorf("kernel version has a wrong format: %s", kernelVersion)
	}
	cmd := fmt.Sprintf(HelmInstallCSICmdTmpl, c.version, c.drivemgr)
	imageVersion, _ := strconv.ParseFloat(v1.DockerImageKernelVersion, 64)
	if version >= imageVersion {
		cmd = fmt.Sprintf(HelmInstallCSICmdTmpl, c.version, c.drivemgr) + fmt.Sprintf(KernelValue, v1.DockerImageKernelVersion)
	}
	if _, _, err := c.e.RunCmd(cmd); err != nil {
		return err
	}
	return nil
}

// convertKernelVersion converts kernelVersion of format x.y.z to x.y to compare with 5.4 version
func (c *CSIInstaller) convertKernelVersion(kernelVersion string) (float64, error) {
	versionSplit := strings.Split(kernelVersion, ".")
	if len(versionSplit) < 2 {
		return 0, fmt.Errorf("kernel version has a wrong format: %s", kernelVersion)
	}
	var newVersion []string
	newVersion = append(newVersion, versionSplit[0], versionSplit[1])
	version := strings.Join(newVersion, ".")
	return strconv.ParseFloat(version, 64)
}
