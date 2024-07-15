import logging
import time
import pytest
from typing import Dict

import framework.const as const

from framework.sts import STS
from framework.utils import Utils
from framework.drive import DriveUtils


class TestFakeAttachMultipleVolumesPerPod:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(
        cls,
        namespace: str,
        drive_utils_executors: Dict[str, DriveUtils],
        utils: Utils,
    ):
        cls.namespace = namespace
        cls.name = "test-sts-fake-attach-dr"
        cls.timeout = 120
        cls.replicas = 1

        cls.utils = utils

        cls.drive_utils = drive_utils_executors
        cls.sts = STS(cls.namespace, cls.name, cls.replicas)
        cls.sts.delete()
        cls.sts.create(storage_classes=[const.SSD_SC, const.HDD_SC])

        yield

        cls.sts.delete()

    @pytest.mark.hal
    def test_6281_multiple_volumes_per_pod_fake_attach(self):
        assert (
            self.sts.verify(self.timeout) is True
        ), f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
        pod = self.utils.list_pods(
            label="app=" + self.name, namespace=self.namespace
        )[0]
        node_ip = self.utils.get_pod_node_ip(
            pod_name=pod.metadata.name, namespace=self.namespace
        )
        pvcs = self.utils.list_persistent_volume_claims(
            namespace=self.namespace, pod_name=pod.metadata.name
        )
        pvc = [
            pvc for pvc in pvcs if pvc.spec.storage_class_name == const.HDD_SC
        ][0]
        volume = self.utils.list_volumes(
            name=pvc.spec.volume_name, storage_class=const.HDD_SC
        )[0]
        volume_name = volume["metadata"]["name"]

        drive_cr = self.utils.get_drive_cr(
            volume_name=volume["metadata"]["name"], namespace=self.namespace
        )
        drive_name = drive_cr["metadata"]["name"]
        drive_path = drive_cr["spec"]["Path"]

        self.utils.annotate_custom_resource(
            resource_name=drive_name,
            resource_type=const.DRIVES_PLURAL,
            annotation_key=const.DRIVE_HEALTH_ANNOTATION,
            annotation_value=const.HEALTH_BAD,
        )

        assert self.utils.wait_drive(
            name=drive_name,
            expected_health=const.HEALTH_BAD,
            expected_status=const.STATUS_ONLINE,
            expected_usage=const.USAGE_RELEASING,
        ), f"Drive: {drive_name} failed to reach expected health: {const.HEALTH_BAD}"

        assert self.utils.event_in(
            resource_name=drive_name,
            reason=const.DRIVE_HEALTH_FAILURE_EVENT,
        )

        self.utils.annotate_custom_resource(
            resource_name=volume_name,
            resource_type=const.VOLUMES_PLURAL,
            annotation_key=const.VOLUME_RELEASE_ANNOTATION,
            annotation_value=const.VOLUME_RELEASE_DONE_VALUE,
            namespace=self.namespace,
        )

        assert self.utils.wait_volume(
            name=volume_name,
            expected_usage=const.USAGE_RELEASED,
        ), f"Volume: {volume_name} failed to reach expected usage: {const.USAGE_RELEASED}"

        self.utils.annotate_pvc(
            resource_name=pvc.metadata.name,
            annotation_key=const.FAKE_ATTACH_PVC_ANNOTATION_KEY,
            annotation_value=const.FAKE_ATTACH_PVC_ANNOTATION_VALUE,
            namespace=self.namespace,
        )
        logging.info(
            f"PVC {pvc.metadata.name} annotated with {const.FAKE_ATTACH_PVC_ANNOTATION_KEY} = {const.FAKE_ATTACH_PVC_ANNOTATION_VALUE}"
        )
        time.sleep(5)

        pod = self.utils.recreate_pod(
            name=pod.metadata.name, namespace=self.namespace
        )

        assert self.utils.event_in(
            resource_name=drive_name,
            reason=const.DRIVE_READY_FOR_PHYSICAL_REMOVAL_EVENT,
        )

        assert self.utils.wait_drive(
            name=drive_name,
            expected_status=const.STATUS_ONLINE,
            expected_usage=const.USAGE_REMOVED,
        )

        scsi_id = self.drive_utils[node_ip].get_scsi_id(drive_path)
        assert scsi_id, "scsi_id not found"
        logging.info(f"scsi_id: {scsi_id}")

        self.drive_utils[node_ip].remove(scsi_id)
        logging.info(f"drive {drive_path}, {scsi_id} removed")

        assert self.utils.wait_for_event_with_reason(
            reason=const.DRIVE_SUCCESSFULLY_REMOVED_EVENT
        )

        self.utils.clear_pvc_and_pod(
            pod_name=pod.metadata.name,
            pvc_name=pvc.metadata.name,
            volume_name=pvc.spec.volume_name,
            namespace=self.namespace,
        )
        assert self.utils.is_pod_running(
            pod_name=pod.metadata.name, timeout=self.timeout
        ), f"Pod: {pod.metadata.name} failed to reach running state"
