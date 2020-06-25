Bare-metal CSI Plugin
=====================

CSI spec implementation for local volumes.

Supported environments
----------------------
- Kubernetes
  - 1.15
- Node OS
  - Ubuntu 18.10
  - Ubuntu 16 ?
  - CentOS 7, 8 ?
- Helm
  - 2.13
  - 3.0
  
Features
--------

- [Dynamic provisioning](https://kubernetes-csi.github.io/docs/external-provisioner.html): Volumes are created dynamically when `PersistentVolumeClaim` objects are created.
- Handle REST calls for disk node mapping

### Planned features
- Custom scheduler
- Volume expand support
- User defined storage classes
- NVMe/NVMeOf devices support

Installation process
---------------------

Installation depend weather you use Helm 2 or Helm 3. If you use Helm 2 you have to setup correspond service account and clusterrolebinding for Tiller  component.

1. [Install helm on your local machine.](https://helm.sh/docs/intro/install/)  
    1.1 Prepare Tiller account (for Helm 2)
    ```
    1) kubectl create serviceaccount tiller --namespace kube-system 
    serviceaccount/tiller created
     
    2) kubectl create clusterrolebinding tiller-cluster-admin  --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
    clusterrolebinding.rbac.authorization.k8s.io/tiller-cluster-admin created
     
    3) helm init --service-account tiller --wait
    ......
    Happy Helming!
    
    4) Ensure that Tiller pod is running
    $ kubectl get pods -n kube-system | grep -i tiller
    tiller-deploy-7d44fddf6c-jgtjf             1/1     Running   0          2m35s
    ```

2. Install CSI plugin:

    2.1 For Helm 3 
    
    ```cd charts && helm install baremetal-csi ./baremetal-csi-plugin```
    
    2.2 For Helm 2
    
    ```helm install --name baremetal-csi ./baremetal-csi-plugin```
    
    Check CSI pods readiness
    
    ```
    $ kubectl get pods -o wide
    NAME                         READY   STATUS    RESTARTS   AGE
    baremetal-csi-controller-0   3/3     Running   0          179m
    baremetal-csi-node-2hp2k     3/3     Running   0          179m
    baremetal-csi-node-lz7xb     3/3     Running   0          179m
    baremetal-csi-node-p7r7w     3/3     Running   0          179m
    baremetal-csi-node-zjxzq     3/3     Running   0          179m   
    ```
    Check Storage Class
    
    ``` 
    $ kubectl get storageclass
    NAME                         PROVISIONER     RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
    baremetal-csi-sc (default)   baremetal-csi   Delete          WaitForFirstConsumer   false                  3h
    baremetal-csi-sc-hddlvg      baremetal-csi   Delete          WaitForFirstConsumer   false                  3h
    ```

Usages
------
 
Provide `baremetal-csi-sc` storage class for PVC in PVC manifest or in persistenVolumeClameTemapate section if you need 
to provision PVC based on HDD disk and that whole disk will be consumed by that PVC. In that case size of PVC will be 
not less then you required and will be equal size of whole underlying HDD drive size.

Provide `baremetal-csi-sc-hddlvg` storage class for PVC in PVC manifest or persistenVolumeClameTemapate section if you 
need to provision PVC based on logical volume. In that case logical volume group is created on the system based on one 
drive and there are could be multiple logical volumes associated with multiple PVCs from one logical volume group. 
Size of PVC will be as is requested in manifest.
  
For developers
---------------------

1. Compile proto files
    1.1 There is `make` target 'compile-proto' that will generate GO code from proto files:
    ```
    make compile-proto
    ``` 
    Proto files are located under `/api/API_VERSION/` folder. Generated GO files will be located under `/api/generated/API_VERSION` folder.
    Default API_VERSION is `v1`
2. Installs controller-gen tool
    ```
   make install-controller-gen
    ```
3. Generate CRDs manifests and code
    2.1 There is `make` target 'generate-crd' that will generate CRD yaml manifests:
    ```
    make generate-crd
    ```
    Manifests are located under `charts/baremetal-csi-plugin/crds` folder.
   
    2.2 There is `make` target 'generate-deepcopy' that will generate GO deepcopy code for CRDs instances:
    ```
    make generate-deepcopy 
    ```
    Go files are located under `api/vi/SOME_CRD/` folder