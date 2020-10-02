Bare-metal CSI Plugin
=====================

Bare-metal CSI Plugin is a [CSI spec](https://github.com/container-storage-interface/spec) implementation to manage locally attached drives for Kubernetes.

- **Project status**: Alpha - no backward compatibility guaranteed   

Supported environments
----------------------
- **Kubernetes**: 1.18
- **Node OS**: Ubuntu 18.10  
- **Helm**: 3.0
  
Features
--------

- [Dynamic provisioning](https://kubernetes-csi.github.io/docs/external-provisioner.html): Volumes are created dynamically when `PersistentVolumeClaim` objects are created.
- LVM support
- Storage classes for the different drive types: HDD, SSD, NVMe

### Planned features
- Service procedures - node and disk replacement
- Volume expand support
- User defined storage classes
- NVMeOf support
- Cross-platform

Installation process
---------------------

1. Pre-requisites
 
    1.1. *go version 1.14.2*
    
    1.2. *protoc version 3*
    
    1.3. [*helm*](https://helm.sh/docs/intro/install/)
    
    1.4. *kubectl*

2. Build and deploy CSI plugin
    
    2.1 Build Bare-metal CSI Plugin images and push them to your registry server
    
    ```REGISTRY=<your-registry.com> make generate-api build images```

    2.2 Deploy CSI plugin 
    
    ```cd charts && helm install csi-baremetal baremetal-csi-plugin --set global.registry=<your-registry.com> --set image.tag=<tag>```
    
    2.3 Deploy Kubernetes scheduler extender 
        
    ```cd charts && helm install csi-scheduler-extender scheduler-extender --set  global.registry=<your-registry.com> --set image.tag=<tag>```
    
3. Check default storage classes available

    ```kubectl get storageclasses```
    
Usage
------
 
Use `baremetal-csi-sc` storage class for PVC in PVC manifest or in persistentVolumeClaimTemplate section if you need to 
provision PV bypassing LVM. Size of the resulting PV will be equal to the size of underlying physical drive.

Use `baremetal-csi-sc-hddlvg` or `baremetal-csi-sc-ssdlvg` storage classes for PVC in PVC manifest or in 
persistentVolumeClaimTemplate section if you need to provision PVC based on the logical volume. Size of the resulting PV
will be equal to the size of PVC. 
  
For developers
---------------------

1. Compile proto files  
    ```
    make compile-proto
    ```     
2. Install controller-gen tool
    ```
   make install-controller-gen
    ```
3. Generate CRD manifests and code

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