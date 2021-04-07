Bare-metal CSI Driver
=====================

Bare-metal CSI Driver is a [CSI spec](https://github.com/container-storage-interface/spec) implementation to manage locally attached drives for Kubernetes.

- **Project status**: Beta - no backward compatibility is provided   

Supported environments
----------------------
- **Kubernetes**: 1.18, 1.19
- **OpenShift**: 4.6
- **Node OS**:
  - Ubuntu 18.04 LTS
  - Red Hat Enterprise Linux 7.7 / CoreOS 4.6   
  - CentOS Linux 7.9 / 
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
- Service procedures - node and disk replacement
- Volume expand support
- Raw block mode
- Ability to deploy on subset of nodes within cluster

### Planned features
- User defined storage classes
- NVMeOf support
- CSI Operator
- Kubernetes Scheduler
- SMART Self Test execution
- Volume cloning
- Support of additional Linux distributions/versions

Related repositories
--------
- [Bare-metal CSI Operator](https://github.com/dell/csi-baremetal-operator) - Kubernetes Operator to deploy and manage CSI
- [Bare-metal CSI Scheduling](https://github.com/dell/csi-baremetal-scheduling) - Kubernetes Scheduler and Scheduler Extender to guarantee correct pod placement

Installation process
---------------------

1. Pre-requisites
    
    1.1. Build
 
    - *go version 1.14.2*
    
    - *protoc version 3*
        
    1.2. Installation 
    
    -  *lvm2* packet installed on the Kubernetes nodes
    
    - [*helm*](https://helm.sh/docs/intro/install/)
    
    - *kubectl*    

2. Build CSI driver
    
    2.1 Build binaries
    
    ```make generate-api build```
    
    2.2 Build images
        
    ```REGISTRY=<your-registry.com> make images push```
    
    2.3 Push images to your registry server
        
    ```REGISTRY=<your-registry.com> make push```
    
3. Deploy CSI Driver

    3.1 Deploy CSI Node Operator
    
    ```helm install csi-baremetal-operator charts/csi-baremetal-operator --set global.registry=<your-registry.com> --set image.tag=<tag>```
    
    3.2 Deploy CSI plugin 
    
    ```helm install csi-baremetal-driver charts/csi-baremetal-driver --set global.registry=<your-registry.com> --set image.tag=<tag> --set feature.extender=true```
    
    3.3 Deploy Kubernetes scheduler extender 
        
    ```helm install csi-baremetal-scheduler-extender charts/csi-baremetal-scheduler-extender --set registry=<your-registry.com> --set image.tag=<tag>```
    
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
 
Use `csi-baremetal-sc` storage class for PVC in PVC manifest or in persistentVolumeClaimTemplate section if you need to 
provision PV bypassing LVM. Size of the resulting PV will be equal to the size of underlying physical drive.

Use `csi-baremetal-sc-hddlvg` or `csi-baremetal-sc-ssdlvg` storage classes for PVC in PVC manifest or in 
persistentVolumeClaimTemplate section if you need to provision PVC based on the logical volume. Size of the resulting PV
will be equal to the size of PVC.

Contribution
------
Please refer [Contribution Guideline](https://github.com/dell/csi-baremetal/blob/master/docs/CONTRIBUTING.md) fo details
