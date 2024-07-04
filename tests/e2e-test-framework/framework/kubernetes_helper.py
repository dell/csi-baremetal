import os
from kubernetes import client, config


class KubernetesHelper:
    @classmethod
    def load_kube_api(cls):
        if 'KUBERNETES_SERVICE_HOST' in os.environ and 'KUBERNETES_SERVICE_PORT' in os.environ:
            config.load_incluster_config()
        else:
            config.load_kube_config()

        configuration = client.Configuration().get_default_copy()
        configuration.verify_ssl = False
        configuration.assert_hostname = False
        client.Configuration.set_default(configuration)

        api_client = client.ApiClient()
        core_v1_api = client.CoreV1Api(api_client)
        custom_objects_api = client.CustomObjectsApi(api_client)
        apps_v1_api = client.AppsV1Api(api_client)
        network_v1_api = client.NetworkingV1Api(api_client)
        coordination_v1_api = client.CoordinationV1Api(api_client)
        return core_v1_api, custom_objects_api, apps_v1_api, network_v1_api, coordination_v1_api