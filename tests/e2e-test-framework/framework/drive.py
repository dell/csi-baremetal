import json
import logging
from typing import Any, List, TypedDict
from framework.ssh import SSHCommandExecutor

class DriveChild(TypedDict):
    type: str
    children: List[str]

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
        logging.debug(f"lsblk output for {disk}:\n{entry}")
        assert len(entry) == 1, f"Found {len(entry)} drives for requested disk {disk}"
        while entry[0].find('  ') >= 0:
            entry[0] = entry[0].replace('  ', ' ')

        logging.debug(f"final lsblk string: {entry[0]}")
        return entry[0].split(' ')[1].split(':')[0]
    
    def _get_drives_to_wipe(self, lsblk_out: dict) -> dict[str, DriveChild]:
        """
        Retrieves the drives to wipe based on the lsblk output.

        Args:
            lsblk_out (dict): The lsblk output containing information about the block devices.

        Returns:
            dict[str, DriveChild]: A dictionary mapping the drive names to the DriveChild objects.
        """
        to_wipe = {}
        for drive in lsblk_out['blockdevices']:
            children = drive.get('children')
            if children:
                for child in children:
                    mountpoints = child.get('mountpoints', [])
                    mountpoints = [mountpoint for mountpoint in mountpoints if mountpoint]
                    if len(mountpoints) == 0:
                        logging.info(f"found drive \"/dev/{drive['name']}\" with child \"{child['name']}\" with no mountpoints.")
                        drive_type = to_wipe.get(drive['name'], {'children': []})
                        drive_type['type'] = child['type']
                        drive_type['children'].append(child['name'])
                        to_wipe[drive['name']] = drive_type
                    else:
                        logging.warning(f"found drive with OS: \"/dev/{drive['name']}\", skipping...")
                        break
        return to_wipe

    def _remove_csi_device_mapper(self, child_name: str) -> None:
        """
        Removes the CSI device mapper for the given device name.

        Args:
            child_name (str): The name of the device to remove the CSI device mapper for.
        """
        all_children, errors = self.executor.exec("ls -l /dev/mapper | grep dm | grep pvc | awk '{print $9}'")
        self._handle_errors(errors)
        param = "'{print $11}'"
        if all_children:
            for child in all_children.splitlines():
                if child == child_name:
                    csi_dm_cmd = f"ls -l /dev/mapper | grep dm | grep \"{child_name}\" | awk {param} | sed -e 's|..|/dev|'"
                    csi_dm, errors = self.executor.exec(csi_dm_cmd)
                    self._handle_errors(errors)
                    self.executor.exec(f"sudo dmsetup remove {csi_dm}")
                    return

    def _exec_dd(self, device_name: str) -> None:
        """
        Executes the "dd" command on the given device.

        Args:
            device_name (str): The name of the device on which to execute the "dd" command.
        """
        logging.warning(f"dd executing on device: {device_name}")
        dd_out, _ = self.executor.exec(f"sudo dd if=/dev/zero of=/dev/{device_name} bs=4096 count=1024")
        logging.warning(f"executed dd: {device_name}, output: {dd_out}")

    def _exec_wipefs(self, device_name: str) -> None:
        """
        Executes the "wipefs" command on the given device.

        Args:
            device_name (str): The name of the device on which to execute the "wipefs" command.

        Returns:
            None: This function does not return any value.

        """
        logging.warning(f"wiping: {device_name}")
        wipe_out, errors = self.executor.exec(f"sudo wipefs -af /dev/{device_name}")
        self._handle_errors(errors)
        logging.warning(f"wiped: {device_name}, output: {wipe_out}")

    def wipe_drives(self) -> None:
        """
        Wipes the drives by executing the necessary commands.

        This function retrieves the list of drives from the lsblk command and performs the wiping operation on each drive.
        It iterates over the drives and their children, and executes the wiping operation based on the drive type.
        The wiping operation is performed on the drive itself and its children.
        """ 
        output, errors = self.executor.exec("lsblk --json")
        self._handle_errors(errors)
        output = json.loads(output)
        drives_to_wipe = self._get_drives_to_wipe(lsblk_out=output)
        logging.warning(f"drives to wipe: {drives_to_wipe}")

        for drive, children in drives_to_wipe.items():
            if children['type'] == "part":
                for child in children['children']:
                    self._exec_wipefs(device_name=child)
                self._exec_wipefs(device_name=drive)
            elif children['type'] == "lvm":
                self._exec_wipefs(device_name=drive)
                self._exec_dd(device_name=drive)
                for child in children['children']:
                    self._remove_csi_device_mapper(child_name=child)
            else:
                raise Exception(f"Unknown drive type: {children['type']}")

    def _get_device_name(self, device_path_or_name: str) -> str:
        return device_path_or_name[5:] if device_path_or_name.startswith("/dev/") else device_path_or_name

    def _handle_errors(self, errors: List[Any] | None) -> None:
        assert errors is None or len(
            errors) == 0, f"remote execution failed: {errors}"
