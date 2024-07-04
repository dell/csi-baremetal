import logging
import docker


class Docker:
    @classmethod
    def is_docker_running(cls):
        try:
            client = docker.from_env()
            client.ping()

            logging.info("\nDocker is running.")
            return True
        except Exception as exc:
            logging.error(f"Error: {exc}")
            return False