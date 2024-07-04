import time
import logging
from framework.kubernetes_helper import KubernetesHelper


class Utils:
    def __init__(self, vm_user: str, vm_cred: str, namespace: str):
        self.vm_user = vm_user
        self.vm_cred = vm_cred
        self.namespace = namespace
        self.core_v1_api, self.custom_objects_api, self.apps_v1_api, self.network_v1_api, self.coordination_v1_api = KubernetesHelper.load_kube_api()

    def is_pod_running(self, pod_name: str, timeout=30) -> bool:
        end_time = time.time() + timeout
        while time.time() < end_time:
            status = self.core_v1_api.read_namespaced_pod_status(pod_name, self.namespace).status
            if status.phase == 'Running':
                logging.info(f"Pod '{pod_name}' in namespace '{self.namespace}' is running.")
                return True
            time.sleep(1)
        logging.warning(f"POD '{pod_name}' not running after {timeout} seconds timeout.")

    def is_pod_ready(self, pod_name: str, timeout=5):
        try:
            start_time = time.time()
            while time.time() - start_time < timeout:
                pod = self.core_v1_api.read_namespaced_pod_status(pod_name, self.namespace)
                if any(condition.status == "True" and condition.type == "Ready" for condition in pod.status.conditions):
                    logging.info(f"Pod '{pod_name}' in namespace '{self.namespace}' is ready.")
                    time.sleep(2)
                    return True
                time.sleep(0.2)
            logging.warning(f"Pod '{pod_name}' not ready after {timeout} seconds timeout.")
            return False
        except Exception as exc:
            logging.warning(f"Failed check if '{pod_name}' pod is ready. Reason: {str(exc)}")
            return False