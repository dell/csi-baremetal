package common

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	v1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/mocks"
)

func TestCSIInstaller_install_success(t *testing.T) {
	var (
		drivemgr = "basemgr"
		version  = "green"
		testNS   = "default"
		logger   = logrus.New()
		wg       sync.WaitGroup
	)
	kubeClient, err := k8s.GetFakeKubeClient(testNS, logger)
	assert.Nil(t, err)
	mockExec := &mocks.GoMockExecutor{}
	cmd := fmt.Sprintf(HelmInstallCSICmdTmpl, version, drivemgr)
	mockExec.On("RunCmd", cmd).Return("", "", nil)
	installer := NewCSIInstaller(version, drivemgr, kubeClient, mockExec, logger)
	wg.Add(1)
	go func() {
		err = installer.install(context.Background())
		wg.Done()
	}()
	installer.Notify("4.15")
	wg.Wait()
	assert.Nil(t, err)
}

func TestCSIInstaller_install_failed(t *testing.T) {
	var (
		drivemgr = "basemgr"
		version  = "green"
		testNS   = "default"
		logger   = logrus.New()
		wg       sync.WaitGroup
	)
	kubeClient, err := k8s.GetFakeKubeClient(testNS, logger)
	assert.Nil(t, err)
	mockExec := &mocks.GoMockExecutor{}
	cmd := fmt.Sprintf(HelmInstallCSICmdTmpl, version, drivemgr)
	mockExec.On("RunCmd", cmd).Return("", "", nil)
	installer := NewCSIInstaller(version, drivemgr, kubeClient, mockExec, logger)
	wg.Add(1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err = installer.install(ctx)
		wg.Done()
	}()
	wg.Wait()
	assert.NotNil(t, err)
}

func TestCSIInstaller_installWithHelm_success(t *testing.T) {
	var (
		drivemgr      = "basemgr"
		version       = "green"
		testNS        = "default"
		logger        = logrus.New()
		kernelVersion = "4.15"
	)
	kubeClient, err := k8s.GetFakeKubeClient(testNS, logger)
	assert.Nil(t, err)
	mockExec := &mocks.GoMockExecutor{}
	cmd := fmt.Sprintf(HelmInstallCSICmdTmpl, version, drivemgr)
	mockExec.On("RunCmd", cmd).Return("", "", nil).Times(1)
	installer := NewCSIInstaller(version, drivemgr, kubeClient, mockExec, logger)
	err = installer.installWithHelm(kernelVersion)
	assert.Nil(t, err)
}

func TestCSIInstaller_installWithHelm_success_kernel_image(t *testing.T) {
	var (
		drivemgr      = "basemgr"
		version       = "green"
		testNS        = "default"
		logger        = logrus.New()
		kernelVersion = "5.4"
	)
	kubeClient, err := k8s.GetFakeKubeClient(testNS, logger)
	assert.Nil(t, err)
	mockExec := &mocks.GoMockExecutor{}
	cmd := fmt.Sprintf(HelmInstallCSICmdTmpl, version, drivemgr) + fmt.Sprintf(KernelValue, v1.DockerImageKernelVersion)
	mockExec.On("RunCmd", cmd).Return("", "", nil).Times(1)
	installer := NewCSIInstaller(version, drivemgr, kubeClient, mockExec, logger)
	err = installer.installWithHelm(kernelVersion)
	assert.Nil(t, err)
}

func TestCSIInstaller_installWithHelm_failed(t *testing.T) {
	var (
		drivemgr      = "basemgr"
		version       = "green"
		testNS        = "default"
		logger        = logrus.New()
		kernelVersion = "4.15"
	)
	kubeClient, err := k8s.GetFakeKubeClient(testNS, logger)
	assert.Nil(t, err)
	mockExec := &mocks.GoMockExecutor{}
	cmd := fmt.Sprintf(HelmInstallCSICmdTmpl, version, drivemgr)
	mockExec.On("RunCmd", cmd).Return("", "", fmt.Errorf("error")).Times(1)
	installer := NewCSIInstaller(version, drivemgr, kubeClient, mockExec, logger)
	err = installer.installWithHelm(kernelVersion)
	assert.NotNil(t, err)
}

func TestNewCSIInstaller_convertKernelVersion(t *testing.T) {
	var (
		test1    = "4"
		test2    = "4.14"
		test3    = "5.4.0"
		drivemgr = "basemgr"
		version  = "green"
		logger   = logrus.New()
	)

	kubeClient, err := k8s.GetFakeKubeClient(testNS, logger)
	assert.Nil(t, err)
	installer := NewCSIInstaller(version, drivemgr, kubeClient, &mocks.GoMockExecutor{}, logger)
	_, err = installer.convertKernelVersion(test1)
	assert.NotNil(t, err)

	_, err = installer.convertKernelVersion(test1)
	assert.NotNil(t, err)

	kernelVersion, err := installer.convertKernelVersion(test2)
	assert.Nil(t, err)
	assert.Equal(t, 4.14, kernelVersion)

	kernelVersion, err = installer.convertKernelVersion(test3)
	assert.Nil(t, err)
	assert.Equal(t, 5.4, kernelVersion)
}
