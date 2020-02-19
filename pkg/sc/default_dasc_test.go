package sc

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var (
	targetPathTest    = "/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount"
	eTest             = &mocks.GoMockExecutor{}
	d                 = &DefaultDASC{executor: eTest, log: logrus.NewEntry(logrus.New())}
	deviceTest        = "/dev/children1"
	mountpointCmdTest = fmt.Sprintf(MountpointCmdTmpl, deviceTest)
	mkfsCmdTest       = fmt.Sprintf(MkFSCmdTmpl, deviceTest)
	mkdirCmdTest      = fmt.Sprintf(MKdirCmdTmpl, targetPathTest)
	mountCmdTest      = fmt.Sprintf(MountCmdTmpl, deviceTest, targetPathTest)
	rmCmdTest         = fmt.Sprintf(RMCmdTmpl, targetPathTest)
	wipefsCmdTest     = fmt.Sprintf(WipeFSCmdTmpl, deviceTest)
	errTest           = errors.New("error")
)

var defaultDaSCSuccess = &DefaultDASC{
	executor: mocks.EmptyExecutorSuccess{},
	log:      logrus.NewEntry(logrus.New()),
}

var defaultDaSCFail = &DefaultDASC{
	executor: mocks.EmptyExecutorFail{},
	log:      logrus.NewEntry(logrus.New()),
}

func TestCreateFileSystem(t *testing.T) {
	err := defaultDaSCSuccess.CreateFileSystem(XFS, "/dev/sda")
	assert.Nil(t, err)
}

func TestCreateFileSystemFail(t *testing.T) {
	// unknown file system
	err := defaultDaSCSuccess.CreateFileSystem("qwe", "/dev/sda")
	assert.NotNil(t, err)

	err = defaultDaSCFail.CreateFileSystem(XFS, "/dev/sda")
	assert.NotNil(t, err)
}

func TestDeleteFileSystem(t *testing.T) {
	err := defaultDaSCSuccess.DeleteFileSystem("/dev/sda")
	assert.Nil(t, err)
}

func TestDeleteFileSystemFail(t *testing.T) {
	err := defaultDaSCFail.DeleteFileSystem("/dev/sda")
	assert.NotNil(t, err)
}

func TestCreateTargetPath(t *testing.T) {
	err := defaultDaSCSuccess.CreateTargetPath(targetPathTest)
	assert.Nil(t, err)
}

func TestCreateTargetPathFail(t *testing.T) {
	err := defaultDaSCFail.CreateTargetPath(targetPathTest)
	assert.NotNil(t, err)
}

func TestDeleteTargetPath(t *testing.T) {
	err := defaultDaSCSuccess.DeleteTargetPath(targetPathTest)
	assert.Nil(t, err)
}

func TestDeleteTargetPathFail(t *testing.T) {
	err := defaultDaSCFail.DeleteTargetPath(targetPathTest)
	assert.NotNil(t, err)
}

func TestIsMounted(t *testing.T) {
	ok, err := defaultDaSCSuccess.IsMounted("/dev/sdb1")
	assert.False(t, ok)
	assert.Nil(t, err)
}

func TestIsMountedFail(t *testing.T) {
	ok, err := defaultDaSCFail.IsMounted("/dev/sda1")
	assert.False(t, ok)
	assert.Equal(t, err, err)
}

func TestMount(t *testing.T) {
	err := defaultDaSCSuccess.Mount("/dev/sda", targetPathTest)
	assert.Nil(t, err)
}

func TestMountFail(t *testing.T) {
	err := defaultDaSCFail.Mount("/dev/sda", targetPathTest)
	assert.NotNil(t, err)
}

func TestUnmountSuccess(t *testing.T) {
	err := defaultDaSCSuccess.Unmount("/mnt/sda")
	assert.Nil(t, err)
}

func TestUnmountFail(t *testing.T) {
	err := defaultDaSCFail.Unmount("/mnt/sda")
	assert.NotNil(t, err)
}

func TestDefaultDASC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DefaultDASC testing suite")
}

var _ = Describe("Successful scenarios ", func() {
	Context("CreateVolume() success", func() {
		It("Should create volume", func() {
			eTest.On(mocks.RunCmd, mountpointCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mkfsCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mkdirCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mountCmdTest).Return("", "", nil).Times(1)

			rollBacked, err := d.PrepareVolume(deviceTest, targetPathTest)
			Expect(rollBacked).To(BeFalse())
			Expect(err).To(BeNil())
		})

		Context("Should rollback", func() {
			It("On mount stage", func() {
				newdeviceTest := "proc"

				rollBacked, err := d.PrepareVolume(newdeviceTest, targetPathTest)
				Expect(rollBacked).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("On creating target path stage", func() {
				eTest.On(mocks.RunCmd, mountpointCmdTest).Return("", "", nil).Times(1)
				eTest.On(mocks.RunCmd, mkfsCmdTest).Return("", "", nil).Times(1)

				eTest.On(mocks.RunCmd, mkdirCmdTest).Return("", "", errTest).Times(1)
				eTest.On(mocks.RunCmd, wipefsCmdTest).Return("", "", nil).Times(1)

				rollBacked, err := d.PrepareVolume(deviceTest, targetPathTest)
				Expect(rollBacked).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("On mount stage", func() {
				eTest.On(mocks.RunCmd, mountpointCmdTest).Return("", "", nil).Times(1)
				eTest.On(mocks.RunCmd, mkfsCmdTest).Return("", "", nil).Times(1)
				eTest.On(mocks.RunCmd, mkdirCmdTest).Return("", "", nil).Times(1)

				eTest.On(mocks.RunCmd, mountCmdTest).Return("", "", errTest).Times(1)
				eTest.On(mocks.RunCmd, rmCmdTest).Return("", "", nil).Times(1)
				eTest.On(mocks.RunCmd, wipefsCmdTest).Return("", "", nil).Times(1)

				rollBacked, err := d.PrepareVolume(deviceTest, targetPathTest)
				Expect(rollBacked).To(BeTrue())
				Expect(err).To(BeNil())
			})
		})
	})
})

var _ = Describe("Failure scenarios ", func() {
	Context("CreateVolume() failure", func() {
		It("Should fail with creating file system error", func() {
			eTest.On(mocks.RunCmd, mountpointCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mkfsCmdTest).Return("", "", errTest).Times(1)

			rollBacked, err := d.PrepareVolume(deviceTest, targetPathTest)
			Expect(rollBacked).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})

		It("Should fail with creating target path system error", func() {
			eTest.On(mocks.RunCmd, mountpointCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mkfsCmdTest).Return("", "", nil).Times(1)

			eTest.On(mocks.RunCmd, mkdirCmdTest).Return("", "", errTest).Times(1)
			eTest.On(mocks.RunCmd, wipefsCmdTest).Return("", "", errTest).Times(1)

			rollBacked, err := d.PrepareVolume(deviceTest, targetPathTest)
			Expect(rollBacked).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})

		It("Should fail with deleting target path system error", func() {
			eTest.On(mocks.RunCmd, mountpointCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mkfsCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mkdirCmdTest).Return("", "", nil).Times(1)

			eTest.On(mocks.RunCmd, mountCmdTest).Return("", "", errTest).Times(1)
			eTest.On(mocks.RunCmd, rmCmdTest).Return("", "", errTest).Times(1)

			rollBacked, err := d.PrepareVolume(deviceTest, targetPathTest)
			Expect(rollBacked).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})

		It("Should fail with deleting file system error", func() {
			eTest.On(mocks.RunCmd, mountpointCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mkfsCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, mkdirCmdTest).Return("", "", nil).Times(1)

			eTest.On(mocks.RunCmd, mountCmdTest).Return("", "", errTest).Times(1)
			eTest.On(mocks.RunCmd, rmCmdTest).Return("", "", nil).Times(1)
			eTest.On(mocks.RunCmd, wipefsCmdTest).Return("", "", errTest).Times(1)

			rollBacked, err := d.PrepareVolume(deviceTest, targetPathTest)
			Expect(rollBacked).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})
	})
})
