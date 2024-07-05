import time
import logging

from typing import Any, Callable, Dict, List, Optional
from kubernetes.client.models import (
    V1Pod,
    V1PersistentVolumeClaim,
    CoreV1Event,
)
from kubernetes.client.rest import ApiException

import framework.const as const

from framework.kubernetes_helper import KubernetesHelper


class Utils:
    def __init__(self, vm_user: str, vm_cred: str, namespace: str):
        self.vm_user = vm_user
        self.vm_cred = vm_cred
        self.namespace = namespace
        (
            self.core_v1_api,
            self.custom_objects_api,
            self.apps_v1_api,
            self.network_v1_api,
            self.coordination_v1_api,
        ) = KubernetesHelper.load_kube_api()
        self.sc_mapper = {
            const.HDD_SC: const.STORAGE_TYPE_HDD,
            const.SSD_SC: const.STORAGE_TYPE_SSD,
            const.NVME_SC: const.STORAGE_TYPE_NVME,
            const.HDDLVG_SC: const.STORAGE_TYPE_HDDLVG,
            const.SSDLVG_SC: const.STORAGE_TYPE_SSDLVG,
            const.NVMELVG_SC: const.STORAGE_TYPE_NVMELVG,
            const.SYSLVG_SC: const.STORAGE_TYPE_SYSLVG,
        }

    def is_pod_running(self, pod_name: str, timeout=30) -> bool:
        """
        Checks if a given pod is running in the Kubernetes cluster.

        Args:
            pod_name (str): The name of the pod to check readiness.
            timeout (int): The timeout in seconds. Defaults to 30.

        Returns:
            bool: True if the pod is running, False otherwise.
        """
        end_time = time.time() + timeout
        while time.time() < end_time:
            status = self.core_v1_api.read_namespaced_pod_status(
                pod_name, self.namespace
            ).status
            if status.phase == "Running":
                logging.info(
                    f"Pod '{pod_name}' in namespace '{self.namespace}' is running."
                )
                return True
            time.sleep(1)
        logging.warning(
            f"POD '{pod_name}' not running after {timeout} seconds timeout."
        )

    def is_pod_ready(self, pod_name: str, timeout=5):
        """
        Checks if a given pod is ready in the Kubernetes cluster.

        Args:
            pod_name (str): The name of the pod to check readiness.
            timeout (int): The timeout for checking readiness in seconds. Defaults to 5 seconds.

        Returns:
            bool: True if the pod is ready, False otherwise.
        """
        try:
            start_time = time.time()
            while time.time() - start_time < timeout:
                pod = self.core_v1_api.read_namespaced_pod_status(
                    pod_name, self.namespace
                )
                if any(
                    condition.status == "True" and condition.type == "Ready"
                    for condition in pod.status.conditions
                ):
                    logging.info(
                        f"Pod '{pod_name}' in namespace '{self.namespace}' is ready."
                    )
                    time.sleep(2)
                    return True
                time.sleep(0.2)
            logging.warning(
                f"Pod '{pod_name}' not ready after {timeout} seconds timeout."
            )
            return False
        except Exception as exc:
            logging.warning(
                f"Failed check if '{pod_name}' pod is ready. Reason: {str(exc)}"
            )
            return False

    def list_pods(
        self,
        name_prefix: Optional[str] = None,
        namespace: Optional[str] = None,
        label: Optional[str] = None,
    ) -> List[V1Pod]:
        """
        Retrieves a list of Pod objects from the Kubernetes API.

        Args:
            name_prefix (Optional[str], optional): The name prefix to filter Pods. Defaults to None.
            namespace (Optional[str], optional): The namespace to filter Pods. Defaults to None.
            label (Optional[str], optional): The label to filter Pods. Defaults to None.

        Returns:
            List[V1Pod]: A list of Pod objects that match the provided name prefix, namespace, and label.

        """
        pods = self.core_v1_api.list_pod_for_all_namespaces(watch=False).items
        if name_prefix:
            pods = [p for p in pods if p.metadata.name.startswith(name_prefix)]
        if namespace:
            pods = [p for p in pods if p.metadata.namespace == namespace]
        if label:
            label_split = label.split("=")
            key, value = label_split[0], label_split[1]
            labeled_pods = {p.metadata.name: p for p in pods}
            for p in pods:
                requested_label = p.metadata.labels.get(key, None)
                if requested_label is None or requested_label != value:
                    del labeled_pods[p.metadata.name]
            pods = list(labeled_pods.values())
        return pods

    def list_persistent_volume_claims(
        self,
        namespace: str,
        name: Optional[str] = None,
        label: Optional[str] = None,
        pod_name: Optional[str] = None,
    ) -> List[V1PersistentVolumeClaim]:
        """
        Retrieves a list of PersistentVolumeClaim objects based on the provided filters.

        Args:
            namespace (str): The namespace of the PersistentVolumeClaim objects.
            name (Optional[str], default=None): The name of the PersistentVolumeClaim objects.
            label (Optional[str], default=None): The label of the PersistentVolumeClaim objects.
            pod_name (Optional[str], default=None): The name of the Pod objects.

        Returns:
            List[V1PersistentVolumeClaim]: A list of PersistentVolumeClaim objects that match the provided filters.
        """
        pvcs = self.core_v1_api.list_namespaced_persistent_volume_claim(
            namespace
        ).items
        if name:
            pvcs = [p for p in pvcs if p.metadata.name == name]
        if namespace:
            pvcs = [p for p in pvcs if p.metadata.namespace == namespace]
        if label:
            label_split = label.split("=")
            key, value = label_split[0], label_split[1]
            labeled_pods = {p.metadata.name: p for p in pvcs}
            for p in pvcs:
                requested_label = p.metadata.labels.get(key, None)
                if requested_label is None or requested_label != value:
                    del labeled_pods[p.metadata.name]
            pvcs = list(labeled_pods.values())
        if pod_name:
            pods = self.list_pods(name_prefix=pod_name, namespace=namespace)
            pvc_names = set(
                v.persistent_volume_claim.claim_name
                for p in pods
                for v in p.spec.volumes
                if v.persistent_volume_claim is not None
            )
            pvcs = [p for p in pvcs if p.metadata.name in pvc_names]
        return pvcs

    def list_volumes(
        self,
        name: Optional[str] = None,
        pod_name: Optional[str] = None,
        location: Optional[str] = None,
        storage_class: Optional[str] = None,
        node: Optional[str] = None,
    ) -> List[dict]:
        """
        Retrieves a list of volumes based on the provided filters.

        Args:
            name (Optional[str], default=None): The name of the volume.
            pod_name (Optional[str], default=None): The name of the Pod object.
            location (Optional[str], default=None): The location of the volume.
            storage_class (Optional[str], default=None): The storage class of the volume.
            node (Optional[str], default=None): The node ID of the volume.

        Returns:
            List[dict]: A list of volumes that match the provided filters.
        """
        volumes = self.custom_objects_api.list_cluster_custom_object(
            const.CR_GROUP, const.CR_VERSION, "volumes"
        )["items"]
        if name:
            volumes = [v for v in volumes if v["metadata"]["name"] == name]
        if pod_name:
            pvcs = self.list_persistent_volume_claims(
                namespace=self.namespace, pod_name=pod_name
            )
            volume_names = set(pvc.spec.volume_name for pvc in pvcs)
            volumes = [
                v for v in volumes if v["metadata"]["name"] in volume_names
            ]
        if location:
            volumes = [v for v in volumes if v["spec"]["Location"] == location]
        if storage_class:
            volumes = [
                v
                for v in volumes
                if v["spec"]["StorageClass"] == storage_class
                or v["spec"]["StorageClass"]
                == self.sc_mapper.get(storage_class, "UNKNOWN")
            ]
        if node:
            volumes = [v for v in volumes if v["spec"]["NodeId"] == node]
        return volumes

    def get_drive_cr(self, volume_name: str, namespace: str) -> dict:
        """
        Retrieves the custom resource configuration for a specific drive by volume name.

        Args:
            volume_name (str): The name of the volume.
            namespace (str): The namespace of the volume.

        Returns:
            dict: The custom resource configuration for the drive.

        Raises:
            ApiException: If an error occurs while retrieving the custom resource configuration.
        """
        volume = self.custom_objects_api.get_namespaced_custom_object(
            const.CR_GROUP, const.CR_VERSION, namespace, "volumes", volume_name
        )
        location = volume["spec"]["Location"]
        try:
            drive_cr = self.custom_objects_api.get_cluster_custom_object(
                const.CR_GROUP, const.CR_VERSION, "drives", location
            )
            return drive_cr
        except ApiException:
            logging.warning(f"drive cr {location} not found, looking for LVG")
            lvg_cr = self.custom_objects_api.get_cluster_custom_object(
                const.CR_GROUP,
                const.CR_VERSION,
                "logicalvolumegroups",
                location,
            )
            drive_cr = self.custom_objects_api.get_cluster_custom_object(
                const.CR_GROUP,
                const.CR_VERSION,
                "drives",
                lvg_cr["spec"]["Locations"][0],
            )
            return drive_cr

    def get_events_by_reason(
        self,
        plural: str,
        resource_name: str,
        reason: Optional[str] = None,
    ) -> List[CoreV1Event]:
        """
        Retrieves events related to a specific resource by reason.

        Args:
            plural (str): The plural name of the resource.
            resource_name (str): The name of the resource.
            reason (Optional[str], optional): The reason for filtering events. Defaults to None.

        Returns:
            List[CoreV1Event]: A list of events related to the resource by reason.
        """
        cr = self.custom_objects_api.get_cluster_custom_object(
            const.CR_GROUP, const.CR_VERSION, plural, resource_name
        )
        uid = cr["metadata"]["uid"]
        field_selector = f"involvedObject.uid={uid}"
        events_list = self.core_v1_api.list_event_for_all_namespaces(
            field_selector=field_selector
        ).items

        if reason:
            events_list = [e for e in events_list if e.reason == reason]

        return events_list

    def wait_volume(
        self,
        name: str,
        expected_status: Optional[str] = None,
        expected_health: Optional[str] = None,
        expected_usage: Optional[str] = None,
        timeout: int = 60,
    ) -> bool:
        """
        Waits for a volume with the given name to meet the expected status, health, and usage within the given timeout.

        Args:
            name (str): The name of the volume to wait for.
            expected_status (Optional[str], optional): The expected status of the volume. Defaults to None.
            expected_health (Optional[str], optional): The expected health of the volume. Defaults to None.
            expected_usage (Optional[str], optional): The expected usage of the volume. Defaults to None.
            timeout (int): The maximum time to wait for the volume in seconds. Defaults to 60.

        Returns:
            bool: True if the volume meets the expected status, health, and usage within the given timeout, False otherwise.
        """
        expected = {}
        if expected_status:
            expected["CSIStatus"] = expected_status
        if expected_usage:
            expected["Usage"] = expected_usage
        if expected_health:
            expected["Health"] = expected_health

        def callback():
            return self.list_volumes(name)[0]

        return self._wait_cr(
            expected=expected, get_cr_fn=callback, timeout=timeout
        )

    def wait_drive(
        self,
        name: str,
        expected_status: Optional[str] = None,
        expected_health: Optional[str] = None,
        expected_usage: Optional[str] = None,
        timeout: int = 60,
    ) -> bool:
        """
        Waits for a drive with the given name to meet the expected status, health, and usage within the given timeout.

        Args:
            name (str): The name of the drive to wait for.
            expected_status (Optional[str], optional): The expected status of the drive. Defaults to None.
            expected_health (Optional[str], optional): The expected health of the drive. Defaults to None.
            expected_usage (Optional[str], optional): The expected usage of the drive. Defaults to None.
            timeout (int): The maximum time to wait for the drive in seconds. Defaults to 60.

        Returns:
            bool: True if the drive meets the expected status, health, and usage within the given timeout, False otherwise.
        """
        expected = {}
        if expected_status:
            expected["Status"] = expected_status
        if expected_usage:
            expected["Usage"] = expected_usage
        if expected_health:
            expected["Health"] = expected_health

        def callback():
            return self.custom_objects_api.get_cluster_custom_object(
                const.CR_GROUP, const.CR_VERSION, "drives", name
            )

        return self._wait_cr(
            expected=expected, get_cr_fn=callback, timeout=timeout
        )

    def _wait_cr(
        self,
        expected: Dict[str, str],
        get_cr_fn: Callable[[None], Any],
        timeout: int = 60,
    ) -> bool:
        """
        Waits for the custom resource (CR) to reach the expected state.

        Args:
            expected (dict): The expected state of the CR's spec.
            get_cr_fn (callable): The function to get the CR.
            timeout (int, optional): The timeout for checking the CR, defaults to 60.

        Returns:
            bool: True if the CR meets the expected state within the given timeout, False otherwise.
        """
        assertions = {key: False for key, _ in expected.items()}
        end_time = time.time() + timeout
        retry_count = 0
        while time.time() < end_time:
            if retry_count > 0:
                logging.warning(
                    f"CR is not in expected state, retry number: {retry_count}"
                )

            cr = get_cr_fn()
            for key, value in expected.items():
                if cr["spec"][key] == value:
                    assertions[key] = True

            if all(assertions.values()):
                return True

            time.sleep(1)
            retry_count += 1

        for k, v in assertions.items():
            if not v:
                logging.error(
                    f"CR is not in expected state: {k} != {expected[k]}"
                )

        return False

    def annotate_custom_resource(
        self,
        resource_name: str,
        resource_type: str,
        annotation_key: str,
        annotation_value: str,
        namespace: Optional[str] = None,
    ) -> None:
        """
        Annotates a custom resource with the given annotation key and value.

        Args:
            resource_name (str): The name of the custom resource.
            resource_type (str): The type of the custom resource.
            annotation_key (str): The key of the annotation.
            annotation_value (str): The value of the annotation.
            namespace (str, optional): The namespace of the custom resource. Defaults to None.

        Returns:
            None: This function does not return anything.
        """
        if namespace:
            custom_resource = (
                self.custom_objects_api.get_namespaced_custom_object(
                    const.CR_GROUP,
                    const.CR_VERSION,
                    namespace,
                    resource_type,
                    resource_name,
                )
            )
        else:
            custom_resource = (
                self.custom_objects_api.get_cluster_custom_object(
                    const.CR_GROUP,
                    const.CR_VERSION,
                    resource_type,
                    resource_name,
                )
            )

        annotations = custom_resource["metadata"].get("annotations", {})
        annotations[annotation_key] = annotation_value
        custom_resource["metadata"]["annotations"] = annotations

        if namespace:
            self.custom_objects_api.patch_namespaced_custom_object(
                const.CR_GROUP,
                const.CR_VERSION,
                namespace,
                resource_type,
                resource_name,
                custom_resource,
            )
        else:
            self.custom_objects_api.patch_cluster_custom_object(
                const.CR_GROUP,
                const.CR_VERSION,
                resource_type,
                resource_name,
                custom_resource,
            )

    def annotate_pvc(
        self,
        resource_name: str,
        annotation_key: str,
        annotation_value: str,
        namespace: str,
    ) -> None:
        """
        Annotates a PersistentVolumeClaim with the given annotation key and value.

        Args:
            resource_name (str): The name of the PersistentVolumeClaim.
            annotation_key (str): The key of the annotation.
            annotation_value (str): The value of the annotation.
            namespace (str): The namespace of the PersistentVolumeClaim.

        Returns:
            None: This function does not return anything.

        """
        pvc = self.core_v1_api.read_namespaced_persistent_volume_claim(
            name=resource_name, namespace=namespace
        )
        pvc.metadata.annotations[annotation_key] = annotation_value
        self.core_v1_api.patch_namespaced_persistent_volume_claim(
            name=resource_name, namespace=namespace, body=pvc
        )
