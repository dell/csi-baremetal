Bare-metal CSI Plugin
=====================

Bare-metal CSI Plugin is a [CSI spec](https://github.com/container-storage-interface/spec) implementation to manage locally attached drives for Kubernetes.

- **Project status**: Beta - no backward compatibility is provided   

Supported environments
----------------------
- **OpenShift**: 4.6
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
- Service procedures - node and disk replacement
- Volume expand support
- Raw block mode

### Planned features
- User defined storage classes
- NVMeOf support
- Cross-platform

Installation process
---------------------

1. Pre-requisites
 
    1.1. *lvm2* packet installed on the Kubernetes nodes  
        
    1.2. [*helm*](https://helm.sh/docs/intro/install/)
    
    1.4. *kubectl*

2. Deploy CSI driver
        
    2.1 Deploy CSI Node Operator 
    
    ```helm install csi-baremetal-operator charts/csi-baremetal-operator```
    
    2.2 Deploy CSI Driver
    
    ```helm install csi-baremetal-driver charts/csi-baremetal-driver --set drivemgr.type=halmgr```
    
    2.3 Deploy Kubernetes scheduler extender
    
    * Vanilla Kubernetes
    
    ```helm install csi-baremetal-scheduler-extender charts/csi-baremetal-scheduler-extender --set patcher.enable=true```
    
    * OpenShift
    
    ```helm install csi-baremetal-scheduler-extender charts/csi-baremetal-scheduler-extender```
    
    ```pkg/scheduler/patcher/openshift_patcher.sh --install```
    
3. Check default storage classes available

    ```kubectl get storageclasses```

Usage
------
 
Use `csi-baremetal-sc` storage class for PVC in PVC manifest or in persistentVolumeClaimTemplate section if you need to 
provision PV bypassing LVM. Size of the resulting PV will be equal to the size of underlying physical drive.

Use `csi-baremetal-sc-hddlvg` or `csi-baremetal-sc-ssdlvg` storage classes for PVC in PVC manifest or in 
persistentVolumeClaimTemplate section if you need to provision PVC based on the logical volume. Size of the resulting PV
will be equal to the size of PVC.

Uninstallation process
---------------------

1. Delete related _PVCs_

    ```kubectl delete pvc <pvc name>```

2. Delete _volumes.csi-baremetal.dell.com_ custom resources

    ```kubectl delete volumes.csi-baremetal.dell.com --all```
    
3. Delete _logicalvolumegroups.csi-baremetal.dell.com_ custom resources
    
    ```kubectl delete logicalvolumegroups.csi-baremetal.dell.com --all```
    
4. Delete _nodes.csi-baremetal.dell.com_ custom resources

    ```kubectl delete nodes.csi-baremetal.dell.com --all```
    
5. Restore scheduler settings - OpenShift ONLY!
    
    ```pkg/scheduler/patcher/openshift_patcher.sh --uninstall```
    
6. Delete helm releases

    ```helm delete csi-baremetal-scheduler-extender csi-baremetal-driver csi-baremetal-operator```

7. Delete CSI CRDs

    ```kubectl delete crd availablecapacities.csi-baremetal.dell.com availablecapacityreservations.csi-baremetal.dell.com logicalvolumegroups.csi-baremetal.dell.com volumes.csi-baremetal.dell.com drives.csi-baremetal.dell.com nodes.csi-baremetal.dell.com```

Contribution
------
Please refer [Contribution Guideline](https://github.com/dell/csi-baremetal/blob/master/docs/CONTRIBUTING.md) fo details
