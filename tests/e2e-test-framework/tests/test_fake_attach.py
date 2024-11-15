import logging
from typing import Dict
import pytest

import framework.const as const

from framework.sts import STS
from framework.utils import Utils
from framework.drive import DriveUtils


class TestFakeAttach:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(
        cls,
        namespace: str,
        drive_utils_executors: Dict[str, DriveUtils],
        utils: Utils,
    ):
        cls.namespace = namespace
        cls.name = "test-sts-fake-attach-without-dr"
        cls.timeout = 120
        cls.replicas = 1

        cls.utils = utils

        cls.drive_utils = drive_utils_executors
        cls.sts = STS(cls.namespace, cls.name, cls.replicas)
        cls.sts.delete()
        cls.sts.create(storage_classes=[const.SSD_SC])

        yield

        cls.utils.wait_for_pod_removing(cls.sts.delete())
        cls.utils.clear_csi_resources(namespace=cls.namespace)

    @pytest.mark.hal
    def test_5808_fake_attach_without_dr(self):
        assert (
            self.sts.verify(self.timeout) is True
        ), f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
        pod = self.utils.list_pods(
            label="app=" + self.name, namespace=self.namespace
        )[0]
        node_ip = self.utils.get_pod_node_ip(
            pod_name=pod.metadata.name, namespace=self.namespace
        )
        pvc = self.utils.list_persistent_volume_claims(
            namespace=self.namespace, pod_name=pod.metadata.name
        )[0]

        self.utils.annotate_pvc(
            resource_name=pvc.metadata.name,
            annotation_key=const.FAKE_ATTACH_PVC_ANNOTATION_KEY,
            annotation_value=const.FAKE_ATTACH_PVC_ANNOTATION_VALUE,
            namespace=self.namespace,
        )

        volume = self.utils.list_volumes(name=pvc.spec.volume_name)[0]
        drive_cr = self.utils.get_drive_cr(
            volume_name=volume["metadata"]["name"], namespace=self.namespace
        )
        drive_path = drive_cr["spec"]["Path"]
        assert drive_path, "Drive path not found"
        logging.info(f"drive_path: {drive_path}")

        host_num = self.drive_utils[node_ip].get_host_num(drive_path)
        scsi_id = self.drive_utils[node_ip].get_scsi_id(drive_path)
        assert scsi_id, "scsi_id not found"
        logging.info(f"scsi_id: {scsi_id}")

        self.drive_utils[node_ip].remove(scsi_id)
        logging.info(f"drive {drive_path}, {scsi_id} removed")

        drive_name = drive_cr["metadata"]["name"]
        self.utils.wait_drive(
            name=drive_name, expected_status=const.STATUS_OFFLINE
        )
        logging.info(f"drive {drive_name} went {const.STATUS_OFFLINE}")

        pod = self.utils.recreate_pod(name=pod.metadata.name, namespace=self.namespace)
        volume_name = volume["metadata"]["name"]
        assert self.utils.wait_event_in(
            resource_name=volume_name, reason=const.FAKE_ATTACH_INVOLVED
        ), f"event {const.FAKE_ATTACH_INVOLVED} not found"

        self.drive_utils[node_ip].restore(host_num=host_num)
        logging.info(
            f"waiting for a drive {drive_name} to be {const.STATUS_ONLINE}"
        )
        self.utils.wait_drive(
            name=drive_name, expected_status=const.STATUS_ONLINE
        )

        self.utils.recreate_pod(name=pod.metadata.name, namespace=self.namespace)
        assert self.utils.wait_event_in(
            resource_name=volume_name,
            reason=const.FAKE_ATTACH_CLEARED,
        ), f"event {const.FAKE_ATTACH_CLEARED} not found"
