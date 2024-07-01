import os
import logging
import time
import pytest
from kubernetes import client, config
from kubernetes.client.rest import ApiException

class STS:
    def __init__(self, namespace: str, name: str, replicas: int = 1) -> None:
        self.namespace = namespace
        self.name = name
        self.replicas = replicas
        self.image = "asdrepo.isus.emc.com:9042/alpine:3.18.4"

        if 'KUBERNETES_SERVICE_HOST' in os.environ and 'KUBERNETES_SERVICE_PORT' in os.environ:
            config.load_incluster_config()
        else:
            config.load_kube_config()

        configuration = client.Configuration().get_default_copy()
        configuration.verify_ssl = False
        configuration.assert_hostname = False
        client.Configuration.set_default(configuration)

        self.apps_v1_api = client.AppsV1Api()

    def create(self, storage_classes) -> None:
        volume_mounts = []
        volume_claim_templates = []
        for index, storage_class in enumerate(storage_classes):
            volume_mount = client.V1VolumeMount(
                                    name="volume"+str(index),
                                    mount_path="/mnt/volume"+str(index))
            volume_mounts.append(volume_mount)
            volume_claim_template = client.V1PersistentVolumeClaim(
                        api_version="v1",
                        kind="PersistentVolumeClaim",
                        metadata=client.V1ObjectMeta(
                            name="volume"+str(index)
                        ),
                        spec=client.V1PersistentVolumeClaimSpec(
                            access_modes=[
                                "ReadWriteOnce"
                            ],
                            storage_class_name=storage_class,
                            resources=client.V1VolumeResourceRequirements(
                                requests={
                                    "storage": "50Mi"
                                }
                            )
                        )
                    )
            volume_claim_templates.append(volume_claim_template)
            
        body = client.V1StatefulSet(
            api_version="apps/v1",
            kind="StatefulSet",
            metadata=client.V1ObjectMeta(
                name=self.name,
                namespace=self.namespace
            ),
            spec=client.V1StatefulSetSpec(
                replicas=self.replicas,
                persistent_volume_claim_retention_policy=client.V1StatefulSetPersistentVolumeClaimRetentionPolicy(
                    when_deleted="Delete"
                ),
                service_name=self.name,
                selector=client.V1LabelSelector(
                    match_labels={
                        "app": self.name
                    }
                ),
                template=client.V1PodTemplateSpec(
                    metadata=client.V1ObjectMeta(
                        labels={
                            "app": self.name
                        }
                    ),
                    spec=client.V1PodSpec(
                        termination_grace_period_seconds=1,
                        containers=[
                            client.V1Container(
                                name=self.name,
                                image=self.image,
                                image_pull_policy="IfNotPresent",
                                command=[
                                    "sleep",
                                    "infinity",
                                ],
                                volume_mounts=volume_mounts
                            )
                        ]
                    )
                ),
                volume_claim_templates=volume_claim_templates
            )
        )

        try:
            response = self.apps_v1_api.create_namespaced_stateful_set(
                self.namespace,
                body)
            assert response is not None, f"Failed to create StatefulSet: {self.name}"
        except ApiException as exc:
            pytest.fail(f"Failed to create StatefulSet: {self.name}. Reason: {str(exc)}")

    def delete(self) -> None:
        try:
            response = self.apps_v1_api.delete_namespaced_stateful_set(
                self.name,
                self.namespace)
            assert response is not None, f"Failed to delete StatefulSet: {self.name}"
        except ApiException as exc:
            logging.warning(f"Failed to delete StatefulSet: {self.name}. Reason: {str(exc)}")

    def verify(self, timeout: int) -> bool:
        start_time = time.time()
        result = False

        while result is False:
            try:
                logging.info(f'Waiting for STS {self.name} reaching {self.replicas} replica count...{int(time.time() - start_time)}/{timeout}s')
                time.sleep(2)

                response = self.apps_v1_api.read_namespaced_stateful_set(
                    self.name,
                    self.namespace)
                assert response is not None, f"Failed to read StatefulSet: {self.name}"

                result = (response.status.available_replicas == self.replicas
                          and response.status.ready_replicas == self.replicas
                          and response.status.replicas == self.replicas)

                if time.time() - start_time > timeout:
                    return result
            except ApiException as exc:
                logging.warning(f"Failed to read StatefulSet: {self.name}. Reason: {str(exc)}")

        return result
