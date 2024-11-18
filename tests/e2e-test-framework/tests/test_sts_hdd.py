import pytest
from framework.sts import STS
from framework.utils import Utils


class TestStsHdd:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(
        cls,
        namespace,
        utils: Utils,
    ):
        cls.namespace = namespace
        cls.utils = utils
        cls.name = "test-sts-hdd"
        cls.timeout = 120
        cls.replicas = 1

        cls.sts = STS(cls.namespace, cls.name, cls.replicas)
        cls.sts.delete()
        cls.sts.create(storage_classes=["csi-baremetal-sc-hdd"])

        yield

        cls.utils.wait_for_pod_removing(cls.sts.delete())
        cls.utils.clear_csi_resources(namespace=cls.namespace)

    @pytest.mark.hal
    def test_6105_create_sts_with_hdd_volume(self):
        assert self.sts.verify(self.timeout) is True, f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
