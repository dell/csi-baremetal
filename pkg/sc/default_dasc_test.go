package sc

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

type testExecutorSuccess struct{}

func (t testExecutorSuccess) RunCmd(cmd interface{}) (string, string, error) {
	return "stdout", "", nil
}

var defaultDaSCSuccess = &DefaultDASC{
	executor: testExecutorSuccess{},
}

type testExecutorFail struct{}

func (t testExecutorFail) RunCmd(cmd interface{}) (string, string, error) {
	return "error happened", "error", errors.New("error")
}

var defaultDaSCFail = &DefaultDASC{
	executor: testExecutorFail{},
}

func TestCreateFileSystem(t *testing.T) {
	ok, err := defaultDaSCSuccess.CreateFileSystem(XFS, "/dev/sda")
	assert.Equal(t, ok, true)
	assert.Equal(t, err, nil)

	// unknown file system
	ok, err = defaultDaSCSuccess.CreateFileSystem("qwe", "/dev/sda")
	assert.Equal(t, ok, false)
	assert.Equal(t, err, errors.New("unknown file system"))
}

func TestCreateFileSystemFail(t *testing.T) {
	ok, err := defaultDaSCFail.CreateFileSystem(XFS, "/dev/sda")
	assert.Equal(t, ok, false)
	assert.Equal(t, err, err)
}

func TestDeleteFileSystem(t *testing.T) {
	ok, err := defaultDaSCSuccess.DeleteFileSystem("/dev/sda")
	assert.Equal(t, ok, true)
	assert.Equal(t, err, nil)
}

func TestDeleteFileSystemFail(t *testing.T) {
	ok, err := defaultDaSCFail.DeleteFileSystem("/dev/sda")
	assert.Equal(t, ok, false)
	assert.Equal(t, err, err)
}

func TestCreateTargetPath(t *testing.T) {
	ok, err := defaultDaSCSuccess.CreateTargetPath("/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount")
	assert.Equal(t, ok, true)
	assert.Equal(t, err, nil)
}

func TestCreateTargetPathFail(t *testing.T) {
	ok, err := defaultDaSCFail.CreateTargetPath("/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount")
	assert.Equal(t, ok, false)
	assert.Equal(t, err, err)
}

func TestDeleteTargetPath(t *testing.T) {
	ok, err := defaultDaSCSuccess.DeleteTargetPath("/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount")
	assert.Equal(t, ok, true)
	assert.Equal(t, err, nil)
}

func TestDeleteTargetPathFail(t *testing.T) {
	ok, err := defaultDaSCFail.DeleteTargetPath("/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount")
	assert.Equal(t, ok, false)
	assert.Equal(t, err, err)
}

func TestIsMounted(t *testing.T) {
	ok, err := defaultDaSCSuccess.IsMounted("/dev/sda", "/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount")
	assert.Equal(t, ok, false)
	assert.Equal(t, err, nil)
}

func TestIsMountedFail(t *testing.T) {
	ok, err := defaultDaSCFail.IsMounted("/dev/sda", "/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount")
	assert.Equal(t, ok, false)
	assert.Equal(t, err, err)
}

func TestMount(t *testing.T) {
	ok, err := defaultDaSCSuccess.Mount("/dev/sda", "/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount")
	assert.Equal(t, ok, true)
	assert.Equal(t, err, nil)
}

func TestMountFail(t *testing.T) {
	ok, err := defaultDaSCFail.Mount("/dev/sda", "/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount")
	assert.Equal(t, ok, false)
	assert.Equal(t, err, err)
}
