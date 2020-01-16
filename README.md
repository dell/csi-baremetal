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
- LVM support for micro partitioning [FABRIC-8367](https://asdjira.isus.emc.com:8443/browse/FABRIC-8367)
- NVMe devices support

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
    NAME                            READY   STATUS    RESTARTS   AGE    IP            NODE               NOMINATED NODE   READINESS GATES
    baremetal-csiplugin-8dxc5       3/3     Running   0          118s   10.244.2.14   shmelr-ubuntu-32   <none>           <none>
    baremetal-csiplugin-cw6pw       3/3     Running   0          118s   10.244.1.21   shmelr-ubuntu-31   <none>           <none>
    baremetal-csiplugin-j99gn       3/3     Running   0          118s   10.244.3.10   shmelr-ubuntu-33   <none>           <none>
    baremetal-csiplugin-l2qjx       3/3     Running   0          118s   10.244.4.12   shmelr-ubuntu-34   <none>           <none>
    csi-do-controller-0             5/5     Running   0          118s   10.244.1.22   shmelr-ubuntu-31   <none>           <none>
    deployment-1-56f94b4c5c-br44b   1/1     Running   27         27h    10.244.2.13   shmelr-ubuntu-32   <none>           <none>
    deployment-1-56f94b4c5c-h27m4   1/1     Running   27         27h    10.244.4.11   shmelr-ubuntu-34   <none>           <none>
    ```
    Check Storage Class
    
    ``` 
    $ kubectl get storageclass
    NAME                         PROVISIONER     AGE
    baremetal-csi-sc (default)   baremetal-csi   3m23s
    ```

Usages
------
 
Provide `baremetal-csi` storage class for PVC in PVC manifest or persistenVolumeClameTemapate section. 


For developers
---------------------

1. Compile proto files
    1.1 There is make target 'compile-proto' that will generate GO code from proto files:
    ```
    make compile-proto
    ``` 
    Proto files located under `/api/API_VERSION/` folder. Generated GO files will be located under `/api/generated/API_VERSION` folder.
    Default API_VERSION is `v1`