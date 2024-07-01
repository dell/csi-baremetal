import pytest
from framework.sts import STS, VOLUME_MOUNTS_SINGLE_HDD, VOLUME_CLAIM_TEMPLATES_SINGLE_HDD


class TestStsHdd:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(cls, namespace):
        cls.namespace = namespace
        cls.name = "test-sts-hdd"
        cls.timeout = 120
        cls.replicas = 1

        cls.sts = STS(cls.namespace, cls.name, cls.replicas)
        cls.sts.delete()
        cls.sts.create(volume_mounts=VOLUME_MOUNTS_SINGLE_HDD, volume_claim_templates=VOLUME_CLAIM_TEMPLATES_SINGLE_HDD)

        yield

        cls.sts.delete()

    @pytest.mark.hal
    def test_6105_create_sts_with_hdd_volume(self):
        assert self.sts.verify(self.timeout) is True, f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
