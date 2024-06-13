import pytest
from framework.sts import STS


class TestStsLoopback:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(cls, namespace):
        cls.namespace = namespace
        cls.name = "test-sts-loopback"
        cls.timeout = 120
        cls.replicas = 1

        cls.sts = STS(cls.namespace, cls.name, cls.replicas)
        cls.sts.delete()
        cls.sts.create(storage_class="csi-baremetal-sc-hdd")

        yield

        cls.sts.delete()

    @pytest.mark.loopback
    def test_1000_create_sts_with_loopback_volume(self):
        assert self.sts.verify(self.timeout) is True, f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
