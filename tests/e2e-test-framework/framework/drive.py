import logging
from typing import Any, List
from framework.ssh import SSHCommandExecutor


class DriveUtils:
    def __init__(self, executor: SSHCommandExecutor) -> None:
        self.executor = executor

    def get_scsi_id(self, device_name: str) -> str:
        """
        Retrieves the SCSI ID for the given device name.

        Args:
            device_name (str): The name of the device. It can be either a path
                (starting with "/dev/") or a device name.

        Returns:
            str: The SCSI ID of the device.

        """
        device_name = self._get_device_name(device_path_or_name=device_name)
        scsi_id, errors = self.executor.exec(
            f"sudo ls /sys/block/{device_name}/device/scsi_device/")
        self._handle_errors(errors)
        return scsi_id

    def remove(self, scsi_id: str) -> None:
        """removes a device from the system using the SCSI ID or device name"""
        logging.info(f"removing drive SCSI ID: {scsi_id}")
        _, errors = self.executor.exec(
            f"echo 1 | sudo tee -a /sys/class/scsi_device/{scsi_id}/device/delete")
        self._handle_errors(errors)

    def restore(self, host_num: int) -> None:
        """restores the drive for a specified host number"""
        logging.info(f"restoring drive for host: {host_num}")
        _, errors = self.executor.exec(
            f"echo '- - -' | sudo tee -a /sys/class/scsi_host/host{host_num}/scan")
        self._handle_errors(errors)

    def get_host_num(self, drive_path_or_name: str) -> int:
        """
        Retrieves the host number associated with the specified drive path or name.

        Args:
            drive_path_or_name (str): The path or name of the drive. It can be either a path starting with "/dev/" or a device name.

        Returns:
            int: The host number associated with the drive.
        """
        disk = self._get_device_name(drive_path_or_name)
        logging.info(f"getting host number for disk: {disk}")
        lsblk_output, errors = self.executor.exec("lsblk -S")
        lsblk_output = lsblk_output.split('\n')
        self._handle_errors(errors)

        entry = [e for e in lsblk_output if e.find(disk) >= 0]
        logging.debug('lsblk output for {}:\n{}'.format(disk, entry))
        assert len(entry) == 1, 'Found {} drives for requested disk {}'.format(
            len(entry), disk)

        while entry[0].find('  ') >= 0:
            entry[0] = entry[0].replace('  ', ' ')

        logging.debug('final lsblk string: {}'.format(entry[0]))
        return entry[0].split(' ')[1].split(':')[0]

    def _get_device_name(self, device_path_or_name: str) -> str:
        return device_path_or_name[5:] if device_path_or_name.startswith("/dev/") else device_path_or_name

    def _handle_errors(self, errors: List[Any] | None) -> None:
        assert errors is None or len(
            errors) == 0, f"remote execution failed: {errors}"
