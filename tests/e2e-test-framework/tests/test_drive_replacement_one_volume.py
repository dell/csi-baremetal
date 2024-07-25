import logging
from typing import Dict
import pytest

import framework.const as const

from framework.sts import STS
from framework.utils import Utils
from framework.drive import DriveUtils


class TestAutoDriveReplacementWithOneVolumePerPod:
    @classmethod
    @pytest.fixture(autouse=True)
    def setup_class(
        cls,
        namespace: str,
        drive_utils_executors: Dict[str, DriveUtils],
        utils: Utils,
    ):
        cls.namespace = namespace
        cls.name = "test-auto-drive-replacement-one-volume"
        cls.timeout = 120
        cls.replicas = 1

        cls.utils = utils

        cls.drive_utils = drive_utils_executors
        cls.sts = STS(cls.namespace, cls.name, cls.replicas)
        cls.sts.delete()
        cls.sts.create(storage_classes=[const.HDD_SC])

        yield

        cls.sts.delete()

    @pytest.mark.hal
    def test_5771_auto_drive_replacement_with_one_volume_per_pod(self):
    # 1. get volume for deployed pod
        assert (
            self.sts.verify(self.timeout) is True
        ), f"STS: {self.name} failed to reach desired number of replicas: {self.replicas}"
        pod = self.utils.list_pods(name_prefix=self.name)[0]
        node_ip = self.utils.get_pod_node_ip(
            pod_name=pod.metadata.name, namespace=self.namespace
        )
        volumes = self.utils.list_volumes(pod_name=pod.metadata.name)
        assert ( len(volumes) is 1 ), f"volumes: Unexpected number of volumes: {len(volumes)}"
        volume = volumes[0]

        # get drive
        drive = self.utils.get_drive_cr(
            volume_name=volume["metadata"]["name"],
            namespace=volume["metadata"]["namespace"])

    # 2.1 simulate drive failure. Annotate drive used by pod with health=BAD
        drive_name = drive["metadata"]["name"]
        self.utils.annotate_custom_resource(
            resource_name=drive_name,
            resource_type="drives",
            annotation_key="health",
            annotation_value="BAD"
        )
        logging.info(f"drive: {drive_name} was annotated with health=BAD")

    # 2.2 wait until drive health is BAD, status=ONLINE, usage=RELEASING.
        drive_name = drive["metadata"]["name"]
        logging.info(f"Waiting for drive: {drive_name}")
        assert self.utils.wait_drive(
            name=drive_name,
            expected_status=const.STATUS_ONLINE,
            expected_health=const.HEALTH_BAD,
            expected_usage=const.USAGE_RELEASING
        ), f"Drive {drive_name} failed to reach expected Status: {const.STATUS_ONLINE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_RELEASING}"
        logging.info(f"drive {drive_name} went in Status: {const.STATUS_ONLINE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_RELEASING}")

    # 2.3. wait until volume health is BAD, status=OPERATIVE, usage=RELEASING.
        volume_name = volume["metadata"]["name"]
        logging.info(f"Waiting for volume: {volume_name}")
        assert self.utils.wait_volume(
            name=volume_name,
            expected_health=const.HEALTH_BAD,
            expected_usage=const.USAGE_RELEASING,
            expected_operational_status=const.STATUS_OPERATIVE
        ), f"Volume {volume_name} failed to reach OperationalStatus: {const.STATUS_OPERATIVE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_RELEASING}"
        logging.info(f"volume {volume_name} went in OperationalStatus: {const.STATUS_OPERATIVE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_RELEASING}")

    # 3. check events and locate event related to DriveHealthFailure
        drive_name = drive["metadata"]["name"]
        assert self.utils.event_in(
            resource_name=drive_name,
            reason=const.DRIVE_HEALTH_FAILURE,
        ), f"event {const.DRIVE_HEALTH_FAILURE} for drive {drive_name} not found"

    # 6.1. annotate volume with release=done
        volume_name = volume["metadata"]["name"]
        self.utils.annotate_custom_resource(
            resource_name=volume_name,
            resource_type="volumes",
            annotation_key="release",
            annotation_value="done",
            namespace=volume['metadata']['namespace']
        )
        logging.info(f"volume: {volume_name} was annotated with release=done")

    # 6.2. check drive usages are RELEASED
        assert self.utils.wait_drive(
            name=drive['metadata']['name'],
            expected_usage=const.USAGE_RELEASED
        ), f"Drive {drive_name} failed to reach expected Usage: {const.USAGE_RELEASED}"
        logging.info(f"drive {drive_name} went in Usage: {const.USAGE_RELEASED}")

    # 6.3. check volume is RELEASED
        assert self.utils.wait_volume(
            name=volume['metadata']['name'],
            expected_usage=const.USAGE_RELEASED
        ), f"Volume {volume_name} failed to reach expected Usage {const.USAGE_RELEASED}"
        logging.info(f"volume {volume_name} went in Usage: {const.USAGE_RELEASED}")

    # 7.check event DriveReadyForRemoval is generated, check events and locate event related to VolumeBadHealth
        drive_name = drive["metadata"]["name"]
        assert self.utils.event_in(
            resource_name=drive_name,
            reason=const.DRIVE_READY_FOR_REMOVAL,
        ), f"event {const.DRIVE_READY_FOR_REMOVAL} for drive {drive_name} not found"

        volume_name = volume["metadata"]["name"]
        assert self.utils.event_in(
            resource_name=volume_name,
            reason=const.VOLUME_BAD_HEALTH,
        ), f"event {const.VOLUME_BAD_HEALTH} for volume {volume_name} not found"

    # 8. delete pod and pvc
        self.utils.clear_pvc_and_pod(pod_name=pod.metadata.name, namespace=self.namespace)

    # 9.  check drive status to be REMOVING or REMOVED
    # 10. check LED state to be 1 (if drive supports LED ) or 2 (if drive does not support LED)
    # 11. check drive status to be ONLINE
        assert self.utils.wait_drive(
            name=drive['metadata']['name'],
            expected_status=const.STATUS_ONLINE,
            expected_usage=const.USAGE_REMOVED,
            expected_health=const.HEALTH_BAD,
            expected_led_state=const.LED_STATE
        ), f"Drive {drive_name} failed to reach expected Status: {const.STATUS_ONLINE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_REMOVED}, LEDState: {drive["spec"]["LEDState"]}"
        logging.info(f"drive {drive_name} went in Status: {const.STATUS_ONLINE}, Health: {const.HEALTH_BAD}, Usage: {const.USAGE_REMOVED}, LEDState: {drive["spec"]["LEDState"]}")

    # 12. check for events: DriveReadyForPhysicalRemoval
        drive_name = drive["metadata"]["name"]
        assert self.utils.event_in(
            resource_name=drive_name,
            reason=const.DRIVE_READY_FOR_PHYSICAL_REMOVAL,
        ), f"event {const.DRIVE_READY_FOR_PHYSICAL_REMOVAL} for drive {drive_name} not found"

    # 13. get Node ID on which drive resides, Obtain path for affected drive, identify node name for corresponding node id and remove drive
        drive_name = drive["metadata"]["name"]
        drive_path = drive["spec"]["Path"]
        assert drive_path, f"Drive path for drive {drive_name} not found"
        logging.info(f"drive_path: {drive_path}")

        scsi_id = self.drive_utils[node_ip].get_scsi_id(drive_path)
        assert scsi_id, f"scsi_id for drive {drive_name} not found"
        logging.info(f"scsi_id: {scsi_id}")

    # 14. remnove drive
        self.drive_utils[node_ip].remove(scsi_id)
        logging.info(f"drive {drive_path}, {scsi_id} removed")

    # 15. check driveCR succesfully removed
        drive_name = drive["metadata"]["name"]
        assert self.utils.check_drive_cr_not_exist(
            drive_name=drive_name
        ), f"Drive CR {drive_name} still exists"

    # 16. check for events DriveSuccessfullyRemoved in kubernetes events
        drive_name = drive["metadata"]["name"]
        assert self.utils.event_in(
            resource_name=drive_name,
            reason=const.DRIVE_SUCCESSFULLY_REMOVED,
        ), f"event {const.DRIVE_SUCCESSFULLY_REMOVED} for drive {drive_name} not found"

