import paramiko


class Ssh:
    def execute_ssh_command(self, ip_address: str, command: str, username: str, password: str) -> str:
        ssh_client = paramiko.SSHClient()
        ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        ssh_client.connect(ip_address, username=username, password=password)

        # pylint: disable=unused-variable
        stdin, stdout, stderr = ssh_client.exec_command(command)
        output = stdout.read().decode().strip()
        error = stderr.readlines()

        ssh_client.close()

        return output, error