from typing import Any, List, Tuple

import logging
import paramiko


class SSHCommandExecutor:
    """
    A class for executing SSH commands on a remote server.

    Args:
        ip_address (str): The IP address of the SSH server.
        username (str): The username for authentication.
        password (str): The password for authentication.
    """

    def __init__(self, ip_address: str, username: str, password: str) -> None:
        """
        Initializes the SSHCommandExecutor with the given IP address, username, and password.
        """
        self.ip_address = ip_address
        self.username = username
        self.password = password

    def exec(self, command: str) -> Tuple[str, List[Any]]:
        """
        Executes an SSH command on the remote server.

        Args
            command (str): The command to execute.

        Returns:
            str: The output of the executed command.
            list: A list of error messages, if any, from the executed command.
        """
        ssh_client = paramiko.SSHClient()
        ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        ssh_client.connect(
            self.ip_address, username=self.username, password=self.password)

        logging.info(f"SSH connected, executing command: {command}")
        _, stdout, stderr = ssh_client.exec_command(command)
        output = stdout.read().decode().strip()
        error = stderr.readlines()

        ssh_client.close()

        if len(error) > 0:
            logging.error(f"SSH command {command} failed: {error}")

        return output, error