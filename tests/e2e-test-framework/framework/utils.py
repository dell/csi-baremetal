import os
import json
import time
import copy
import logging
import base64
import subprocess
import re
from typing import List
import pytest
import requests
from kubernetes.stream import stream
import yaml
from framework.http_share import HttpShare
from framework.ssh import Ssh
from framework.kubernetes_helper import KubernetesHelper

MIN_RAM_SIZE_PER_HOST = 124
MIN_WORKER_COUNT = 3


class CceUtils:
    def __init__(self, vm_user: str, vm_cred: str, namespace: str, ansible_server: str, hosts: list = None):
        self.cce_group = 'cce-operator.dell.com'
        self.cce_operator_app = 'cce-operator'
        self.version = 'v1'
        self.alpha_version = 'v1alpha1'
        self.scanning_plural = 'scannings'
        self.profile_plural = 'profiles'
        self.check_plural = 'checks'
        self.checkloader_plural = 'checkloaders'
        self.vm_user = vm_user
        self.vm_cred = vm_cred
        self.namespace = namespace
        self.hosts = hosts
        self.ansible_server = ansible_server
        self.bundle_pv = "/platform/cce/"
        self.core_v1_api, self.custom_objects_api, self.apps_v1_api, self.network_v1_api, self.coordination_v1_api = KubernetesHelper.load_kube_api()
        self.default_timeout = 10

    def get_yaml_path(self, file_name: str) -> str:
        current_path = os.getcwd()
        yaml_path = os.path.join(current_path, f"yaml/{file_name}")
        return yaml_path

    def read_source_file_from_local(self, source_file_path: str, file_format='yaml') -> str:
        with open(source_file_path, 'r', encoding="utf-8") as file:
            if file_format == 'yaml':
                file_content = yaml.load(file, Loader=yaml.Loader)
            elif file_format == 'json':
                file_content = json.load(file)
            elif file_format == 'shell' or 'cer':
                file_content = file.read()
            else:
                raise TypeError(f"File format is not 'yaml' or 'json'. Reading from {file_format} is not supported yet.")
        return file_content

    def cce_update_scanning_yaml(self, file_name: str, scan_name: str, check_names: list, cluster_check=False, profile_name='',
                                 additional_params='', host_test: bool = False, scrt_name: str = '', cmd: str = 'SCAN', scanning_mode: str = 'sequential',) -> None:

        try:
            logging.info("[STAGE] Update scanning yaml with namespace and nodes/hosts.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info("Scanning dict before update.")
            logging.info(yaml_dict)
            hosts = []

            if cluster_check:
                controlplane_ips = ''
            elif host_test:
                controlplane_ips = ''
                for host_ip in self.hosts:
                    hosts.append({'host_ip': host_ip, 'secret': scrt_name})
            else:
                controlplane_ips = ",".join(self.get_controlplane_ips())
                logging.info(controlplane_ips)

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = scan_name

            if len(additional_params) > 0:
                logging.info(f'Additional params: {additional_params}')
                additional_param_list = [{'additional_param_name': 'optional_json', 'additional_param_value': additional_params}]
            else:
                additional_param_list = []

            yaml_dict['spec']['command'] = cmd
            yaml_dict['spec']['env']['additional_params'] = additional_param_list
            yaml_dict['spec']['env']['nodes'] = controlplane_ips
            yaml_dict['spec']['env']['hosts'] = hosts
            yaml_dict['spec']['env']['check_list'] = check_names
            yaml_dict['spec']['env']['profile_name'] = profile_name
            yaml_dict['spec']['env']['mode'] = scanning_mode

            if len(profile_name) > 0:
                yaml_dict['spec']['env']['scan_type'] = '-p'
            else:
                yaml_dict['spec']['env']['scan_type'] = '-c'

            logging.info("Updated scanning dict.")
            logging.info(yaml_dict)

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False,
                               explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Scanning deployment file '{yaml_path}' has been updated.")

            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            assert False, 'Scanning deployment file was not updated.'

    def cce_update_profile_yaml(self, file_name: str, profile_name: str, description: str, plugins: List, scanning_mode: str = 'sequential', scope: str = '') -> None:

        try:
            logging.info("[STAGE] Update profile yaml with new parameters.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info("Profile dict before update.")
            logging.info(yaml_dict)

            profile = profile_name.split('.')[0]
            profile_alias = profile.upper().replace('-', '_')

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = profile_name
            yaml_dict['spec']['description'] = description
            yaml_dict['spec']['plugins'] = plugins
            yaml_dict['spec']['profile'] = profile
            yaml_dict['spec']['profile-alias'] = profile_alias
            yaml_dict['spec']['scanning-mode'] = scanning_mode
            yaml_dict['spec']['scope'] = scope

            logging.info("Updated profile dict.")
            logging.info(yaml_dict)

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False,
                               explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Profile file '{yaml_path}' has been updated.")

            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Profile file was not updated.')

    def cce_update_checkloader_yaml(self, file_name: str, checkloader_name: str, source_name: str, source_type: str,
                                    source_uri: str, bundle_name: str,
                                    http_share_ip: str = None) -> None:

        try:
            logging.info("[STAGE] Update CheckLoader yaml with new parameters.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info("CheckLoader dict before update.")
            logging.info(yaml_dict)

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = checkloader_name

            yaml_dict['spec']['sources'][0]['name'] = source_name
            yaml_dict['spec']['sources'][0]['type'] = source_type
            if source_uri == "pv":
                yaml_dict['spec']['sources'][0]['uri'] = self.bundle_pv
            elif http_share_ip is None:
                http_share_ip = self.get_http_share_ingress_upload_ip()
                yaml_dict['spec']['sources'][0]['uri'] = f'https://[{http_share_ip}]{source_uri}' if '::' in http_share_ip else f'https://{http_share_ip}{source_uri}'
            else:
                yaml_dict['spec']['sources'][0]['uri'] = f'https://[{http_share_ip}]{source_uri}' if '::' in http_share_ip else f'https://{http_share_ip}{source_uri}'

            yaml_dict['spec']['sources'][0]['bundleName'] = bundle_name

            logging.info("Updated CheckLoader dict.")
            logging.info(yaml_dict)

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False,
                               explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"CheckLoader file '{yaml_path}' has been updated.")

            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('CheckLoader file was not updated.')

    def get_yaml_body(self, file_name: str) -> str:
        yaml_path = self.get_yaml_path(file_name)
        return self.read_source_file_from_local(yaml_path)

    def cce_create_custom_object(self, group: str, version: str, plural: str, body: str) -> None:
        response = self.custom_objects_api.create_namespaced_custom_object(group, version, self.namespace, plural, body)
        assert response is not None, "Failed to create custom resource"
        logging.info(f'\n[ASSERT] {plural} {response["metadata"]["name"]} has been created.')

    def cce_delete_custom_object(self, group: str, version: str, plural: str, resource_name: str, timeout: int = 20) -> None:
        try:
            response = self.custom_objects_api.delete_namespaced_custom_object(group, version, self.namespace, plural, resource_name)
            if plural == self.check_plural:
                start_time = time.time()
                while True:
                    try:
                        self.cce_describe_check(resource_name)
                        if time.time() - start_time > timeout:
                            pytest.fail(f'{plural} {resource_name} has NOT been deleted')
                        time.sleep(1)
                        logging.info(f'Waiting for {resource_name} check deletion...{time.time() - start_time}s')
                    except Exception:
                        with pytest.raises(Exception):
                            self.cce_describe_check(resource_name)
                        logging.info(f'[ASSERT_BY_EXCEPTION] {plural} {resource_name} has been deleted.')
                        break
            else:
                assert response['status'] == 'Success', f'{plural} {resource_name} has not been deleted.'
                logging.info(f'[ASSERT] {plural} {resource_name} has been deleted.')
        except Exception as exc:
            logging.warning(f"Failed to delete custom resource. Reason: {str(exc)}")

    def cce_delete_scanning(self, scanning_name: str) -> None:
        self.cce_delete_custom_object(self.cce_group, self.version, self.scanning_plural, scanning_name)

    def cce_create_scanning(self, body: str) -> None:
        return self.cce_create_custom_object(self.cce_group, self.version, self.scanning_plural, body)

    def cce_delete_profile(self, profile_name: str) -> None:
        self.cce_delete_custom_object(self.cce_group, self.version, self.profile_plural, profile_name)

    def cce_create_profile(self, body: str) -> None:
        self.cce_create_custom_object(self.cce_group, self.version, self.profile_plural, body)

    def cce_delete_checkloader(self, checkloader_name: str) -> None:
        self.cce_delete_custom_object(self.cce_group, self.alpha_version, self.checkloader_plural, checkloader_name)

    def cce_create_checkloader(self, body: str) -> None:
        self.cce_create_custom_object(self.cce_group, self.alpha_version, self.checkloader_plural, body)

    def cce_verify_scanning_status(self, scanning_name: str, scan_result: str, timeout=30, host_test=False) -> None:

        try:
            time_interval = 1
            is_scanning_completed = False

            while not is_scanning_completed or timeout > 0:
                logging.info(f"Waiting for the scanning execution status...{timeout} second(s)")
                time.sleep(time_interval)
                response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)

                if "status" in response:
                    if response['status']['scanStatus'] != 'IN-PROCESS':
                        is_scanning_completed = True
                        logging.info("Scanning is finished.")
                        break
                    else:
                        logging.info("Scanning is 'IN-PROCESS' status.")

                timeout -= time_interval

                if timeout == 0:
                    self.print_cce_log_content()
                    self.print_cce_operator_pod_logs()
                    self.pretty_print(response)
                    raise RuntimeError("Timed out waiting for scanning status")

            assert response['status']['scanResult'] == scan_result, f"Scan Result is not {scan_result}"
            logging.info(f"[ASSERT] Scan Result is {scan_result}")

            assert response['status']['scanStatus'] == 'COMPLETED', "Scan Status is not COMPLETED"
            logging.info("[ASSERT] Scan Status is COMPLETED")
        except Exception as exc:
            logging.error(exc)
            self.print_logs(host_test)
            pytest.fail("Scanning result/status verification failed.")

    def cce_verify_checkloader_status(self, checkloader_name: str, checkloader_status: str, timeout=60, check_timeout=True) -> None:
        try:
            time_interval = 1
            is_load_completed = False

            response = ''

            while not is_load_completed and timeout > 0:
                logging.info(f"Waiting for the CheckLoader execution status...{timeout} second(s)")
                time.sleep(time_interval)
                response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.alpha_version, self.namespace, self.checkloader_plural, checkloader_name)
                if "status" in response:
                    if response['status']['loadStatus'] != '':
                        is_load_completed = True
                        logging.info("CheckLoader is finished.")
                    else:
                        logging.info("CheckLoader has not finished yet.")

                timeout -= time_interval

                if timeout == 0 and check_timeout:
                    self.print_cce_log_content()
                    self.print_cce_operator_pod_logs()
                    self.pretty_print(response)
                    pytest.fail("Timed out waiting for load status")

            assert response['status']['loadStatus'] == checkloader_status, \
                f"CheckLoader Status is {response['status']['loadStatus']}, not {checkloader_status}"
            logging.info(f"[ASSERT] CheckLoader status is {checkloader_status}")
        except Exception as exc:
            logging.error(exc)
            pytest.fail("Checkloader result/status verification failed.")

    def cce_verify_checkloader_missing_status(self, checkloader_name: str, timeout=60) -> None:
        try:
            response = ""
            time_interval = 1
            is_load_completed = False

            while not is_load_completed and timeout > 0:
                logging.info(f"Waiting for the CheckLoader execution without status...{timeout} second(s)")
                time.sleep(time_interval)
                response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.alpha_version, self.namespace, self.checkloader_plural, checkloader_name)
                if "status" not in response:
                    is_load_completed = True

                timeout -= time_interval

                if timeout == 0:
                    self.print_cce_log_content()
                    self.print_cce_operator_pod_logs()
                    pytest.fail("Timed out waiting for load status")

            self.pretty_print(response)

            logging.info("[ASSERT] CheckLoader status is Success")
        except Exception as exc:
            logging.error(exc)
            pytest.fail("Checkloader result/status verification failed.")

    def cce_verify_checkloader_content(self, checkloader_name: str, checkloader: dict, expected_status: dict) -> None:
        try:
            logging.info("Verifying CheckLoader content fetched from K8s")

            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.alpha_version, self.namespace, self.checkloader_plural, checkloader_name)

            self.pretty_print(response)

            assert response['metadata']['namespace'] == checkloader['metadata']['namespace'], 'Checkloader namespace is unexpected.'
            logging.info(["[ASSERT] Checkloader namespace is as expected."])

            assert response['metadata']['name'] == checkloader['metadata']['name'], 'Checkloader name is unexpected.'
            logging.info(["[ASSERT] Checkloader name is as expected."])

            assert response['spec']['sources'][0]['name'] == checkloader['spec']['sources'][0]['name'], 'Checkloader sources name is unexpected.'
            logging.info(["[ASSERT] Checkloader sources name is as expected."])

            assert response['spec']['sources'][0]['type'] == checkloader['spec']['sources'][0]['type'], 'Checkloader sources type is unexpected.'
            logging.info(["[ASSERT] Checkloader sources type is as expected."])

            assert response['spec']['sources'][0]['uri'] == checkloader['spec']['sources'][0]['uri'], 'Checkloader sources uri is unexpected.'
            logging.info(["[ASSERT] Checkloader sources uri is as expected."])

            assert response['spec']['sources'][0]['bundleName'] == checkloader['spec']['sources'][0]['bundleName'], 'Checkloader sources bundleName is unexpected.'
            logging.info(["[ASSERT] Checkloader sources bundleName is as expected."])

            assert response['spec']['sources'][0]['action'] == checkloader['spec']['sources'][0]['action'], 'Checkloader sources action is unexpected.'
            logging.info(["[ASSERT] Checkloader sources action is as expected."])

            if 'processedChecks' in expected_status['processedChecksStatus']:
                expected_status['processedChecksStatus']['processedChecks'].sort(key=lambda x: x['name'])
                response['status']['processedChecksStatus']['processedChecks'].sort(key=lambda x: x['name'])

            if 'processedProfiles' in expected_status['processedProfilesStatus']:
                expected_status['processedProfilesStatus']['processedProfiles'].sort(key=lambda x: x['name'])
                response['status']['processedProfilesStatus']['processedProfiles'].sort(key=lambda x: x['name'])

            if 'removedChecks' in expected_status['removedChecksStatus']:
                expected_status['removedChecksStatus']['removedChecks'].sort(key=lambda x: x['name'])
                response['status']['removedChecksStatus']['removedChecks'].sort(key=lambda x: x['name'])

            if 'removedProfiles' in expected_status['removedProfilesStatus']:
                expected_status['removedProfilesStatus']['removedProfiles'].sort(key=lambda x: x['name'])
                response['status']['removedProfilesStatus']['removedProfiles'].sort(key=lambda x: x['name'])

            assert all(item in response['status'].items() for item in expected_status.items()), f'Checkloader response status is unexpected.\n{response["status"]}\nvs\n{expected_status}'
            # assert response['status'] == expected_status, f'Checkloader response status is unexpected.\n{response["status"]}\nvs\n {expected_status}'
            logging.info(["[ASSERT] Checkloader response status is as expected."])

        except Exception as exc:
            logging.error(exc)
            pytest.fail('K8s CheckLoader verification failed.')

    def cce_verify_checkloader_error(self, checkloader_name: str, error_message: str) -> None:
        try:
            logging.info("Verifying CheckLoader content fetched from K8s")

            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.alpha_version, self.namespace, self.checkloader_plural, checkloader_name)

            self.pretty_print(response)

            assert response['status']['error'].startswith(error_message), f'CheckLoader error message is unexpected. {response["status"]["error"]}'
            logging.info(f"[ASSERT] CheckLoader error message is as expected. {response['status']['error']}")
        except Exception as exc:
            logging.error(exc)
            pytest.fail('K8s CheckLoader verification failed.')

    def cce_verify_bundles_pv_exist_on_all_pods(self) -> None:
        cce_pods = self.get_cce_pod_names()

        for cce_pod_name in cce_pods:
            try:
                logging.info(f"Verifying CCE bundle directory on pod: {cce_pod_name}")
                exec_command = [
                    "ls",
                    self.bundle_pv
                ]
                bundle_pv_result = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command,
                                          stderr=True, stdin=False, stdout=True, tty=False)
                logging.info(f"Bundle PV result: {bundle_pv_result}")
                assert "bundle" in bundle_pv_result.splitlines(), f"Bundle directory not found on pod {cce_pod_name}"
            except Exception as exc:
                logging.error(exc)
                pytest.fail('CCE bundle directory verification failed.')

    def cce_verify_bundles_single_plugin(self, check_id: str, is_there: bool) -> None:
        try:
            logging.info("Verifying plugin directory fetched from CCE")
            cce_pod_name = self.get_cce_leader_name_from_logs()

            exec_command = [
                "ls",
                "plugins/1.0/"]
            actual_checks = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command,
                                   stderr=True, stdin=False, stdout=True, tty=False)
            assert (check_id in actual_checks.splitlines()) ^ (not is_there), \
                f"Plugin directory is {'not ' if is_there else ''}there"
            logging.info(f"[ASSERT] Plugin directory is {'not ' if not is_there else ''}there")

        except Exception as exc:
            logging.error(exc)
            pytest.fail('CCE plugin directory verification failed.')

    def cce_verify_bundles(self, expected_checks: List, expected_profiles: List) -> None:
        try:
            logging.info("Verifying bundle content fetched from CCE")
            cce_pod_name = self.get_cce_leader_name_from_logs()

            exec_command = [
                "ls",
                "plugins/1.0.2/"]
            actual_checks = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command,
                                   stderr=True, stdin=False, stdout=True, tty=False)
            assert actual_checks.splitlines() == expected_checks, f"Bundle plugins content does not match \nactual:{actual_checks.splitlines()} \n vs \n{expected_checks}"
            logging.info(f"[ASSERT] Bundle plugins content matches:{actual_checks.splitlines()} \n vs \n{expected_checks}")

            exec_command = [
                "ls",
                "conf/profile"]
            actual_profiles = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command,
                                     stderr=True, stdin=False, stdout=True, tty=False)
            assert actual_profiles.splitlines() == expected_profiles, f"Bundle profile content does not match:{actual_checks.splitlines()} \n vs \n{expected_checks}"
            logging.info(f"[ASSERT] Bundle profile content matches:{actual_checks.splitlines()} \n vs \n{expected_checks}")

        except Exception as exc:
            logging.error(exc)
            pytest.fail('CCE bundle verification failed.')

    def cce_verify_profile_content(self, profile_name: str, profile: dict) -> None:
        try:
            logging.info("Verifying profile content fetched from K8s")

            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.profile_plural, profile_name)

            self.pretty_print(response)

            assert response['metadata']['namespace'] == profile['metadata']['namespace']
            assert response['metadata']['name'] == profile['metadata']['name']
            assert response['spec']['description'] == profile['spec']['description'], \
                f"Description mismatch: expected {profile['spec']['description']} actual: {response['spec']['description']}"
            assert response['spec']['plugins'].sort() == profile['spec']['plugins'].sort(), \
                f"Plugins mismatch: expected {profile['spec']['plugins']} actual: {response['spec']['plugins']}"
            assert response['spec']['profile'] == profile['spec']['profile'], \
                f"Profile mismatch: expected {profile['spec']['profile']} actual: {response['spec']['profile']}"
            assert response['spec']['profile-alias'] == profile['spec']['profile-alias'], \
                f"Profile alias mismatch: expected {profile['spec']['profile-alias']} actual: {response['spec']['profile-alias']}"
            assert response['spec']['scanning-mode'] == profile['spec']['scanning-mode'], \
                f"Scanning mode mismatch: expected {profile['spec']['scanning-mode']} actual: {response['spec']['scanning-mode']}"
            assert response['spec']['scope'] == profile['spec']['scope'], \
                f"Scope mismatch: expected {profile['spec']['scope']} actual: {response['spec']['scope']}"

        except Exception as exc:
            logging.error(exc)
            pytest.fail('K8s Profile verification failed.')

    def cce_verify_profile_content_from_disk(self, profile_file_name: str, profile: dict) -> None:

        try:
            logging.info("Verifying profile content fetched from CCE")

            exec_command = [
                "/bin/sh",
                "-c",
                f"cat /home/app/cce/conf/profile/{profile_file_name}"]

            cce_pod_name = self.get_cce_leader_name_from_logs()
            response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command,
                              stderr=True, stdin=False, stdout=True, tty=False)

            profile_disk = yaml.safe_load(response)

            logging.info(profile_disk)

            assert profile_disk['description'] == profile['spec']['description'], \
                f"Description mismatch: expected {profile['spec']['description']} actual: {profile_disk['description']}"
            assert profile_disk['plugins'].sort() == profile['spec']['plugins'].sort(), \
                f"Plugins mismatch: expected {profile['spec']['plugins']} actual: {profile_disk['plugins']}"
            assert profile_disk['profile'] == profile['spec']['profile'], \
                f"Profile mismatch: expected {profile['spec']['profile']} actual: {profile_disk['profile']}"
            assert profile_disk['profile-alias'] == profile['spec']['profile-alias'], \
                f"Profile alias mismatch: expected {profile['spec']['profile-alias']} actual: {profile_disk['profile-alias']}"
            assert profile_disk['execution-mode'] == profile['spec']['scanning-mode'], \
                f"Scanning mode mismatch: expected {profile['spec']['scanning-mode']} actual: {profile_disk['execution-mode']}"
            assert profile_disk['scope'] == profile['spec']['scope'], \
                f"Scope mismatch: expected {profile['spec']['scope']} actual: {profile_disk['scope']}"

        except Exception as exc:
            logging.error(exc)
            pytest.fail('CCE Profile verification failed.')

    def cce_verify_scanning_report(self, scanning_name: str, check_name: str, exp_symptom: str, scan_result: str, cluster_check: bool = False, host_test: bool = False) -> None:
        try:
            logging.info("[STAGE] Checking scanning execution report.")

            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)

            self.pretty_print(response)

            scan_report = json.loads(response['status']['report'])

            logging.info(f'Scan report details:\n{scan_report}')

            # check checks
            assert scan_report['checks'] == check_name, "Wrong scan report check name"
            logging.info("[ASSERT] Scan report - successful check name verification.")

            # check status
            assert scan_report["status"] == "COMPLETED", "Wrong scan report status"
            logging.info("[ASSERT] Scan report - successful status verification.")

            # check result
            if not cluster_check:
                if host_test:
                    node_count = len(self.hosts)
                else:
                    node_count = len(self.get_controlplane_ips())

                assert scan_report["result"] == f'{node_count} {scan_result}', "Wrong scan report result"
                logging.info("[ASSERT] Scan report - successful result verification.")

            # check results symptom
            if len(exp_symptom) > 0:
                for host_check in scan_report["results"]["host_checks"]:
                    assert host_check["result"]["messages"], "Scan report host check result message is empty"
                    logging.info("[ASSERT] Scan report host check result message is not empty")

                    act_symptom = host_check["result"]["messages"][0]["symptom"]
                    assert exp_symptom == act_symptom, f"Wrong scan report results host check symptom.\nACTUAL:{act_symptom}\nEXPECTED:{exp_symptom}"
                    logging.info(f"[ASSERT] Scan report - successful host checks symptom verification - {act_symptom}")

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning report verification failed.')

    def cce_verify_scanning_report_for_callback_api(
            self, scanning_name: str, check_name: str, exp_symptoms: list[str], scan_result: str, profile_name: str = None, cluster_check: bool = False, host_test: bool = False) -> None:
        try:
            logging.info("[STAGE] Checking scanning execution report.")

            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)

            logging.info(response)

            scan_report = json.loads(response['status']['report'])

            logging.info(f'Scan report details:\n{scan_report}')

            # check checks
            if check_name:
                assert scan_report['checks'] == check_name, "Wrong scan report check name"
            if profile_name:
                assert scan_report['profile'] == profile_name, "Wrong scan report check name"
            logging.info("[ASSERT] Scan report - successful check name verification.")

            # check status
            assert scan_report["status"] == "COMPLETED", "Wrong scan report status"
            logging.info("[ASSERT] Scan report - successful status verification.")

            # check result
            if not cluster_check:
                if host_test:
                    node_count = len(self.hosts)
                else:
                    node_count = len(self.get_controlplane_ips())

                assert scan_report["result"] == f'{node_count} {scan_result}', "Wrong scan report result"
                logging.info("[ASSERT] Scan report - successful result verification.")

            for idx, api_check in enumerate(scan_report["results"]["api_checks"]):
                logging.info(f'API check index: {idx}')

                # todo add service_url check

                # symptom
                exp_symptom = exp_symptoms[idx % len(exp_symptoms)]

                # check results symptom
                if len(exp_symptom) > 0:
                    assert api_check["result"]["messages"], "Scan report api check result message is empty"
                    logging.info("[ASSERT] Scan report api check result message is not empty")

                    act_symptom = api_check["result"]["messages"][0]["symptom"]
                    assert exp_symptom == act_symptom, f"Wrong scan report results api check symptom.\nACTUAL:{act_symptom}\nEXPECTED:{exp_symptom}"
                    logging.info(f"[ASSERT] Scan report - successful host checks symptom verification - {act_symptom}")

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning report verification failed.')

    def cce_verify_node_logs(self, check_name: str, scan_result: str, host_test: bool = False) -> None:
        logging.info("[STAGE] Checking node logs.")

        exec_command_ls = [
            "/bin/sh",
            "-c",
            "ls /home/app/cce/log"]

        try:
            if host_test:
                controlplane_ips = self.hosts
            else:
                controlplane_ips = self.get_controlplane_ips()

            cce_pod_name = self.get_cce_leader_name_from_logs()
            response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command_ls,
                              stderr=True, stdin=False, stdout=True, tty=False)

            available_logs = response.splitlines()
            logging.info(f'Available logs: {available_logs}')

            assert any(".log" in element for element in available_logs if ".log" in element), 'No logs available on the cce pod.'
            logging.info('[ASSERT] Cce logs are on the cce-operator pod.')

            for controlplane_ip in controlplane_ips:
                log_name = f'{controlplane_ip}.log'

                exec_command_cat = [
                    "/bin/sh",
                    "-c",
                    f"cat /home/app/cce/log/{log_name}"]

                assert log_name in available_logs, f'{log_name} does not exist on cce-operator pod.'
                logging.info(f"[ASSERT] {log_name} exists on cce-operator pod.")

                response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command_cat,
                                  stderr=True, stdin=False, stdout=True, tty=False)
                log_content = response.lower()
                logging.info(f'Kubernetes stream response:\n{log_content}')

                assert check_name in log_content, f'{check_name} does not exist {log_name}'
                logging.info(f"[ASSERT] {check_name} exists {log_name}")

                assert all([word in log_content for word in
                            [f'"result": "{scan_result.lower()}"', f'"severity": "{scan_result.lower()}"']]), f'Scan result {scan_result} does not exist in {log_name} file.'
                logging.info(f"[ASSERT] Scan result {scan_result} exists {log_name} file.")

                if scan_result == "ERROR":
                    log_content = log_content.lower()\
                        .replace('"result": "error"', '"result": "replaced-text"').replace('"severity": "error"', '"severity": "replaced-text"')\
                        .replace("'severity': 'error'", "'severity': 'replaced-text'")

                logging.info(f'Kubernetes stream response (replaced scan status "error"):\n{log_content}')

                assert self.check_password_in_logs(log_content), "Passwords are not properly masked or set to default"

                assert not any([word in log_content for word in ['error', 'exception']]), f'Errros / Exceptions in {log_name} file'
                logging.info(f"[ASSERT] No Errros / Exceptions in {log_name} file.")
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Logs verification failed.')

    def check_password_in_logs(self, log_content):
        host_user_password_pattern = re.compile(r'(hosts|localhost)=([^@]+@[^:]+):(\*+|default)')
        lines = log_content.splitlines()

        for line_number, line in enumerate(lines, 1):
            match = host_user_password_pattern.search(line)
            if match:
                password = match.group(3)
                if not (password == "default" or password != "********"):
                    logging.info(f"[ASSERT] Password not properly masked or is incorrect on line {line_number}: {line.strip()}")
                    return False
        logging.info("[ASSERT] All passwords in the host@user:password format are properly masked or set to default.")
        return True

    def cce_verify_cce_log(self, check_name: str, symptom: list[str], scan_result: str, is_error_expected: bool = False) -> None:
        logging.info("[STAGE] Checking if cce logs does not contain errors.")

        try:
            exec_command_ls = [
                "/bin/sh",
                "-c",
                "ls /home/app/cce/log"]
            cce_pod_name = self.get_cce_leader_name_from_logs()
            response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command_ls,
                              stderr=True, stdin=False, stdout=True, tty=False)
            available_logs = response.splitlines()
            logging.info(f'Available logs: {available_logs}')

            assert any(".log" in element for element in available_logs if ".log" in element), 'No logs available on the cce pod.'
            logging.info('[ASSERT] Cce logs are on the cce-operator pod.')

            exec_command_cat = [
                "/bin/sh",
                "-c",
                "cat /home/app/cce/log/cce.log"]

            response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command_cat,
                              stderr=True, stdin=False, stdout=True, tty=False).lower()

            assert check_name in response, "Check name does not exist in cce.log file for last report."
            logging.info(f"[ASSERT] Check name '{check_name}' exists in cce.log file for last report.")

            assert symptom in response, f"Symptom '{symptom}' does not exist in cce.log file for last report."
            logging.info(f"[ASSERT] Symptom '{symptom}' exists  in cce.log file for last report.")

            if is_error_expected:
                assert any([word in response for word in ['error', 'exception']]), "No Error found in cce.log file for last report."
                logging.info("[ASSERT] Expected Errros/ Exceptions found in cce.log file for last report.")
            else:
                if scan_result == "ERROR":
                    response = response.replace('"result": "error"', '').replace('"severity": "error"', '')

                assert not any([word in response for word in ['error', 'exception']]), "Error in cce.log file for last report."
                logging.info("[ASSERT] No Errros/ Exceptions in cce.log file for last report.")

        except Exception as exc:
            logging.error(exc)
            pytest.fail('cce.log verification failed.')

    def get_cce_log_content(self, wait=0.01) -> str:
        logging.info("[STAGE] Get cce log content.")
        try:
            logging.info(f"Waiting for the cce log...{wait} second(s)")
            time.sleep(wait)
            exec_command_cat = [
                "/bin/sh",
                "-c",
                "cat /home/app/cce/log"]

            cce_pod_name = self.get_cce_leader_name_from_logs()

            exec_command_cat = [
                "/bin/sh",
                "-c",
                "cat /home/app/cce/log/cce.log"]

            return stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command_cat,
                          stderr=True, stdin=False, stdout=True, tty=False).lower()

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Getting cce.log content failed.')

    def print_cce_log_content(self) -> None:
        logging.info("CCE Log content:")
        logging.info(self.get_cce_log_content())

    def get_controlplane_ips(self):
        nodes = self.core_v1_api.list_node().items
        controlplane_nodes = [node for node in nodes if "node-role.kubernetes.io/control-plane" in node.metadata.labels]
        assert controlplane_nodes, "No control plane nodes found in the cluster"
        logging.info("[ASSERT] Control plane nodes found in the cluster.")

        controlplane_ips = [node.status.addresses[0].address for node in controlplane_nodes]
        assert controlplane_ips, "No IP addresses found for control plane nodes"
        logging.info(f"[ASSERT] IP addresses found for control plane nodes - {controlplane_ips}")

        return controlplane_ips

    def get_host_ips(self):
        return self.hosts

    def get_controlplane_names(self):
        nodes = self.core_v1_api.list_node().items
        controlplane_nodes = [node for node in nodes if "node-role.kubernetes.io/control-plane" in node.metadata.labels]
        assert controlplane_nodes, "No control plane nodes found in the cluster"
        logging.info("[ASSERT] Control plane nodes found in the cluster.")

        controlplane_names = [node.status.addresses[1].address for node in controlplane_nodes]
        assert controlplane_names, "No names found for control plane nodes"
        logging.info(f"[ASSERT] Names found for control plane nodes - {controlplane_names}")

        return controlplane_names

    def get_worker_ips(self) -> list:
        nodes = self.core_v1_api.list_node().items
        worker_nodes = [node for node in nodes if "node-role.kubernetes.io/control-plane" not in node.metadata.labels]
        assert worker_nodes, "No worker nodes found in the cluster"
        logging.info("[ASSERT] Worker nodes found in the cluster.")

        worker_ips = [node.status.addresses[0].address for node in worker_nodes]
        assert worker_ips, "No IP addresses found for worker nodes"
        logging.info(f"[ASSERT] IP addresses found for worker nodes - {worker_ips}")

        return worker_ips

    def get_worker_count(self) -> int:
        nodes = self.core_v1_api.list_node().items
        worker_nodes = [node for node in nodes if "node-role.kubernetes.io/worker" in node.metadata.labels]

        return len(worker_nodes)

    def cce_verify_scanning_profile_report(self, scanning_name: str, exp_profile_name: str, exp_report_result: str, exp_check_ids: str, exp_symptoms: str) -> None:
        try:
            logging.info("[STAGE] Checking scanning execution report.")

            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)

            scan_report = json.loads(response['status']['report'])

            logging.info(f'Scan report details:\n{scan_report}')

            # check checks
            assert scan_report['profile'] == exp_profile_name, "Wrong scan report profile name"
            logging.info("[ASSERT] Scan report - successful profile name verification.")

            # check status
            assert scan_report["status"] == "COMPLETED", "Wrong scan report status"
            logging.info("[ASSERT] Scan report - successful status verification.")

            # check result

            assert scan_report["result"] == exp_report_result, "Wrong scan report result"
            logging.info("[ASSERT] Scan report - successful result verification.")

            # check results symptom
            act_host_checks = scan_report["results"]["host_checks"]

            for idx, act_host_check in enumerate(act_host_checks):
                logging.info(f'Host check index: {idx}')

                # check_id
                exp_check_id = exp_check_ids[idx % len(exp_check_ids)]
                assert act_host_check["check_id"] == exp_check_id, f"Wrong scan report results host check id - {act_host_check['check_id']} vs {exp_check_id}"
                logging.info(f'[ASSERT] {act_host_check["host_id"]} - scan report - successful host checks check id verification "{exp_check_id}".')

                # symptom
                exp_symptom = exp_symptoms[idx % len(exp_symptoms)]

                if len(exp_symptom) > 0:
                    act_symptom = act_host_check["result"]["messages"][0]["symptom"]
                    assert act_symptom == exp_symptom, f"Wrong scan report results host check symptom - {act_symptom} vs {exp_symptom}"
                    logging.info(f'[ASSERT] {act_host_check["host_id"]} - scan report - successful host checks symptom verification "{act_symptom}".')
                else:
                    assert not act_host_check["result"]["messages"], "Wrong scan report results host check messages"
                    logging.info(f'[ASSERT] {act_host_check["host_id"]} - scan report - successful host checks symptom verification "{exp_symptom}".')

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning report verification failed.')

    def cce_verify_scanning_multiple_checks_report(self, scanning_name: str, exp_report_result: str, exp_check_ids: str, exp_symptoms: str) -> None:
        try:
            logging.info("[STAGE] Checking scanning execution report for multiple checks.")

            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)

            self.pretty_print(response)

            scan_report = json.loads(response['status']['report'])

            logging.info(f'Scan report details:\n{scan_report}')

            # check check names
            assert scan_report['checks'] == ','.join(exp_check_ids), "Wrong scan report check names"
            logging.info("[ASSERT] Scan report - successful check names verification.")

            # check status
            assert scan_report["status"] == "COMPLETED", "Wrong scan report status"
            logging.info("[ASSERT] Scan report - successful status verification.")

            # check result
            assert scan_report["result"] == exp_report_result, f'Wrong scan report result {scan_report["result"]} vs {exp_report_result}'
            logging.info("[ASSERT] Scan report - successful result verification.")

            # check results symptom
            act_host_checks = scan_report["results"]["host_checks"]

            for idx, act_host_check in enumerate(act_host_checks):
                logging.info(f'Host check index: {idx}')

                # check_id
                exp_check_id = exp_check_ids[idx % len(exp_check_ids)]
                assert act_host_check["check_id"] == exp_check_id, f"Wrong scan report results host check id - {act_host_check['check_id']} vs {exp_check_id}"
                logging.info(f'[ASSERT] {act_host_check["host_id"]} - scan report - successful host checks check id verification "{exp_check_id}".')

                # symptom
                exp_symptom = exp_symptoms[idx % len(exp_symptoms)]

                if len(exp_symptom) > 0:
                    act_symptom = act_host_check["result"]["messages"][0]["symptom"]
                    assert act_symptom == exp_symptom, f"Wrong scan report results host check symptom - {act_symptom} vs {exp_symptom}"
                    logging.info(f'[ASSERT] {act_host_check["host_id"]} - scan report - successful host checks symptom verification "{act_symptom}".')
                else:
                    assert not act_host_check["result"]["messages"], "Wrong scan report results host check messages"
                    logging.info(f'[ASSERT] {act_host_check["host_id"]} - scan report - successful host checks symptom verification "{exp_symptom}".')

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning report verification failed.')

    def cce_verify_node_logs_zypper(self, check_name: str) -> None:
        try:
            exec_command_ls = [
                "/bin/sh",
                "-c",
                "ls /home/app/cce/log"]

            cce_pod_name = self.get_cce_leader_name_from_logs()
            response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command_ls,
                              stderr=True, stdin=False, stdout=True, tty=False)

            available_logs = response.splitlines()
            logging.info(f'Available logs: {available_logs}')

            assert any(".log" in element for element in available_logs if ".log" in element), 'No logs available on the cce pod.'
            logging.info('[ASSERT] Cce logs are on the cce-operator pod.')

            controlplane_ips = self.get_controlplane_ips()

            for controlplane_ip in controlplane_ips:
                log_name = f'{controlplane_ip}.log'

                zypper_with_version = Ssh.execute_ssh_command(self, ip_address=controlplane_ip, command='zypper --version', username=self.vm_user, password=self.vm_cred)[0]
                exec_command_cat = [
                    "/bin/sh",
                    "-c",
                    f"cat /home/app/cce/log/{log_name}"]

                response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command_cat,
                                  stderr=True, stdin=False, stdout=True, tty=False)
                log_content = response.lower()

                logging.info(f'Kubernetes stream response from {log_name}:\n{log_content}')

                assert check_name in log_content, f'{check_name} does not exist {log_name}'
                logging.info(f"[ASSERT] {check_name} exists {log_name}")

                assert zypper_with_version in log_content, f"'{zypper_with_version}' does not exist in {log_name} file\n{check_name}"
                logging.info(f"[ASSERT] '{zypper_with_version}' exists in {log_name} file.")
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Logs verification failed.')

    def is_zypper_on_controlplanes(self) -> bool:

        controlplane_ips = self.get_controlplane_ips()
        for controlplane_ip in controlplane_ips:
            zypper_with_version = Ssh.execute_ssh_command(self, ip_address=controlplane_ip, command='zypper --version', username=self.vm_user, password=self.vm_cred)[0]
            if not zypper_with_version:
                return False
        return True

    def cce_get_total_cores_number(self) -> int:
        try:
            controlplane_ips = self.get_controlplane_ips()
            cpu_counter = 0

            for controlplane_ip in controlplane_ips:
                cpu_number = self.cce_get_cores_number(controlplane_ip)
                cpu_counter = cpu_counter + cpu_number

            logging.info(f'Total cpu number for {controlplane_ips}: {cpu_counter}')

            return cpu_counter

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Total cpu cores counter failed.')

    def cce_get_cores_number(self, controlplane_ip: str) -> int:
        try:
            cpu_counter = Ssh.execute_ssh_command(self, ip_address=controlplane_ip, command='nproc', username=self.vm_user, password=self.vm_cred)[0]
            logging.info(f'Cpu number for {controlplane_ip}: {cpu_counter}')
            return int(cpu_counter)

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Cpu cores counter failed.')

    def get_persistent_storage(self, controlplane_ip: str) -> int:
        try:
            storage = Ssh.execute_ssh_command(self, ip_address=controlplane_ip, command="df -k /dev/system/root | tail -1 | tr -s ' ' | cut -d' ' -f4", username=self.vm_user, password=self.vm_cred)[0]
            logging.info(f'Persistent storage for {controlplane_ip}: {int(storage)} bytes')
            return int(storage)
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Getting persistent storage failed.')

    def get_persistent_storage_in_gb(self, controlplane_ip: str) -> int:
        storage = self.get_persistent_storage(controlplane_ip) // 2**20
        logging.info(f'Persistent storage for {controlplane_ip}: {storage} GB')
        return storage

    def cce_get_min_cores_number(self, use_host: bool = False) -> int:
        try:
            if not use_host:
                controlplane_ips = self.get_controlplane_ips()
            else:
                controlplane_ips = self.hosts
            cpu_min_counter = 1000

            for controlplane_ip in controlplane_ips:
                cpu_number = self.cce_get_cores_number(controlplane_ip)
                cpu_min_counter = min(cpu_number, cpu_min_counter)

            logging.info(f'Minimum cpu number for {controlplane_ips}: {cpu_min_counter}')

            return cpu_min_counter

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Minimum cpu cores counter failed.')

    def cce_get_ram_size(self, controlplane_ip: str) -> int:
        try:
            command = "free -b  | sed -n -e 's/^Mem: *\\([0-9]*\\).*/\\1/p'"
            ram_size = Ssh.execute_ssh_command(self, ip_address=controlplane_ip, command=command, username=self.vm_user, password=self.vm_cred)[0]
            logging.info(f'RAM size for {controlplane_ip}: {ram_size}')
            return int(ram_size)

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Checking RAM size failed.')

    def cce_get_min_ram_size(self) -> int:
        try:
            controlplane_ips = self.get_controlplane_ips()
            ram_size_min = MIN_RAM_SIZE_PER_HOST * pow(2, 30)

            for controlplane_ip in controlplane_ips:
                ram_size = self.cce_get_ram_size(controlplane_ip)
                ram_size_min = min(ram_size, ram_size_min)

            logging.info(f'Minimum RAM size for {controlplane_ips}: {ram_size_min}')

            return ram_size_min

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Minimum RAM size checking failed.')

    def get_node_ip_for_pod(self, pod_name: str) -> str:
        pod = self.core_v1_api.read_namespaced_pod(name=pod_name, namespace=self.namespace)
        node_name = pod.spec.node_name
        node = self.core_v1_api.read_node(name=node_name)
        addresses = node.status.addresses
        for addr in addresses:
            if addr.type == "InternalIP":
                return addr.address
        return None

    def get_node_name_for_pod(self, pod_name: str) -> str:
        pod = self.core_v1_api.read_namespaced_pod(name=pod_name, namespace=self.namespace)
        return pod.spec.node_name

    def get_pod_ip(self, pod_name: str) -> str:
        pod = self.core_v1_api.read_namespaced_pod(name=pod_name, namespace=self.namespace)
        return pod.status.pod_ip

    def clear_cce_logs(self, ip_address: str) -> None:
        command_rm = "rm /home/app/cce/log/*"
        Ssh.execute_ssh_command(self, ip_address=ip_address, command=command_rm, username=self.vm_user, password=self.vm_cred)
        logging.info(f'CCE logs deleted on {ip_address}.')

        command_touch = "touch /home/app/cce/log/cce.log"
        Ssh.execute_ssh_command(self, ip_address=ip_address, command=command_touch, username=self.vm_user, password=self.vm_cred)
        logging.info(f'cce.log file created on {ip_address}.')

        self.show_cce_logs_by_ip(ip_address)

    def clear_cce_logs_on_cce_operator_node(self) -> None:
        logging.info("[STAGE] Cleaning cce logs on cce-operator node.")
        cce_pod_name = self.get_cce_leader_name_from_logs()
        cce_node_ip = self.get_node_ip_for_pod(cce_pod_name)

        self.clear_cce_logs(cce_node_ip)

    def clear_cce_logs_on_hosts(self):
        for host_ip in self.hosts:
            self.clear_cce_logs(host_ip)

    def show_cce_logs_by_ip(self, ip_address: str) -> None:
        command_ls = "ls /home/app/cce/log"
        command_output = Ssh.execute_ssh_command(self, ip_address=ip_address, command=command_ls, username=self.vm_user, password=self.vm_cred)[0]
        logging.info(f'Available logs on node {ip_address}# ls /home/app/cce/log\n{command_output}')

    def show_cce_logs(self) -> None:
        cce_pod_name = self.get_cce_leader_name_from_logs()
        cce_node_ip = self.get_node_ip_for_pod(cce_pod_name)

        self.show_cce_logs_by_ip(cce_node_ip)

    def get_sles_version(self, node_ip: str) -> None:
        command_cat = "cat /etc/os-release | grep VERSION="
        command_output = Ssh.execute_ssh_command(self, ip_address=node_ip, command=command_cat, username=self.vm_user, password=self.vm_cred)[0]
        logging.info(f'Available SLES version on node {node_ip}# ls /home/app/cce/log\n{command_output}')
        sles_version = command_output.replace('VERSION=', '').replace('\"', '')
        return sles_version

    def cce_delete_check(self, check_name: str) -> None:
        time.sleep(1)
        self.cce_delete_custom_object(self.cce_group, self.version, self.check_plural, check_name)

    def cce_update_check_yaml(self, file_name: str, check_name: str, spec_command: str, spec_description: str, spec_check_id: str, spec_check_name: str) -> None:
        try:
            logging.info("Update check yaml with check details.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info(f"Check dict before update.\n{yaml_dict}")

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = check_name
            yaml_dict['spec']['command'] = spec_command
            yaml_dict['spec']['description'] = spec_description
            yaml_dict['spec']['id'] = spec_check_id
            yaml_dict['spec']['name'] = spec_check_name

            logging.info(f"Updated scanning dict.\n{yaml_dict}")

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False, explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Check yaml file '{yaml_path}' has been updated.")
            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Check yaml file was not updated.')

    def cce_create_check(self, body: str) -> None:
        self.cce_create_custom_object(self.cce_group, self.version, self.check_plural, body)

    def describe_resource(self, plural: str, resource_name: str) -> json:
        return self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, plural, resource_name)

    def cce_describe_check(self, check_name: str) -> json:
        return self.describe_resource(self.check_plural, check_name)

    def update_custom_object(self, plural: str, name: str, body: str) -> None:
        response = self.custom_objects_api.patch_namespaced_custom_object(self.cce_group, self.version, self.namespace, plural, name, body)
        assert response is not None, "Failed to update custom resource"
        logging.info(f'\n[ASSERT] {plural} {response["metadata"]["name"]} has been updated with {body}.')
        self.pretty_print(response)

    def cce_update_check(self, check_name: str, body: str) -> None:
        self.update_custom_object(self.check_plural, check_name, body)

    def cce_create_scrt(self, body: str) -> None:
        response = self.core_v1_api.create_namespaced_secret(self.namespace, body)
        assert response is not None, "Failed to create scrt"
        logging.info("[ASSERT] Scrt has been created.")

    def cce_delete_scrt(self, scrt_name: str) -> None:
        try:
            response = self.core_v1_api.delete_namespaced_secret(scrt_name, self.namespace)
            assert response is not None, "Failed to delete scrt"
            logging.info(f"[ASSERT] Scrt '{scrt_name}' has been deleted.")
        except Exception as exc:
            logging.warning(f"Failed to delete scrt. Reason: {str(exc)}")

    def cce_update_scrt_yaml(self, file_name: str, scrt_name: str, passwd="", username="") -> None:
        try:
            logging.info("Update scrt yaml with check details.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info(f"Scrt dict before update.\n{yaml_dict}")

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = scrt_name

            yaml_dict['data']['username'] = base64.b64encode(bytes(self.vm_user, "utf-8")).decode('utf-8') if username == "" else username
            yaml_dict['data']['password'] = base64.b64encode(bytes(self.vm_cred, "utf-8")).decode('utf-8') if passwd == "" else passwd

            logging.info(f"Updated scrt dict.\n{yaml_dict}")

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False, explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Scrt yaml file '{yaml_path}' has been updated.")
            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scrt yaml file was not updated.')

    def cce_check_docker(self):
        try:
            host_ips = self.hosts

            for host_ip in host_ips:
                response = Ssh.execute_ssh_command(self, ip_address=host_ip, command='docker ps', username=self.vm_user, password=self.vm_cred)[0]
                logging.info(response)
                assert 'CONTAINER ID' in str(response), f'Docker is not installed in {host_ip}.'

            logging.info(f'Docker is installed on all hosts: {host_ips}')
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Docker is not installed on all required hosts.')

    def cce_check_image_on_hosts(self, image_name: str) -> bool:
        try:
            for host_ip in self.hosts:
                command = f'docker images -q {image_name}'
                response = Ssh.execute_ssh_command(self, ip_address=host_ip, command=command, username=self.vm_user, password=self.vm_cred)[0]
                logging.info(f'Response from {host_ip}: {response}')
                if not response.strip():
                    logging.info(f'Image {image_name} is not found on {host_ip}.')
                    return False
            logging.info(f'Image {image_name} is present on all hosts: {self.hosts}')
            return True
        except Exception as exc:
            logging.error(exc)
            return False

    def cce_delete_image_on_hosts(self, image_name: str) -> bool:
        try:
            for host_ip in self.hosts:
                command = f'docker rmi {image_name}'
                response = Ssh.execute_ssh_command(self, ip_address=host_ip, command=command, username=self.vm_user, password=self.vm_cred)[0]
                logging.info(f'Response from {host_ip}: {response}')
                if 'Error' in response or 'No such image' in response:
                    logging.info(f'Failed to delete image {image_name} on {host_ip}.')
                    return False
            logging.info(f'Image {image_name} has been deleted from all hosts: {self.hosts}')
            return True
        except Exception as exc:
            logging.error(exc)
            return False

    def cce_check_hosts(self) -> bool:
        if self.cce_count_hosts() > 0:
            logging.info(f'Defined hosts: {self.hosts}')
            return True
        else:
            logging.info('Hosts are not defined')
            return False

    def cce_count_hosts(self) -> int:
        return len(self.hosts)

    def cce_add_host(self, host: str) -> None:
        self.hosts.append(host)

    def cce_replace_host(self, host: str, index: int) -> None:
        self.hosts[index] = host

    def cce_remove_host(self, host: str) -> None:
        self.hosts.remove(host)

    def cce_copy_bundles_to_leader_pv(self, bundle_location: str) -> None:
        try:
            cce_leader_name = self.get_cce_leader_name_from_logs()
            logging.info(f"Copying bundles from {bundle_location} to CCE-operator PV")
            subprocess.run(["kubectl", "cp", bundle_location, cce_leader_name + ":" + self.bundle_pv, "-n", self.namespace])  # pylint: disable=subprocess-run-check
            self.verify_file_on_pod(cce_leader_name, self.bundle_pv, os.path.basename(bundle_location))

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Copying bundle to CCE-operator PV failed.')

    def cce_delete_bundles_from_pv(self) -> None:
        try:
            logging.info("Deleting CCE-operator bundles from PV")
            cce_pod_name = self.get_cce_leader_name_from_logs()

            exec_command = [
                "rm",
                "-frv",
                f'{self.bundle_pv}']
            self.exec_command(cce_pod_name, exec_command)

            exec_command = [
                "ls",
                self.bundle_pv]
            response = self.exec_command(cce_pod_name, exec_command)
            available_files = response.splitlines()
            assert len(available_files) == 0, f'Files {available_files} in {self.bundle_pv} on pod {cce_pod_name} has not been cleaned.'
            logging.info(f'[ASSERT] Bundles folder on pod has been cleaned up. Available files: {available_files}.')
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Deleting CCE-operator bundle from PV failed.')

    def exec_command(self, pod_name: str, exec_command: list) -> None:
        try:
            logging.info(f"Executing {exec_command} on pod {pod_name}")
            response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, pod_name, self.namespace, container="cce-operator", command=exec_command,
                              stderr=True, stdin=False, stdout=True, tty=False)
            return response
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Command execution failed with exception.')

    def cce_is_http_share_ip_exist(self):
        return len(self.get_http_share_ingress_upload_ip()) != 0

    def cce_upload_bundles_to_http_share(self, bundle_location: str, upload_folder: str) -> str:
        try:
            http_share_ip = self.get_http_share_ingress_upload_ip()
            url = f'https://[{http_share_ip}]{upload_folder}' if '::' in http_share_ip else f'https://{http_share_ip}{upload_folder}'
            bundle = HttpShare.upload_file(self, url, bundle_location)
            logging.info(f"Bundle '{bundle}' has been uploated to {url}")
            return bundle
        except Exception as exc:
            logging.error(exc)
            pytest.fail(f"Failed to upload bundle file to: {url}")

    def cce_delete_bundles_from_http_share(self, delete_folder: str) -> None:
        try:
            http_share_ip = self.get_http_share_ingress_upload_ip()
            url = f'https://[{http_share_ip}]{delete_folder}' if '::' in http_share_ip else f'https://{http_share_ip}{delete_folder}'
            logging.info(f"Bundle '{url}' has been deleted")
            return
        except Exception as exc:
            logging.warning(f"Failed to delete bundle.  Reason: {str(exc)}")

    def add_worker_node_label(self):
        label_key = "node-role.kubernetes.io/worker"
        label_value = "true"

        nodes = self.core_v1_api.list_node().items
        worker_nodes = [node for node in nodes if "node-role.kubernetes.io/control-plane" not in node.metadata.labels]
        worker_nodes_without_label = [worker_node for worker_node in worker_nodes if "node-role.kubernetes.io/worker" not in worker_node.metadata.labels]

        if not worker_nodes_without_label:
            logging.info('All worker nodes have worker label.')
            return

        for worker_node_without_label in worker_nodes_without_label:
            if label_key not in worker_node_without_label.metadata.labels:
                worker_node_without_label.metadata.labels[label_key] = label_value
                self.core_v1_api.patch_node(worker_node_without_label.metadata.name, worker_node_without_label)
                logging.info(f'{worker_node_without_label.metadata.name} has been updated with worker label')

        nodes = self.core_v1_api.list_node().items
        worker_nodes_with_label = [node for node in nodes if label_key in node.metadata.labels]

        assert len(worker_nodes_without_label) == len(worker_nodes_with_label), "Worker nodes has not been updated with 'worker' label"
        logging.info('\n[ASSERT] Worker nodes have been updated.')

    def cce_delete_test_scannings(self) -> None:
        scannnings = self.custom_objects_api.list_cluster_custom_object(self.cce_group, self.version, self.scanning_plural)["items"]
        test_scanning_names = [scanning["metadata"]["name"] for scanning in scannnings if scanning["metadata"]["name"].startswith('tc-')]

        for test_scanning_name in test_scanning_names:
            self.cce_delete_custom_object(self.cce_group, self.version, self.scanning_plural, test_scanning_name)

    def delete_pod(self, pod_name: str, check_status=True) -> None:
        try:
            self.core_v1_api.delete_namespaced_pod(pod_name, self.namespace)
            logging.info(f'{pod_name} has been deleted.')
            if check_status:
                self.check_pod_runninng_and_ready(pod_name)
        except Exception as exc:
            logging.warning(f"Failed to delete custom resource. Reason: {str(exc)}")

    def delete_pod_simple(self, pod_name: str) -> None:
        try:
            self.core_v1_api.delete_namespaced_pod(pod_name, self.namespace)
            logging.info(f'{pod_name} has been deleted.')
            time.sleep(1)
        except Exception as exc:
            logging.warning(f"Failed to delete pod. Reason: {str(exc)}")

    def check_pod_runninng_and_ready(self, pod_name: str, delay: int = 10) -> None:
        time.sleep(delay)
        assert self.is_pod_running(pod_name), f'Pod {pod_name} is not running'
        assert self.is_pod_ready(pod_name), f'Pod is {pod_name} not ready'

    def is_pod_running(self, pod_name: str, timeout=30) -> bool:
        try:
            start_time = time.time()
            count = 0
            while self.core_v1_api.read_namespaced_pod_status(pod_name, self.namespace).status.phase != 'Running':
                count += 1
                time.sleep(1)
                if time.time() - start_time > timeout:
                    logging.warning(f"POD '{pod_name}' not running after {timeout} seconds timeout.")
                    return False
                else:
                    logging.warning(f"Waiting for {pod_name} pod running...{count}s")
            logging.info(f"Pod '{pod_name}' in namespace '{self.namespace}' is running.")
            return True
        except Exception as exc:
            logging.warning(f"Failed check_cce_pod_running. Reason: {str(exc)}")
            return False

    def is_pod_pending(self, pod_name: str, timeout=10) -> bool:
        try:
            start_time = time.time()
            while self.core_v1_api.read_namespaced_pod_status(pod_name, self.namespace).status.phase != 'Pending':
                time.sleep(0.2)
                if time.time() - start_time > timeout:
                    logging.warning(f"POD '{pod_name}' not pending after {timeout} seconds timeout.")
                    return False
            logging.info(f"Pod '{pod_name}' in namespace '{self.namespace}' is pending.")
            return True
        except Exception as exc:
            logging.warning(f"Failed is_pod_pending {pod_name}. Reason: {str(exc)}")
            return False

    def is_pod_ready(self, pod_name: str, timeout=5):
        try:
            start_time = time.time()
            while True:
                time.sleep(0.2)
                pod = self.core_v1_api.read_namespaced_pod_status(pod_name, self.namespace)
                if pod.status.conditions:
                    ready_condition = next((condition for condition in pod.status.conditions if condition.type == "Ready"), None)
                    if ready_condition is not None and ready_condition.status == "True":
                        logging.info(f"Pod '{pod_name}' in namespace '{self.namespace}' is ready.")
                        time.sleep(2)
                        return True
                if time.time() - start_time > timeout:
                    logging.warning(f"Pod '{pod_name}' not ready after {timeout} seconds timeout.")
                    return False
                logging.warning(f'Waiting on pod {pod_name} to be ready...')
        except Exception as exc:
            logging.warning(f"Failed check_cce_pod_ready. Reason: {str(exc)}")
            return False

    def create_service(self, svc_name: str, body: str) -> None:
        try:
            self.core_v1_api.create_namespaced_service(self.namespace, body=body)
            logging.info(f"[ASSERT] '{svc_name}' has been created.")
        except Exception as exc:
            logging.warning(f"Failed to create service. Reason: {str(exc)}")

    def delete_service(self, svc_name: str) -> None:
        try:
            response = self.core_v1_api.delete_namespaced_service(svc_name, self.namespace)
            assert response is not None, "Failed to delete Service"
            logging.info(f"[ASSERT] Service '{svc_name}' has been deleted.")
        except Exception as exc:
            logging.warning(f"Failed to delete Service. Reason: {str(exc)}")

    def update_service_yaml(self, file_name: str, svc_name: str, pod_name: str) -> None:
        try:
            logging.info("Update svc yaml with details.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info(f"SVC dict before update.\n{yaml_dict}")

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = svc_name
            yaml_dict['metadata']['labels']['app'] = svc_name
            yaml_dict['spec']['selector']['app'] = pod_name

            logging.info(f"Updated SVC dict.\n{yaml_dict}")

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False, explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"SVC yaml file '{yaml_path}' has been updated.")
            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('SVC yaml file was not updated.')

    def create_stateful_set(self, stateful_name: str, body: str) -> None:
        try:
            self.apps_v1_api.create_namespaced_stateful_set(self.namespace, body=body)
            logging.info(f"[ASSERT] '{stateful_name}' has been created.")
        except Exception as exc:
            logging.warning(f"Failed to create stateful set. Reason: {str(exc)}")

    def update_stateful_set(self, stateful_name: str, body: str) -> None:
        try:
            response = self.apps_v1_api.patch_namespaced_stateful_set(stateful_name, self.namespace, body=body)
            assert response['status'] == 'Success', f'{stateful_name} has not been updated.'
            logging.info(f"[ASSERT] '{stateful_name}' has been updated.")
        except Exception as exc:
            logging.warning(f"Failed to update stateful set. Reason: {str(exc)}")

    def delete_stateful_set(self, sts_name: str) -> None:
        try:
            response = self.apps_v1_api.delete_namespaced_stateful_set(sts_name, self.namespace)
            assert response is not None, "Failed to delete STS"
            logging.info(f"[ASSERT] STS '{sts_name}' has been deleted.")
        except Exception as exc:
            logging.warning(f"Failed to delete STS. Reason: {str(exc)}")

    def cce_update_sts_yaml(self, file_name: str, sts_name: str, scrt_name: str, cm_name: str) -> None:
        try:
            logging.info("Update sts yaml with details.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info(f"STS dict before update.\n{yaml_dict}")

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = sts_name
            yaml_dict['metadata']['labels']['app'] = sts_name

            yaml_dict['spec']['selector']['matchLabels']['app'] = sts_name
            yaml_dict['spec']['template']['metadata']['labels']['app'] = sts_name

            yaml_dict['spec']['template']['spec']['volumes'][0]['name'] = f"{sts_name}-volume"
            yaml_dict['spec']['template']['spec']['volumes'][0]['secret']['secretName'] = scrt_name
            yaml_dict['spec']['template']['spec']['containers'][0]['volumeMounts'][0]['name'] = f"{sts_name}-volume"

            yaml_dict['spec']['template']['spec']['volumes'][1]['name'] = f"{sts_name}-tls"
            yaml_dict['spec']['template']['spec']['volumes'][1]['secret']['secretName'] = f"{scrt_name}-tls"
            yaml_dict['spec']['template']['spec']['containers'][0]['volumeMounts'][1]['name'] = f"{sts_name}-tls"

            yaml_dict['spec']['template']['spec']['volumes'][2]['name'] = cm_name
            yaml_dict['spec']['template']['spec']['volumes'][2]['configMap']['name'] = cm_name
            yaml_dict['spec']['template']['spec']['containers'][0]['volumeMounts'][2]['name'] = cm_name

            logging.info(f"Updated STS dict.\n{yaml_dict}")

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False, explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"STS yaml file '{yaml_path}' has been updated.")
            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('STS yaml file was not updated.')

    def update_cert_yaml(self, file_name: str, cert_name: str) -> None:
        try:
            logging.info("Update Cert yaml with details.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info(f"Cert dict before update.\n{yaml_dict}")

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = f"{cert_name}-cert"

            yaml_dict['spec']['commonName'] = cert_name
            yaml_dict['spec']['dnsNames'][0] = f"{cert_name}"
            yaml_dict['spec']['dnsNames'][1] = f"{cert_name}.{self.namespace}"
            yaml_dict['spec']['dnsNames'][2] = f"{cert_name}.{self.namespace}.svc"
            yaml_dict['spec']['dnsNames'][3] = f"{cert_name}.{self.namespace}.svc.cluster.local"
            yaml_dict['spec']['secretName'] = f"{cert_name}-tls"

            logging.info(f"Updated Cert dict.\n{yaml_dict}")

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False, explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Cert yaml file '{yaml_path}' has been updated.")
            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Cert yaml file was not updated.')

    def create_cert(self, body: str) -> None:
        return self.cce_create_custom_object_with_namespace("cert-manager.io", "v1", self.namespace, "certificates", body)

    def delete_cert(self, cert_name: str) -> None:
        self.cce_delete_custom_object("cert-manager.io", "v1", "certificates", f"{cert_name}-cert")

    def update_config_map_yaml(self, file_name: str, cm_name: str) -> None:
        try:
            logging.info("Update ConfigMap yaml with details.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info(f"ConfigMap dict before update.\n{yaml_dict}")

            yaml_dict['metadata']['namespace'] = self.namespace
            yaml_dict['metadata']['name'] = cm_name

            logging.info(f"Updated ConfigMap dict.\n{yaml_dict}")

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False, explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"ConfigMap yaml file '{yaml_path}' has been updated.")
            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('ConfigMap yaml file was not updated.')

    def create_config_map(self, body: str) -> None:
        try:
            response = self.core_v1_api.create_namespaced_config_map(self.namespace, body)
            assert response is not None, "Failed to create ConfigMap"
            logging.info("[ASSERT] ConfigMap has been created.")
        except Exception as exc:
            logging.warning(f"Failed to create service. Reason: {str(exc)}")

    def delete_config_map(self, cm_name: str) -> None:
        try:
            response = self.core_v1_api.delete_namespaced_config_map(cm_name, self.namespace)
            assert response is not None, "Failed to delete ConfigMap"
            logging.info(f"[ASSERT] ConfigMap '{cm_name}' has been deleted.")
        except Exception as exc:
            logging.warning(f"Failed to delete ConfigMap. Reason: {str(exc)}")

    def cce_get_cluster_objects_names(self, group: str, version: str, plural: str):
        custom_objects_names = []
        custom_objects = self.custom_objects_api.list_cluster_custom_object(group, version, plural=plural)["items"]
        for obj in custom_objects:
            custom_objects_names.append(obj["metadata"]["name"])
        return custom_objects_names

    def delete_cce_crds(self, items: list, plural_type: str):
        for item in items:
            time.sleep(0.01)
            if plural_type == 'checks':
                self.cce_delete_check(item)
            elif plural_type == "profiles":
                self.cce_delete_profile(item)
            elif plural_type == "scannings":
                self.cce_delete_scanning(item)

    def cce_read_namespaced_stateful_set(self, name: str):
        try:
            response = self.apps_v1_api.read_namespaced_stateful_set(self.cce_operator_app, self.namespace)
            logging.info(f"[ASSERT] read_namespaced_stateful_set completed successfully for '{self.namespace}' and '{name}'.")
            return response
        except Exception as exc:
            logging.warning(f"Failed to read_namespaced_stateful_set list. Reason: {str(exc)}")

    def cce_patch_namespaced_stateful_set(self, name: str, body: str):
        try:
            response = self.apps_v1_api.patch_namespaced_stateful_set(name, self.namespace, body)
            logging.info(f"[ASSERT] patch_namespaced_stateful_set completed successfully for '{self.namespace}' and '{name}'.")
            return response
        except Exception as exc:
            logging.warning(f"Failed to patch_namespaced_stateful_set. Reason: {str(exc)}")

    def cce_set_namespaced_stateful_set_env_variable(self, name: str, env_variable_name: str, value: str):
        stateful_set = self.cce_read_namespaced_stateful_set(name)
        variables = stateful_set.spec.template.spec.containers[0].env
        for variable in variables:
            if variable.name == env_variable_name:
                variable.value = value
        return stateful_set

    def cce_verify_namespaced_stateful_set_env_variable(self, name: str, env_variable_name: str, expected_value: str):
        env_variables = self.cce_read_namespaced_stateful_set(name).spec.template.spec.containers[0].env
        for variable in env_variables:
            if variable.name == env_variable_name:
                assert variable.value == expected_value, f"'{variable.name}' equals {expected_value}."
                logging.info(f"[ASSERT] Verify that '{variable.name}' value equals {expected_value}.")

    def cordon_nodes(self, label: str) -> None:
        nodes = self.core_v1_api.list_node().items

        for node in nodes:
            if label in node.metadata.labels:
                body = {
                    "spec": {
                        "unschedulable": True,
                    },
                }
                self.core_v1_api.patch_node(node.metadata.name, body)

    def uncordon_nodes(self, label: str, timeout=0) -> None:
        nodes = self.core_v1_api.list_node().items

        for node in nodes:
            if label in node.metadata.labels:
                body = {
                    "spec": {
                        "unschedulable": False,
                    },
                }
                self.core_v1_api.patch_node(node.metadata.name, body)
        time.sleep(timeout)

    def cordon_node(self, node_name: str, timeout=0) -> None:
        nodes = self.core_v1_api.list_node().items

        for node in nodes:
            if node.metadata.name == node_name:
                body = {
                    "spec": {
                        "unschedulable": True,
                    },
                }
                self.core_v1_api.patch_node(node.metadata.name, body)
                logging.info(f"Node '{node.metadata.name}' cordoned.")
                time.sleep(timeout)
                return
            else:
                logging.info(f"Node '{node.metadata.name}' is not going to be cordoned.")

    def uncordon_node(self, node_name: str, timeout=0) -> None:
        nodes = self.core_v1_api.list_node().items

        for node in nodes:
            if node.metadata.name == node_name:
                body = {
                    "spec": {
                        "unschedulable": False,
                    },
                }
                self.core_v1_api.patch_node(node.metadata.name, body)
                logging.info(f"Node '{node_name}' uncordoned.")
                time.sleep(timeout)
                return

    def get_ingress(self, ingress_name):
        try:
            ingress = self.network_v1_api.read_namespaced_ingress(name=ingress_name, namespace=self.namespace)
            logging.info(ingress)
            return ingress
        except Exception as exc:
            if "not found" in str(exc):
                pytest.skip(f'Ingress {ingress_name} does not exist {exc}')
            logging.error(exc)
            pytest.fail(f'Failed to get {ingress_name} Ingress')

    def get_http_share_ingress_upload_ip(self) -> str:
        if self.is_istio_enabled():
            return self.get_istio_ingress_ip()
        else:
            http_share_ingress_upload = self.get_ingress('httpshare-ingress-upload')
            logging.info(f'first ingress[0]: {http_share_ingress_upload.status.load_balancer.ingress[0]}')

            # Extract the first address from the Ingress rules
            if http_share_ingress_upload.status.load_balancer.ingress:
                first_load_balancer_ingress = http_share_ingress_upload.status.load_balancer.ingress[0]
                logging.info(f'httpshare-ingress-upload first_load_balancer_ingress: {first_load_balancer_ingress}')
                if first_load_balancer_ingress.ip:
                    logging.info(f'httpshare-ingress-upload first ip address: {first_load_balancer_ingress.ip}')
                    return first_load_balancer_ingress.ip
                else:
                    pytest.skip('httpshare-ingress-upload does not have any ip address')
            else:
                pytest.skip("No ingress defined in the Ingress.")

    def get_pvc_name(self, app_name: str) -> str:
        try:
            pvcs = self.core_v1_api.list_namespaced_persistent_volume_claim(namespace=self.namespace)
            for pvc in pvcs.items:
                if "app" in pvc.metadata.labels:
                    if pvc.metadata.labels["app"] == app_name:
                        logging.info(f"App '{app_name}' has been found for PVC {pvc.metadata.name}")
                        return pvc.metadata.name
        except Exception as exc:
            if str(exc) == "'app'":
                return None
            logging.error(exc)
            pytest.fail(f"Failed to get pvc name for '{app_name}'")

    def get_pvc_names(self, app_name: str) -> list:
        try:
            pvcs_list = []
            pvcs = self.core_v1_api.list_namespaced_persistent_volume_claim(namespace=self.namespace)
            for pvc in pvcs.items:
                if "app" in pvc.metadata.labels:
                    if pvc.metadata.labels["app"] == app_name:
                        logging.info(f"App '{app_name}' has been found for PVC")
                        pvcs_list.append(pvc.metadata.name)
            return pvcs_list
        except Exception as exc:
            if str(exc) == "'app'":
                return None
            logging.error(exc)
            pytest.fail(f"Failed to get pvc name for '{app_name}'")

    def delete_pvc(self, pvc_name: str) -> str:
        try:
            logging.info(f"Deleting PVC name '{pvc_name}'.")
            remove_finalizers_body = [{
                "op": "remove",
                "path": "/metadata/finalizers"
            }]
            self.core_v1_api.delete_namespaced_persistent_volume_claim(pvc_name, namespace=self.namespace)
            time.sleep(0.1)
            if self.is_pvc_exist(pvc_name):
                self.update_pvc(pvc_name, remove_finalizers_body)

            assert not self.is_pvc_exist(pvc_name), f'PVC {pvc_name} has not been deleted.'
            logging.info(f'PVC {pvc_name} has  been deleted.')
        except Exception as exc:
            logging.error(exc)
            pytest.fail(f'PVC {pvc_name} has not been deleted.')

    def update_pvc(self, pvc_name: str, body: str) -> str:
        try:
            self.core_v1_api.patch_namespaced_persistent_volume_claim(pvc_name, self.namespace, body)
            time.sleep(0.1)
            assert self.is_pvc_exist(pvc_name), f"PVC '{pvc_name}' has not been finally deleted."
            logging.info(f"App with PVC name '{pvc_name}' has been patched.")
        except Exception as exc:
            logging.error(exc)
            pytest.fail(f"Failed to patch pvc {pvc_name}")

    def delete_pvc_cce(self):
        # Get PVC name
        pvc_name = self.get_pvc_name(self.cce_operator_app)
        assert pvc_name, f"PVC does not exist for app '{self.cce_operator_app}'"
        logging.info(f"[ASSERT] PVC name {pvc_name} for {self.cce_operator_app} exists.")

        # Delete PVC
        self.delete_pvc(pvc_name)

    def get_node_name_with_running_cce_pod(self) -> str:
        try:
            pods = self.core_v1_api.list_namespaced_pod(namespace=self.namespace)
            node = [pod.spec.node_name for pod in pods.items if pod.metadata.name.startswith('cce-operator')][0]
            logging.info(f"Node '{node}' with running cce pod")
            return node
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Name for node not found.')

    def is_scanning_exist(self, scanning_name: str) -> bool:
        try:
            self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)
            return True
        except Exception as exc:
            logging.warning(f"Failed to get scanning. Reason: {str(exc)}")
            return bool("Reason: Not Found" not in str(exc))

    def get_pod_logs(self, pod_name: str) -> str:
        try:
            logging.info(f'Get Logs for pod name {pod_name}.')
            return self.core_v1_api.read_namespaced_pod_log(name=pod_name, namespace=self.namespace, container="cce-operator")
        except Exception as exc:
            logging.error(exc)
            pytest.fail(f'Failed to get kubectl logs from {pod_name}')

    def get_cce_operator_pod_logs(self) -> str:
        return self.get_pod_logs(self.get_cce_leader_name_from_logs())

    def print_cce_operator_pod_logs(self) -> None:
        logging.info('Print cce operator pod logs')
        pod_logs = self.get_cce_operator_pod_logs()
        try:
            if "Inside the init create time and status pending" not in pod_logs:
                logging.info(f'cce operator pod logs:\n\n{pod_logs}\n\n')
                pytest.fail('Failed to get last scanning.')
            else:
                last_scanning_pod_logs = pod_logs[pod_logs.rindex('Inside the init create time and status pending'):len(pod_logs) - 1]
                logging.info(f"cce-operator pod logs for last scanning :\n...\n...{last_scanning_pod_logs}")
                logging.info(f"To get all pod logs run command 'kubectl logs {self.get_cce_leader_name_from_logs()} -n {self.namespace}'")
        except Exception as exc:
            logging.error(exc)
            pytest.fail(f'Failed to get last scanning:\n{pod_logs}')

    def is_pvc_exist(self, pvc_name: str, timeout=2) -> bool:
        time.sleep(0.5)
        start_time = time.time()
        while self.get_pvc_name(pvc_name) is not None:
            time.sleep(0.5)
            if time.time() - start_time > timeout:
                return True
        return False

    def cce_verify_scanning_report_mixed_symptoms(self, scanning_name: str, check_name: str, exp_symptoms: list, scan_result: str, cluster_check: bool = False) -> None:
        try:
            logging.info("[STAGE] Checking scanning execution report.")

            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)

            self.pretty_print(response)

            scan_report = json.loads(response['status']['report'])

            logging.info(f'Scan report details:\n{scan_report}')

            # check checks
            assert scan_report['checks'] == check_name, "Wrong scan report check name"
            logging.info("[ASSERT] Scan report - successful check name verification.")

            # check status
            assert scan_report["status"] == "COMPLETED", "Wrong scan report status"
            logging.info("[ASSERT] Scan report - successful status verification.")

            # check result
            if not cluster_check:
                assert scan_report["result"] == f'{scan_result}', "Wrong scan report result"
                logging.info("[ASSERT] Scan report - successful result verification.")

            act_host_checks = scan_report["results"]["host_checks"]

            for idx, act_host_check in enumerate(act_host_checks):
                logging.info(f'Host check index: {idx}')

                # symptom
                exp_symptom = exp_symptoms[idx % len(exp_symptoms)]

                if len(exp_symptom) > 0:
                    act_symptom = act_host_check["result"]["messages"][0]["symptom"]
                    assert act_symptom == exp_symptom, f"Wrong scan report results host check symptom - {act_symptom} vs {exp_symptom}"
                    logging.info(f'[ASSERT] {act_host_check["host_id"]} - scan report - successful host checks symptom verification "{act_symptom}".')
                else:
                    assert not act_host_check["result"]["messages"], "Wrong scan report results host check messages"
                    logging.info(f'[ASSERT] {act_host_check["host_id"]} - scan report - successful host checks symptom verification "{exp_symptom}".')

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning report verification failed.')

    def get_scanning_content(self, scanning_name: str) -> str:
        try:
            return self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Failed to get scanning.')

    def cce_update_scanning_yaml_with_nodes(self, file_name: str, nodes: str) -> None:

        try:
            logging.info("[STAGE] Update scanning yaml with nodes.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            yaml_dict['spec']['env']['nodes'] = nodes

            logging.info("Updated scanning dict.")
            logging.info(yaml_dict)

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False,
                               explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Scanning deployment file '{yaml_path}' has been updated.")

            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning deployment file was not updated.')

    def is_ssd_disk_available(self, ip_adress) -> bool:
        try:
            list_available_block_devices = Ssh.execute_ssh_command(self, ip_address=ip_adress, command="lsblk -d -o name,rota,size,type", username=self.vm_user, password=self.vm_cred)[0]
            logging.info(f'{ip_adress} available block devices:\n {list_available_block_devices}')
            lines = list_available_block_devices.split('\n')
            for line in lines[1:]:
                name, rota, size, device_type = line.split()
                if device_type == "disk" and rota == '0':
                    logging.info(f'Disk {name} with size {size} is SSD')
                    return True
            logging.info(f'{ip_adress}: no SSD disk available.')
            return False
        except Exception as exc:
            logging.error(exc)
            pytest.fail(f'{ip_adress} Failed to list available block devices.')

    def update_scanning_yaml_with_mode(self, file_name: str, scanning_mode: str) -> None:

        try:
            logging.info("[STAGE] Update scanning yaml with mode.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            yaml_dict['spec']['env']['mode'] = scanning_mode

            logging.info("Updated scanning dict.")
            logging.info(yaml_dict)

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False,
                               explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Scanning deployment file '{yaml_path}' has been updated.")

            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning deployment file was not updated.')

    def set_scanning_mode_to_parallel(self, file_name: str) -> None:
        self.update_scanning_yaml_with_mode(file_name, "parallel")

    def get_scanning_created_at(self, scanning_name) -> str:
        try:
            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)
            return response['status']['createdAt']
        except Exception as exc:
            logging.error(exc)
            pytest.fail("Failed to get createdAt for {scanning_name}.")

    def get_scanning_completed_at(self, scanning_name) -> str:
        try:
            response = self.custom_objects_api.get_namespaced_custom_object(self.cce_group, self.version, self.namespace, self.scanning_plural, scanning_name)
            return response['status']['completedAt']
        except Exception as exc:
            logging.error(exc)
            pytest.fail("Failed to get completedAt for {scanning_name}.")

    def get_cce_operator_cm(self, config_map: str = "cce-operator-configmap") -> str:
        try:
            configmap = self.core_v1_api.read_namespaced_config_map(config_map, self.namespace)
            result = yaml.safe_load(configmap.data["cce-operator.yaml"])
            return result
        except Exception as exc:
            logging.error(exc)
            pytest.fail("Failed to get cce-operator configuration")

    def patch_cce_operator_cm(self, data: dict, config_map: str = "cce-operator-configmap") -> None:
        try:
            configmap = self.core_v1_api.read_namespaced_config_map(config_map, self.namespace)
            configmap.data["cce-operator.yaml"] = yaml.dump(data)
            response = self.core_v1_api.patch_namespaced_config_map(config_map, self.namespace, configmap)
            assert response.data == configmap.data, f'{config_map} has not been updated.'
            logging.info(f"[ASSERT] '{config_map}' has been updated.")
        except Exception as exc:
            logging.error(exc)
            pytest.fail(f"Failed to patch config map {config_map}")

    def is_default_route_exist(self, controlplane_ip: str) -> bool:
        default_route = Ssh.execute_ssh_command(self, ip_address=controlplane_ip, command='ip r | grep default', username=self.vm_user, password=self.vm_cred)[0]
        return 'default ' in default_route

    def is_default_routes_exist(self, host_test=False) -> bool:
        try:
            ip_addresses = self.hosts if host_test else self.get_controlplane_ips()
            for ip_address in ip_addresses:
                if not self.is_default_route_exist(ip_address):
                    logging.info(f'Default route for {ip_address} does not exist')
                    return False
            logging.info(f'Default route exists for each control plane {ip_addresses}')
            return True
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Getting default route failed.')

    def cce_create_custom_object_with_namespace(self, group: str, version: str, namespace: str, plural: str, body: str) -> None:
        response = self.custom_objects_api.create_namespaced_custom_object(group, version, namespace, plural, body)
        assert response is not None, "Failed to create custom resource"
        logging.info(f'\n[ASSERT] {plural} {response["metadata"]["name"]} has been created.')
        self.pretty_print(response)

    def cce_create_scanning_with_namespace(self, namespace: str, body: str) -> None:
        return self.cce_create_custom_object_with_namespace(self.cce_group, self.version, namespace, self.scanning_plural, body)

    def cce_update_scanning_yaml_with_namespace(self, file_name: str, namespace: str) -> None:
        try:
            logging.info("[STAGE] Update scanning yaml with namespace.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            yaml_dict['metadata']['namespace'] = namespace

            logging.info("Updated scanning dict.")
            logging.info(yaml_dict)

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False,
                               explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Scanning deployment file '{yaml_path}' has been updated.")

            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning deployment file was not updated.')

    def cce_update_scanning_yaml_with_additional_params(self, file_name: str, additional_params: list = None) -> None:
        if additional_params is None:
            additional_params = []

        try:
            logging.info("[STAGE] Update scanning yaml with additional_params")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            logging.info("Scanning dict before update.")
            logging.info(yaml_dict)

            yaml_dict['spec']['env']['additional_params'] = additional_params

            logging.info("Updated scanning dict.")
            logging.info(yaml_dict)

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False,
                               explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Scanning deployment file '{yaml_path}' has been updated.")

            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            assert False, 'Scanning deployment file was not updated.'

    def list_custom_object(self, group: str, version: str, plural: str) -> list:
        try:
            return self.custom_objects_api.list_namespaced_custom_object(group, version, self.namespace, plural)
        except Exception as exc:
            logging.warning(f"Failed to list custom resource {plural}. Reason: {str(exc)}")

    def list_scanning(self) -> list:
        return self.list_custom_object(self.cce_group, self.version, self.scanning_plural)

    def update_scanning_yaml_with_name(self, file_name: str, scanning_name: str) -> None:
        try:
            logging.info("[STAGE] Update scanning yaml with name.")

            yaml_path = self.get_yaml_path(file_name)
            yaml_dict = self.read_source_file_from_local(yaml_path)

            yaml_dict['metadata']['name'] = scanning_name

            logging.info("Updated scanning dict.")
            logging.info(yaml_dict)

            with open(yaml_path, 'wb') as yaml_file:
                yaml.safe_dump(yaml_dict, yaml_file, default_flow_style=False,
                               explicit_start=True, allow_unicode=True, encoding='utf-8')

            logging.info(f"Scanning deployment file '{yaml_path}' has been updated.")

            return yaml_dict
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Scanning deployment file was not updated.')

    def is_package_installed(self, package_name: str, host_test=False) -> bool:
        ip_addresses = self.hosts if host_test else self.get_controlplane_ips()
        for ip_address in ip_addresses:
            package_installation_status = Ssh.execute_ssh_command(self, ip_address=ip_address, command=f'rpm -q {package_name}', username=self.vm_user, password=self.vm_cred)[0]
            if "not installed" in package_installation_status:
                return False
        return True

    def get_os_images_from_nodes(self) -> None:
        os_images = []
        nodes = self.core_v1_api.list_node().items
        controlplane_nodes = [node for node in nodes if "node-role.kubernetes.io/control-plane" in node.metadata.labels]

        for controlplane_node in controlplane_nodes:
            node_name = controlplane_node.metadata.name
            os_image = controlplane_node.status.node_info.os_image
            os_images.append(os_image)
            logging.info(f'Control Plane: {node_name}, OS Image: {os_image}')

        return os_images

    def get_expected_removed_status(self) -> dict:
        expected_removed_status = {
            'removedChecksStatus': {'message': '', 'removedChecks': []},
            'removedProfilesStatus': {'message': '', 'removedProfiles': []}
        }

        checks = self.custom_objects_api.list_cluster_custom_object(self.cce_group, self.version, plural=self.check_plural)["items"]
        profiles = self.custom_objects_api.list_cluster_custom_object(self.cce_group, self.version, plural=self.profile_plural)["items"]

        for check in checks:
            expected_removed_status['removedChecksStatus']['removedChecks'].append({'name': check['metadata']['name'], 'check_versions': check['spec']['check_versions']})

        for profile in profiles:
            expected_removed_status['removedProfilesStatus']['removedProfiles'].append({'name': profile['metadata']['name']})

        expected_removed_status['removedChecksStatus']['message'] = f'Number of Checks removed from the cluster equals: {len(checks)}'
        expected_removed_status['removedProfilesStatus']['message'] = f'Number of Profiles removed from the cluster equals: {len(profiles)}'

        return expected_removed_status

    def is_all_nodes_status_ready(self) -> bool:
        try:
            nodes_ready_status = False
            nodes = self.core_v1_api.list_node().items
            controlplane_nodes = [node for node in nodes if "node-role.kubernetes.io/control-plane" in node.metadata.labels]

            for controlplane_node in controlplane_nodes:
                node_infos = controlplane_node.status.conditions
                for node_info in node_infos:
                    if node_info.type == 'Ready':
                        logging.info(f"Status Ready={node_info.status} for controlplane_ip '{controlplane_node.status.addresses[0].address}'.")
                        if node_info.status == 'False' or node_info.status == 'Unknown':
                            return False
                        else:
                            nodes_ready_status = True
            return nodes_ready_status
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Getting node ready status failed.')

    def get_schedulable_masters_from_nodes(self) -> str:
        try:
            nodes = self.core_v1_api.list_node().items
            schedulable_masters = [node.metadata.name for node in nodes
                                   if "node-role.kubernetes.io/master" in node.metadata.labels
                                   and (not node.spec.taints
                                        or not any(taint.key == "node-role.kubernetes.io/master"
                                                   and taint.effect == "NoSchedule"
                                                   for taint in node.spec.taints))]
            logging.info(f'Schedulable masters: {schedulable_masters}')

            return schedulable_masters
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Getting schedulable masters from nodes failed.')

    def get_cce_statuses(self) -> list:
        command = "kubectl get pods -n atlantic -o wide | grep cce-operator | awk {'print $3'} | column -t"
        command_output = Ssh.execute_ssh_command(self, ip_address=self.get_controlplane_ips()[0], command=command, username=self.vm_user, password=self.vm_cred)[0]
        return command_output.splitlines()

    def check_all_pods_running(self, statuses: list) -> bool:
        for status in statuses:
            if status != 'Running':
                logging.info(f"Pod status is not 'Running': {status}")
                return False
        return True

    def wait_for_all_cce_pods_running(self, timeout=40) -> None:
        logging.info('Wait for all cce pods to be running.')
        start_time = time.time()
        while not self.check_all_pods_running(self.get_cce_statuses()):
            time.sleep(0.2)
            if time.time() - start_time > timeout:
                pytest.fail(f'Not all cce pods running after {timeout}s.')

    def wait_for_helm_release_upgrade(self, release_name: str, timeout=120) -> bool:
        logging.info(f"[METHOD] Waiting for Helm release '{release_name}' to upgrade")

        try:
            start_time = time.time()
            count = 0
            while True:
                res = self.custom_objects_api.get_namespaced_custom_object(
                    "helm.toolkit.fluxcd.io", "v2beta1", self.namespace, "helmreleases", release_name
                )

                status = res.get("status", {})
                conditions = status.get("conditions", [])

                for condition in conditions:
                    if condition.get("type") == "Ready" and condition.get("status") == "True":
                        logging.info(f"Helm release '{release_name}' in namespace '{self.namespace}' is ready.")
                        return True

                if time.time() - start_time > timeout:
                    logging.warning(f"Helm release '{release_name}' did not become ready after {timeout} seconds timeout.")
                    return False

                logging.warning(f"Waiting for Helm release '{release_name}' to become ready...{count}s")
                count += 5
                time.sleep(5)

        except Exception as exc:
            logging.warning(f"Failed to wait for Helm release '{release_name}' upgrade. Reason: {str(exc)}")
            return False

    def update_helmrelease_node_taints_policy(self, value: str) -> None:
        logging.info('[METHOD] Update helmrelease')

        try:
            res = self.custom_objects_api.get_namespaced_custom_object("helm.toolkit.fluxcd.io", "v2beta2", "atlantic", "helmreleases", "cce-operator")

            res["spec"]["values"]["topologySpreadConstraints"][0]["nodeTaintsPolicy"] = value

            self.custom_objects_api.patch_namespaced_custom_object("helm.toolkit.fluxcd.io", "v2beta2", "atlantic", "helmreleases", "cce-operator", res)

            time.sleep(5)

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Update Helm Release failed.')

    def get_helmrelease_current_node_affinity(self) -> dict:
        logging.info('[METHOD] Get current node affinity')

        try:
            res = self.custom_objects_api.get_namespaced_custom_object("helm.toolkit.fluxcd.io", "v2beta2", "atlantic", "helmreleases", "cce-operator")

            return res["spec"]["values"]["affinity"].get("nodeAffinity", {})

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Get current node affinity failed.')

    def merge_node_affinities(self, affinity1: dict, affinity2: dict) -> dict:
        logging.info('[METHOD] Merge node affinities')

        merged_affinity = copy.deepcopy(affinity1)

        for key in ["requiredDuringSchedulingIgnoredDuringExecution", "preferredDuringSchedulingIgnoredDuringExecution"]:
            if key in merged_affinity and key in affinity2:
                if isinstance(merged_affinity[key], dict) and isinstance(affinity2[key], dict):
                    merged_affinity[key]["nodeSelectorTerms"].extend(
                        affinity2[key].get("nodeSelectorTerms", [])
                    )
            elif key in affinity2:
                merged_affinity[key] = affinity2[key]

        return merged_affinity

    def get_hostname_node_affinity(self, hostname: str) -> dict:
        return {
            "requiredDuringSchedulingIgnoredDuringExecution":
                {
                    "nodeSelectorTerms": [
                        {
                            "matchExpressions": [
                                {
                                    "key": "kubernetes.io/hostname",
                                    "operator": "In",
                                    "values": [hostname]
                                }
                            ]
                        }
                    ]
                }
        }

    def set_helmrelease_node_affinity(self, node_affinity: dict) -> None:
        logging.info('[METHOD] Set node affinity')

        try:
            res = self.custom_objects_api.get_namespaced_custom_object(
                "helm.toolkit.fluxcd.io", "v2beta2", "atlantic", "helmreleases", "cce-operator"
            )

            res["spec"]["values"]["affinity"]["nodeAffinity"] = node_affinity

            self.custom_objects_api.patch_namespaced_custom_object(
                "helm.toolkit.fluxcd.io", "v2beta2", "atlantic", "helmreleases", "cce-operator", res
            )

            time.sleep(5)

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Set node affinity failed.')

    def get_not_same_node_pod_anti_affinity(self) -> dict:
        return {
            "requiredDuringSchedulingIgnoredDuringExecution": [
                {
                    "labelSelector":
                        {
                            "matchExpressions": [
                                {
                                    "key": "app",
                                    "operator": "In",
                                    "values": ["cce-operator"]
                                }
                            ]
                        },
                    "topologyKey": "kubernetes.io/hostname"
                }
            ]
        }

    def get_helmrelease_current_pod_anti_affinity(self) -> dict:
        logging.info('[METHOD] Get current pod anti affinity')

        try:
            res = self.custom_objects_api.get_namespaced_custom_object("helm.toolkit.fluxcd.io", "v2beta2", "atlantic", "helmreleases", "cce-operator")

            return res["spec"]["values"]["affinity"].get("podAntiAffinity", {})

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Get current pod anti affinity failed.')

    def set_helmrelease_pod_anti_affinity(self, pod_anti_affinity: dict) -> None:
        logging.info('[METHOD] Set pod anti affinity')

        try:
            res = self.custom_objects_api.get_namespaced_custom_object(
                "helm.toolkit.fluxcd.io", "v2beta2", "atlantic", "helmreleases", "cce-operator"
            )

            if pod_anti_affinity == {} and res["spec"]["values"]["affinity"].get("podAntiAffinity", {}) != {}:
                res["spec"]["values"]["affinity"]["podAntiAffinity"] = None
            else:
                res["spec"]["values"]["affinity"]["podAntiAffinity"] = pod_anti_affinity

            self.custom_objects_api.patch_namespaced_custom_object(
                "helm.toolkit.fluxcd.io", "v2beta2", "atlantic", "helmreleases", "cce-operator", res
            )

            time.sleep(5)

            self.wait_for_helm_release_upgrade(self.cce_operator_app)

        except Exception as exc:
            logging.error(exc)
            pytest.fail('Set pod anti affinity failed.')

    def update_statefulset_replicas(self, replicas: int, timeout=300) -> bool:
        logging.info(f'[METHOD] Update statefulset with {replicas} replicas')

        try:
            stateful_set = self.cce_read_namespaced_stateful_set(self.cce_operator_app)
            stateful_set.spec.replicas = replicas
            stateful_set.spec.template.metadata.annotations = {'cmo.reschedule': 'true', 'cmo.reschedule.toleration': '5'}
            self.cce_patch_namespaced_stateful_set(self.cce_operator_app, stateful_set)

            time.sleep(3)

            if replicas == 0:
                self.delete_pvcs_cce()
                self.delete_cce_pods(check_status=False)

            time.sleep(3)
            start_time = time.time()
            count = 0

            nodes_with_cce = self.get_nodes_with_cce()

            while len(nodes_with_cce) != replicas or None in nodes_with_cce:
                logging.info(f"Expected replicas: '{replicas}' vs {len(nodes_with_cce)}")
                logging.info(f"All cce replicas started on nodes: '{nodes_with_cce}'")
                count += 1
                time.sleep(1)
                logging.info(f"Waiting to for replicas deployment...{count} s")
                if time.time() - start_time > timeout:
                    logging.error(f"Cce replicas not started with number - {len(nodes_with_cce)}. Stateful_set.spec.replicas: {replicas}'")
                    pytest.fail('Getting schedulable masters from nodes failed.')
                nodes_with_cce = self.get_nodes_with_cce()

            logging.info(f"Expected replicas: '{replicas}' vs {len(nodes_with_cce)}")
            logging.info(f"All cce replicas started on nodes: '{nodes_with_cce}'")
            time.sleep(5)
            if replicas != 0:
                self.print_cce_leader_name_from_logs()
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Update statefulset replicas failed.')

    def are_nodes_uncordoned(self) -> bool:
        nodes = self.core_v1_api.list_node().items
        return all(not node.spec.unschedulable for node in nodes)

    def are_any_node_cordoned(self) -> bool:
        nodes = self.core_v1_api.list_node().items
        return any(node.spec.unschedulable for node in nodes)

    def get_statefulset_replicas(self, is_default=False) -> int:
        try:
            stateful_set = self.cce_read_namespaced_stateful_set(self.cce_operator_app)
            replicas = stateful_set.spec.replicas
            if is_default:
                assert replicas != 0, 'Default statefulset replicas set to 0'
                logging.info(f"[ASSERT] Default statefulset replicas set to {replicas}")
            return replicas
        except Exception as exc:
            logging.error(exc)
            pytest.fail('Update statefulset replicas failed.')

    def reset_statefulset(self, default_replicas) -> None:
        logging.info(f'Reseting cce statefulset with replicas: {default_replicas}')

        self.update_statefulset_replicas(replicas=0)
        self.update_statefulset_replicas(default_replicas)
        self.wait_for_cce_leader_to_be_running(timeout=30)
        self.wait_for_cce_leader_to_be_ready()

    def delete_pvcs_cce(self) -> None:
        logging.info('Delete all pvcs cce')
        cce_pvc_names = self.get_pvc_names(self.cce_operator_app)
        logging.info(f'PVC names to be deleted: {cce_pvc_names}')
        for cce_pvc_name in cce_pvc_names:
            self.delete_pvc(cce_pvc_name)

    def delete_cce_pods(self, timeout=3, check_status=True) -> str:
        logging.info('Deleting cce pods')
        pods = self.core_v1_api.list_namespaced_pod(namespace=self.namespace)
        for pod in pods.items:
            if pod.metadata.name.startswith('cce-operator'):
                self.delete_pod(pod.metadata.name, check_status)
        time.sleep(timeout)
        logging.info('cce pods deleted.')

    def is_cce_leader_running(self) -> None:
        leader_name = self.get_cce_leader_name_from_logs()
        return self.is_pod_running(leader_name)

    def is_cce_leader_ready(self) -> None:
        leader_name = self.get_cce_leader_name_from_logs()
        return self.is_pod_ready(leader_name)

    def pretty_print(self, response: str) -> None:
        logging.info('Response:')
        response_dumps = json.dumps(response, indent=2)
        logging.info(response_dumps)

        response_dict = json.loads(response_dumps)
        if 'status' in response_dict and 'report' in response_dict['status']:
            self.pretty_print_report(response)

    def pretty_print_report(self, response: str) -> None:
        logging.info('Response report:')
        report = json.loads(response['status']['report'])
        report_dumps = json.dumps(report, indent=2)
        logging.info(report_dumps)

        if 'Check out cce.log' in report_dumps:
            logging.warning("'Check out cce.log details' found in report.")
            logging.info(self.get_cce_log_content())
            self.print_cce_operator_pod_logs()

    def verify_file_on_pod(self, pod_name: str, path: str, expected_file: str) -> None:
        logging.info(f"Verify file {expected_file} in {path} on pod {pod_name}")
        exec_command_ls = [
            "/bin/sh",
            "-c",
            f"ls {path}"]

        response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, pod_name, self.namespace, container="cce-operator", command=exec_command_ls,
                          stderr=True, stdin=False, stdout=True, tty=False)

        assert expected_file in response.splitlines(), f'{expected_file} does not exist in {path} on {pod_name}: {response.splitlines()}'
        logging.info(f'[ASSERT] {expected_file} exists in {path} on {pod_name}: {response.splitlines()}')

    def wait_for_cce_leader_to_be_running(self, timeout=20) -> bool:
        count = 0
        start_time = time.time()

        while not self.is_cce_leader_running():
            count += 1
            logging.warning(f'Waiting for cce leader to be running...{count}s')
            time.sleep(1)
            if time.time() - start_time > timeout:
                pytest.fail(f'CCE Leader "{self.get_cce_leader_name_from_logs()}" is not running - TIMEOUT.')

        logging.info(f'CCE Leader "{self.get_cce_leader_name_from_logs()}" is running.')

    def wait_for_cce_leader_to_be_ready(self, timeout=20) -> bool:
        count = 0
        start_time = time.time()

        while not self.is_cce_leader_ready():
            count += 1
            logging.warning(f'Waiting for cce leader to be ready...{count}s')
            time.sleep(1)
            if time.time() - start_time > timeout:
                pytest.fail(f'CCE Leader "{self.get_cce_leader_name_from_logs()}" is not ready - TIMEOUT.')

        logging.info(f'CCE Leader "{self.get_cce_leader_name_from_logs()}" is ready.')

    def get_checks_number(self) -> int:
        checks = self.cce_get_cluster_objects_names(self.cce_group, self.version, self.check_plural)
        return len(checks)

    def wait_for_checks_init(self, timeout=20) -> bool:
        count = 0
        start_time = time.time()

        while 'Successful Init Check CRs' not in self.get_cce_operator_pod_logs():
            count += 1
            logging.warning(f'Waiting for checks to be initilized...{count}s')
            time.sleep(1)

            if time.time() - start_time > timeout:
                pytest.fail('Checks are not initilized - TIMEOUT.')

        logging.info(f'{self.get_checks_number()} checks have been initialized.')

    def delete_all_checks_init(self, timeout=30) -> None:
        ips = self.get_controlplane_ips()
        command = "kubectl delete checks --all"
        command_output = Ssh.execute_ssh_command(self, ip_address=ips[0], command=command, username=self.vm_user, password=self.vm_cred)[0]

        logging.info(f'Delete checks output:\n{command_output}')

        count = 0
        start_time = time.time()

        while self.get_checks_number() != 0:
            count += 1
            logging.warning(f'Waiting to complete checks deletion...{count}s')
            time.sleep(1)
            if time.time() - start_time > timeout:
                pytest.fail('Failed to delete all checks - TIMEOUT.')

        logging.info('All checks have been deleted.')

    def get_cce_leader_name_from_logs(self, timeout=30) -> str:
        logging.info('Get CCE Leader Name from Logs.')

        count = 0
        start_time = time.time()

        is_lease_acuired = False

        while not is_lease_acuired:
            count += 1
            time.sleep(1)
            pods = self.core_v1_api.list_namespaced_pod(namespace=self.namespace)
            for pod in pods.items:
                if pod.metadata.name.startswith('cce-operator'):
                    if 'successfully acquired lease' in self.get_pod_logs(pod.metadata.name):
                        logging.info(f"The CCE Leader name is: {pod.metadata.name}")
                        is_lease_acuired = True
                        return pod.metadata.name

            logging.warning(f'Waiting for successful cce leader election...{count}s')

            if time.time() - start_time > timeout:
                pytest.fail('Failed to complete leader election - TIMEOUT')

        pytest.fail(f"No matching 'successfully acquired lease' for CCE in the namespace '{self.namespace}'")

    def print_cce_leader_name_from_logs(self) -> str:
        pods = self.core_v1_api.list_namespaced_pod(namespace=self.namespace)
        for pod in pods.items:
            if pod.metadata.name.startswith('cce-operator'):
                if 'successfully acquired lease' in self.get_pod_logs(pod.metadata.name):
                    logging.info(f"The CCE Leader name from logs is: {pod.metadata.name}")

    def print_logs(self, host_test=False) -> None:
        logging.info("[STAGE] Print log from all nodes.")
        if host_test:
            ip_adresses = self.hosts
        else:
            ip_adresses = self.get_controlplane_ips()

        for ip_adress in ip_adresses:
            log_name = f'{ip_adress}.log'

            exec_command_cat = [
                "/bin/sh",
                "-c",
                f"cat /home/app/cce/log/{log_name}"]

            cce_pod_name = self.get_cce_leader_name_from_logs()

            response = stream(self.core_v1_api.connect_get_namespaced_pod_exec, cce_pod_name, self.namespace, container="cce-operator", command=exec_command_cat,
                              stderr=True, stdin=False, stdout=True, tty=False)

            logging.info(f'Log from {ip_adress}:\n{response}')

    def is_namespace_exist(self, namespace: str) -> bool:
        return any(ns.metadata.name == namespace for ns in self.core_v1_api.list_namespace().items)

    def is_istio_enabled(self) -> bool:
        namespace = self.core_v1_api.read_namespace(self.namespace)
        is_enable = 'istio-injection' in namespace.metadata.labels and namespace.metadata.labels['istio-injection'] == 'enabled'
        logging.info(f'Istio is enabled: {is_enable}')
        return is_enable

    def get_istio_ingress_ip(self) -> str:
        istio_ingressgateway_svc = self.core_v1_api.read_namespaced_service('istio-ingressgateway', namespace='istio-system')
        istio_ingressgateway_ip = istio_ingressgateway_svc.status.load_balancer.ingress[0].ip
        logging.info(f'Istio ingress IP: {istio_ingressgateway_ip}')
        return istio_ingressgateway_ip

    def get_nodes_with_cce(self, is_unique=True) -> list:
        pods = self.core_v1_api.list_namespaced_pod(namespace=self.namespace)
        cce_pods = [pod for pod in pods.items if pod.metadata.name.startswith('cce-operator')]
        nodes = [pod.spec.node_name for pod in cce_pods]
        logging.info(f'Node with cce-operator: {nodes}')
        if is_unique:
            return list(set(nodes))
        else:
            return nodes

    def get_cce_sts_image_value(self) -> str:
        command = 'kubectl get sts cce-operator -n atlantic -o=jsonpath="{.spec.template.spec.containers[0].image}"'
        image_version = Ssh.execute_ssh_command(self, ip_address=self.get_controlplane_ips()[0], command=command, username=self.vm_user, password=self.vm_cred)[0]
        assert image_version is not None and image_version != "", "image_version is empty or null"
        logging.info(f'Cce-operator image version: {image_version} is not empty or null')
        return image_version

    def get_execution_state_from_status_endpoint(self) -> str:
        response = requests.get(f'http://{self.ansible_server}:8383/v1/status/', timeout=self.default_timeout)
        data = response.json()
        return data['executions'][-1]['state']

    def wait_for_status_complete(self, timeout: int, msg: str):
        start_time = time.time()
        time.sleep(1)

        while self.get_execution_state_from_status_endpoint() != "complete":
            elapsed_time = time.time() - start_time
            remaining_time = int(timeout - elapsed_time)
            minutes, seconds = divmod(int(remaining_time), 60)
            logging.info(f"{msg} Estimated remaining time: {minutes}min {seconds}sec.")

            if elapsed_time > timeout:
                assert False, f'{msg} Failed to complete in {timeout}sec.'

            time.sleep(5)

    def deploy_cce_operator(self, timeout: int) -> None:
        url = f'http://{self.ansible_server}:8383/v1/clusters/apps'
        headers = {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        }

        with open('resources/requests/deploy-cce.json', 'r', encoding="utf-8") as file:
            payload = json.load(file)

        response = requests.post(url, json=payload, headers=headers, timeout=self.default_timeout)
        assert response.status_code == 202

        self.wait_for_status_complete(timeout, "Deploying cce-operator...")
        logging.info("Deploying cce-operator completed.")

    def remove_cce_operator(self, timeout: int) -> None:
        url = f'http://{self.ansible_server}:8383/v1/clusters/apps'
        headers = {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        }

        with open('resources/requests/remove-cce.json', 'r', encoding="utf-8") as file:
            payload = json.load(file)

        response = requests.delete(url, json=payload, headers=headers, timeout=self.default_timeout)
        assert response.status_code == 202

        self.wait_for_status_complete(timeout, "Removing cce-operator...")
        logging.info("Removing cce-operator completed.")

    def get_cce_pod_names(self) -> list:
        command = "kubectl get pods -n atlantic -o wide | grep cce-operator | awk {'print $1'} | column -t"
        command_output = Ssh.execute_ssh_command(self, ip_address=self.get_controlplane_ips()[0], command=command, username=self.vm_user, password=self.vm_cred)[0]
        return command_output.splitlines()

    def scale_down_node(self, timeout: int) -> None:
        url = f'http://{self.ansible_server}:8383/v1/clusters/nodes'
        headers = {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        }

        with open('resources/requests/scale_node.json', 'r', encoding="utf-8") as file:
            payload = json.load(file)

        response = requests.delete(url, json=payload, headers=headers, timeout=self.default_timeout)
        assert response.status_code == 202, f"Wrong status code on response during node scale down: {response.status_code}"

        self.wait_for_status_complete(timeout, "Scaling down node...")
        logging.info("Scaling down node completed.")

    def scale_up_node(self, timeout: int) -> None:
        url = f'http://{self.ansible_server}:8383/v1/clusters/nodes'
        headers = {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        }

        with open('resources/requests/scale_node.json', 'r', encoding="utf-8") as file:
            payload = json.load(file)

        response = requests.post(url, json=payload, headers=headers, timeout=self.default_timeout)
        assert response.status_code == 202, f"Wrong status code on response during node scale up: {response.status_code}"

        self.wait_for_status_complete(timeout, "Scaling up node...")
        logging.info("Scaling up node completed.")

    def update_scale_node_json(self, host_name, host_ip):
        scale_node_json = 'resources/requests/scale_node.json'
        with open(scale_node_json, 'r', encoding='utf-8') as file:
            json_data = json.load(file)

        if 'controlplane' in json_data and len(json_data['controlplane']) > 0:
            json_data['controlplane'][0]['hostname'] = host_name
            json_data['controlplane'][0]['managementhost'] = host_ip
            json_data['controlplane'][0]['kuberneteshost'] = host_ip
            json_data['controlplane'][0]['ansible_password'] = self.vm_cred

        with open(scale_node_json, 'w', encoding='utf-8') as output_file:
            json.dump(json_data, output_file, indent=4)

    def restart_node_by_ip(self, node_ip: str) -> None:
        try:
            node_reboot_result = Ssh.execute_ssh_command(self, ip_address=node_ip, command='shutdown -r now', username=self.vm_user, password=self.vm_cred)[0]
            logging.info(f"Node reboot result: {node_reboot_result}")
        except Exception as e:
            logging.info(f"Restart Failed: {e}")

    def get_node_state(self, node: str) -> str:
        nodes = self.core_v1_api.list_node().items
        for n in nodes:
            if n.metadata.name == node:
                for condition in n.status.conditions:
                    if condition.type == "Ready":
                        logging.info(condition)
                        return condition.status

    def wait_for_cce_operator_pending_count(self, expected_pending_count: int, timeout=30) -> None:
        start_time = time.time()
        while time.time() - start_time < timeout:
            pending_count = sum(1 for pod in self.core_v1_api.list_namespaced_pod(self.namespace, label_selector='app=cce-operator').items if pod.status.phase == 'Pending')
            if pending_count == expected_pending_count:
                return
            time.sleep(0.5)
        logging.warning(f"Expected number '{expected_pending_count}' of pending cce pods not reached.")