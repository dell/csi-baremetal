Bare-metal CSI Plugin
=====================

Bare-metal CSI Plugin is a [CSI spec](https://github.com/container-storage-interface/spec) implementation to manage locally attached drives for Kubernetes.

- **Project status**: Alpha - no backward compatibility is provided   

Supported environments
----------------------
- **Kubernetes**: 1.18
- **Node OS**: Ubuntu 18.10  
- **Helm**: 3.0
  
Features
--------

- [Dynamic provisioning](https://kubernetes-csi.github.io/docs/external-provisioner.html): Volumes are created dynamically when `PersistentVolumeClaim` objects are created.
- Inline volumes
- LVM support
- Storage classes for the different drive types: HDD, SSD, NVMe
- Drive health detection
- Scheduler extender
- Support unique ID for each node in the K8s cluster

### Planned features
- Service procedures - node and disk replacement
- Volume expand support
- User defined storage classes
- NVMeOf support
- Cross-platform
- Raw block mode

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
    
    ```cd charts && helm install csi-baremetal-driver csi-baremetal-driver --set global.registry=<your-registry.com> --set image.tag=<tag> --set feature.extender=true```
    
    2.3 Deploy Kubernetes scheduler extender 
        
    ```cd charts && helm install csi-baremetal-scheduler-extender csi-baremetal-scheduler-extender --set registry=<your-registry.com> --set image.tag=<tag>```
    
3. Check default storage classes available

    ```kubectl get storageclasses```

4. Unique node ID support
   In order to support physical [node replacement](https://github.com/dell/csi-baremetal/blob/master/docs/proposals/node_replacement.md) during which drives remain same CSIBMNode operator should be installed before plugin and extender installation.
    
    ``` helm install operator charts/csibm-operator --set image.registry=<your-registry.com> --set image.tag=<tag> ```
   All options could be found in [values.yaml](https://github.com/dell/csi-baremetal/blob/master/charts/csibm-operator/values.yaml)'

   For using generated ID in plugin and extender they should be installed with next feature option:
   ``` --set feature.usenodeannotation=true ```

Usage
------
 
Use `baremetal-csi-sc` storage class for PVC in PVC manifest or in persistentVolumeClaimTemplate section if you need to 
provision PV bypassing LVM. Size of the resulting PV will be equal to the size of underlying physical drive.

Use `baremetal-csi-sc-hddlvg` or `baremetal-csi-sc-ssdlvg` storage classes for PVC in PVC manifest or in 
persistentVolumeClaimTemplate section if you need to provision PVC based on the logical volume. Size of the resulting PV
will be equal to the size of PVC.

Contribution
------
Please refer [Contribution Guideline](https://github.com/dell/csi-baremetal/blob/master/docs/CONTRIBUTING.md) fo details
