import socket
import threading
import select
import logging
import time
import paramiko
import pytest


class Tunnel:
    def __init__(self, jump_ip: str, target_ip: str, wiremock_ip: str, wiremock_port: int, username: str, password: str) -> None:
        self.reverse_tunnel_port = 443
        self.ssh_port = 22
        self.enabled = False
        self.accept_timeout = 2

        self.jump_ip = jump_ip
        self.target_ip = target_ip
        self.wiremock_ip = wiremock_ip
        self.wiremock_port = wiremock_port
        self.username = username
        self.password = password

        self.jump = None
        self.target = None
        self.acceptor = None

    def start(self, timeout: int = 300) -> None:
        start_time = time.time()
        time_interval = 10
        count = 0

        while True:
            count += time_interval
            logging.info(f"Waiting for tunnel connection...{count}s")
            time.sleep(time_interval)

            if time.time() - start_time > timeout:
                pytest.fail(f"Failed to establish tunnel connection. Timeout {timeout}s exceeded.")

            try:
                self.jump = paramiko.SSHClient()
                self.jump.set_missing_host_key_policy(paramiko.AutoAddPolicy())
                self.jump.connect(self.jump_ip,
                                  username=self.username,
                                  password=self.password)

                jump_transport = self.jump.get_transport()
                jump_channel = jump_transport.open_channel("direct-tcpip",
                                                           (self.target_ip, self.ssh_port),
                                                           ('127.0.0.1', self.ssh_port))

                self.target = paramiko.SSHClient()
                self.target.set_missing_host_key_policy(paramiko.AutoAddPolicy())
                self.target.connect(self.target_ip,
                                    username=self.username,
                                    password=self.password,
                                    sock=jump_channel)

                target_transport = self.target.get_transport()

                self.enabled = True
                self.acceptor = threading.Thread(target=self.__reverse_tunnel,
                                                 args=(self.reverse_tunnel_port,
                                                       self.wiremock_ip,
                                                       self.wiremock_port,
                                                       target_transport))
                self.acceptor.daemon = True
                self.acceptor.start()

                logging.info("Tunnel connection established.")
                return

            except Exception as exc:
                logging.warning(f"Failed to establish tunnel connection. Reason: {str(exc)}")

    def stop(self) -> None:
        self.enabled = False
        self.acceptor.join()
        self.target.close()
        self.jump.close()

    def __reverse_tunnel(self, server_port: int, remote_host: str, remote_port: int, transport) -> None:
        transport.request_port_forward("", server_port)
        while self.enabled:
            chan = transport.accept(self.accept_timeout)
            if chan is None:
                continue
            thr = threading.Thread(
                target=self.__reverse_tunnel_handler, args=(chan, remote_host, remote_port)
            )
            thr.daemon = True
            thr.start()

    def __reverse_tunnel_handler(self, chan, host: str, port: int) -> None:
        sock = socket.socket()
        try:
            sock.connect((host, port))
        except Exception as exc:
            logging.error(exc)
            pytest.fail(f"Forwarding request to {host}:{port} failed: {exc}")
            return

        logging.info(f"Connected!  Tunnel open {chan.origin_addr} -> {chan.getpeername()} -> {host, port}")
        while True:
            read, _, _ = select.select([sock, chan], [], [])
            if sock in read:
                data = sock.recv(1024)
                if len(data) == 0:
                    break
                chan.send(data)
            if chan in read:
                data = chan.recv(1024)
                if len(data) == 0:
                    break
                sock.send(data)
        chan.close()
        sock.close()
        logging.info(f"Tunnel closed from {chan.origin_addr}")