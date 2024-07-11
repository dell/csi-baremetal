import pytest
from framework.sts import STS
from framework.ssh import SSHCommandExecutor
from framework.drive import DriveUtils


class TestStsHdd:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(cls, namespace, vm_user, vm_cred):
        cls.namespace = namespace
        cls.name = "test-sts-hdd"
        cls.timeout = 120
        cls.replicas = 1
        cls.drive_utils = DriveUtils(SSHCommandExecutor(ip_address="", username=vm_user, password=vm_cred))

        # cls.sts = STS(cls.namespace, cls.name, cls.replicas)
        # cls.sts.delete()
        # cls.sts.create(storage_classes=["csi-baremetal-sc-hdd"])

        # yield

        # cls.sts.delete()

    @pytest.mark.hal
    def test_6105_create_sts_with_hdd_volume(self):
        self.drive_utils.wipe_drives() 
        # assert self.sts.verify(self.timeout) is True, f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
