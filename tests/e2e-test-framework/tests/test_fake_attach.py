import logging
import pytest

import framework.const as const
from framework.sts import STS
from framework.utils import Utils
from framework.drive import DriveUtils
from framework.ssh import SSHCommandExecutor

class TestFakeAttach:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(cls, namespace, vm_user, vm_cred):
        cls.namespace = namespace
        cls.name = "test-sts-fake-attach-without-dr"
        cls.timeout = 120
        cls.replicas = 1

        cls.utils = Utils(vm_user, vm_cred, namespace)
        cls.drive_utils = DriveUtils(SSHCommandExecutor(ip_address="", username=vm_user, password=vm_cred))
        cls.sts = STS(cls.namespace, cls.name, cls.replicas)
        cls.sts.delete()
        cls.sts.create(storage_classes=[const.SSD_SC])

        yield

        cls.sts.delete()
        cls.utils.clear_cluster_resources(cls.namespace)

    @pytest.mark.hal
    def test_5808_fake_attach_without_dr(self):
        assert self.sts.verify(self.timeout) is True, f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
        pod = self.utils.list_pods(label="app=" + self.name, namespace=self.namespace)[0]
        pvc = self.utils.list_persistent_volume_claims(namespace=self.namespace, pod_name=pod.metadata.name)[0]

        self.utils.annotate_pvc(
            resource_name=pvc.metadata.name,
            annotation_key="pv.attach.kubernetes.io/ignore-if-inaccessible",
            annotation_value="yes",
            namespace=self.namespace,
        )

        volume = self.utils.list_volumes(name=pvc.spec.volume_name)[0]
        drive_cr = self.utils.get_drive_cr(volume_name=volume['metadata']['name'], namespace=self.namespace)
        drive_path = drive_cr["spec"]["Path"]
        assert drive_path, "Drive path not found"
        logging.info(f"drive_path: {drive_path}")

        host_num =self.drive_utils.get_host_num(drive_path)
        scsi_id = self.drive_utils.get_scsi_id(drive_path)
        assert scsi_id, "scsi_id not found"
        logging.info(f"scsi_id: {scsi_id}")

        self.drive_utils.remove(scsi_id)
        logging.info(f"drive {drive_path}, {scsi_id} removed")

        drive_name = drive_cr["metadata"]["name"]
        self.utils.wait_drive(name=drive_name, expected_status=const.STATUS_OFFLINE)
        logging.info(f"drive {drive_name} went {const.STATUS_OFFLINE}")

        pod = self.utils.recreate_pod(pod)
        volume_name = volume['metadata']['name']
        assert self.event_in(
            plural="volumes", 
            resource_name=volume_name,
            reason=const.FAKE_ATTACH_INVOLVED
        ), f"event {const.FAKE_ATTACH_INVOLVED} not found"

        self.drive_utils.restore(host_num=host_num)
        logging.info(f"waiting for a drive {drive_name} to be {const.STATUS_ONLINE}")
        self.utils.wait_drive(name=drive_name, expected_status=const.STATUS_ONLINE)

        self.utils.recreate_pod(pod)
        assert self.event_in(
            plural="volumes", 
            resource_name=volume_name, 
            reason=const.FAKE_ATTACH_CLEARED
        ), f"event {const.FAKE_ATTACH_CLEARED} not found"

    def event_in(self, plural: str, resource_name: str, reason: str) -> bool:
        events = self.utils.get_events_by_reason(plural=plural, resource_name=resource_name, reason=reason, namespace=self.namespace)
        for event in events:
            if event.reason == reason:
                logging.info(f"event {reason} found")
                return True
        logging.warning(f"event {reason} not found")
        return False