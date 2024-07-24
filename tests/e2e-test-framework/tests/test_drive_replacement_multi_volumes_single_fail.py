import logging
from typing import Dict
import pytest

import framework.const as const

from framework.sts import STS
from framework.utils import Utils
from framework.drive import DriveUtils


class TestAutoDriveReplacementWithMultipleVolumesPerPodSingleFailure:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(
        cls,
        namespace: str,
        drive_utils_executors: Dict[str, DriveUtils],
        utils: Utils,
    ):
        cls.namespace = namespace
        cls.name = "test-auto-dr-multiple-volumes-single-failure"
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
    def test_5955_auto_drive_replacement_with_multiple_volumes_per_pod_single_failure(
        self,
    ):
        # 1. get volume and volume groups for deployed pod
        assert (
            self.sts.verify(self.timeout) is True
        ), f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
        pod = self.utils.list_pods(name_prefix=self.name)[0]
        node_ip = self.utils.get_pod_node_ip(
            pod_name=pod.metadata.name, namespace=self.namespace
        )
        volumes = self.utils.list_volumes(pod_name=pod.metadata.name)
        # get all drives
        drives = []
        for volume in volumes:
            drive = self.utils.get_drive_cr(
                volume_name=volume["metadata"]["name"],
                namespace=volume["metadata"]["namespace"],
            )
            drives.append(drive)
        failed_drive = drives[0]
        healthy_drive = drives[1]
        failed_volume = volumes[0]
        # 2. simulate drive failure. Annotate drive used by pod with health=BAD
        failed_drive_name = failed_drive["metadata"]["name"]
        self.utils.annotate_custom_resource(
            resource_name=failed_drive_name,
            resource_type="drives",
            annotation_key=const.DRIVE_HEALTH_ANNOTATION,
            annotation_value=const.DRIVE_HEALTH_BAD_ANNOTATION,
        )
        logging.info(f"drive: {failed_drive_name} was annotated with health=BAD")
        # 3. wait until drive health is BAD, status=ONLINE, usage=RELEASING.
        logging.info(f"Waiting for drive: {failed_drive_name}")
        assert self.utils.wait_drive(
            name=failed_drive_name,
            expected_status=const.STATUS_ONLINE,
            expected_health=const.HEALTH_BAD,
            expected_usage=const.USAGE_RELEASING,
        ), f"Drive {failed_drive_name} failed to reach expected Status: {const.STATUS_ONLINE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_RELEASING}"
        logging.info(
            f"drive {failed_drive_name} went in Status: {const.STATUS_ONLINE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_RELEASING}"
        )
        # 4. wait until volume health is BAD, status=OPERATIVE, usage=RELEASING.
        failed_volume_name = failed_volume["metadata"]["name"]
        logging.info(f"Waiting for volume: {failed_volume_name}")
        assert self.utils.wait_volume(
            name=failed_volume_name,
            expected_health=const.HEALTH_BAD,
            expected_usage=const.USAGE_RELEASING,
            expected_operational_status=const.STATUS_OPERATIVE,
        ), f"Volume {failed_volume_name} failed to reach OperationalStatus: {const.STATUS_OPERATIVE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_RELEASING}"
        logging.info(
            f"volume {failed_volume_name} went in OperationalStatus: {const.STATUS_OPERATIVE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_RELEASING}"
        )
        # 5. check events and locate event related to DriveHealthFailure
        assert self.utils.event_in(
            resource_name=failed_drive_name,
            reason=const.DRIVE_HEALTH_FAILURE,
        ), f"event {const.DRIVE_HEALTH_FAILURE} for drive {failed_drive_name} not found"
        # 6. annotate volume with release=done
        self.utils.annotate_custom_resource(
            resource_name=failed_volume_name,
            resource_type="volumes",
            annotation_key=const.VOLUME_RELEASE_ANNOTATION,
            annotation_value=const.VOLUME_RELEASE_DONE_VALUE,
            namespace=failed_volume["metadata"]["namespace"],
        )
        logging.info(f"volume: {failed_volume_name} was annotated with release=done")
        # 7. check drive usages are RELEASED
        assert self.utils.wait_drive(
            name=failed_drive_name, expected_usage=const.USAGE_RELEASED
        ), f"Drive {failed_drive_name} failed to reach expected Usage: {const.USAGE_RELEASED}"
        logging.info(f"drive {failed_drive_name} went in Usage: {const.USAGE_RELEASED}")
        # 8. check volumes are RELEASED
        assert self.utils.wait_volume(
            name=failed_volume_name, expected_usage=const.USAGE_RELEASED
        ), f"Volume {failed_volume_name} failed to reach expected Usage {const.USAGE_RELEASED}"
        logging.info(
            f"volume {failed_volume_name} went in Usage: {const.USAGE_RELEASED}"
        )
        # 9. check event DriveReadyForRemoval is generated
        assert self.utils.event_in(
            resource_name=failed_drive_name,
            reason=const.DRIVE_READY_FOR_REMOVAL,
        ), f"event {const.DRIVE_READY_FOR_REMOVAL} for drive {failed_drive_name} not found"
        # 10. check events and locate event related to VolumeBadHealth
        assert self.utils.event_in(
            resource_name=failed_volume_name,
            reason=const.VOLUME_BAD_HEALTH,
        ), f"event {const.VOLUME_BAD_HEALTH} for volume {failed_volume_name} not found"
        # 11. delete pod and pvc
        self.utils.clear_pvc_and_pod(
            pod_name=pod.metadata.name, namespace=self.namespace
        )
        # 12. check Drive status to be REMOVING or REMOVED and LED state to be 1 (if drive supports LED ) or 2 (if drive does not support LED) Status to be ONLINE
        assert self.utils.wait_drive(
            name=failed_drive_name,
            expected_status=const.STATUS_ONLINE,
            expected_usage=const.USAGE_REMOVED,
            expected_health=const.HEALTH_BAD,
            expected_led_state=const.LED_STATE,
        ), f"Drive {failed_drive_name} failed to reach expected Status: {const.STATUS_ONLINE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_REMOVED}, LEDState: {drive["spec"]["LEDState"]}"
        logging.info(
            f"drive {failed_drive_name} went in Status: {const.STATUS_ONLINE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_REMOVED}, LEDState: {drive["spec"]["LEDState"]}"
        )
        # 13. check for events: DriveReadyForPhysicalRemoval
        assert self.utils.event_in(
            resource_name=failed_drive_name,
            reason=const.DRIVE_READY_FOR_PHYSICAL_REMOVAL,
        ), f"event {const.DRIVE_READY_FOR_PHYSICAL_REMOVAL} for drive {failed_drive_name} not found"
        # 14. get Node ID on which drive resides, obtain path for affected drive, identify node name for corresponding node id and remove drive
        failed_drive_path = failed_drive["spec"]["Path"]
        assert failed_drive_path, f"Drive path for drive {failed_drive_name} not found"
        logging.info(f"drive_path: {failed_drive_path}")

        scsi_id = self.drive_utils[node_ip].get_scsi_id(failed_drive_path)
        assert scsi_id, f"scsi_id for drive {failed_drive_name} not found"
        logging.info(f"scsi_id: {scsi_id}")

        self.drive_utils[node_ip].remove(scsi_id)
        logging.info(f"drive {failed_drive_path}, {scsi_id} removed")
        # 15. check driveCR succesfully removed -> only removed one, the second still should be on the cluster
        assert self.utils.check_drive_cr_exist_or_not(
            drive_name=failed_drive_name, cr_existence=False
        ), f"Drive CR {failed_drive_name} still exists"

        healthy_drive_name = healthy_drive["metadata"]["name"]
        assert self.utils.check_drive_cr_exist_or_not(
            drive_name=healthy_drive_name,
            cr_existence=True,
        ), f"Drive CR {healthy_drive_name} does not exist"
        # 16. check for events DriveSuccessfullyRemoved in kubernetes events
        assert self.utils.event_in(
            resource_name=failed_drive_name,
            reason=const.DRIVE_SUCCESSFULLY_REMOVED,
        ), f"event {const.DRIVE_SUCCESSFULLY_REMOVED} for drive {failed_drive_name} not found"
