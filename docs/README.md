[![PR validation](https://github.com/dell/csi-baremetal/actions/workflows/pr.yml/badge.svg)](https://github.com/dell/csi-baremetal/actions/workflows/pr.yml)
[![codecov](https://codecov.io/gh/dell/csi-baremetal/branch/master/graph/badge.svg)](https://codecov.io/gh/dell/csi-baremetal)

Bare-metal CSI Driver
=====================

Bare-metal CSI Driver is a [CSI spec](https://github.com/container-storage-interface/spec) implementation to manage locally attached disks for Kubernetes.

- **Project status**: Beta - no backward compatibility is provided   

Supported environments
----------------------
- **Kubernetes**: 1.18, 1.19, 1.20
- **OpenShift**: 4.6
- **Node OS**:
  - Ubuntu 18.04 / 20.04 LTS
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
 
    - *docker >= 17.09*
    
    - *go version >= 1.15.2*

    - *protoc version 3* & *protoc-gen-go 1.3.5*

        - To install execute `make install-compile-proto`

    - *controller-gen 0.5.0*

        - To install execute `make install-controller-gen`
        
    1.2. Installation 
    
    -  *lvm2* packet installed on the Kubernetes nodes
    
    - [*helm*](https://helm.sh/docs/intro/install/)
    
    - *kubectl*    

2. Build CSI driver
    
    2.1 Build binaries
    
    ```make generate-deepcopy build```
    
    2.2 Build images
        
    ```REGISTRY=<your-registry.com> make images```
    
    2.3 Push images to your registry server
        
    ```REGISTRY=<your-registry.com> make push```

3. Build CSI Operator (https://github.com/dell/csi-baremetal-operator)

4. Deploy CSI Operator (use charts from csi-baremetal-operator repo)

    ```helm install csi-baremetal-operator charts/csi-baremetal-operator --set global.registry=<your-registry.com> --set image.tag=<tag>```

4. Deploy CSI Driver (use charts from csi-baremetal-operator repo)

    - Vanilla Kubernetes
        
    ```helm install csi-baremetal charts/csi-baremetal-deployment --set registry=<your-registry.com> --set image.tag=<tag> --set driver.drivemgr.type=halmgr```

    - OpenShift

    ```helm install csi-baremetal charts/csi-baremetal-deployment --set registry=<your-registry.com> --set image.tag=<tag> --set driver.drivemgr.type=halmgr --set platform=openshift```

    - RKE

   ```helm install csi-baremetal charts/csi-baremetal-deployment --set registry=<your-registry.com> --set image.tag=<tag> --set driver.drivemgr.type=halmgr --set platform=rke```
    
4. Check default storage classes available

    ```kubectl get storageclasses```

5. To obtain information about:

    5.1 Node IDs assigned by CSI:

    ```kubectl get nodes.csi-baremetal.dell.com```

    5.2 Local Drives discovered by CSI:

    ```kubectl get drives.csi-baremetal.dell.com```

    5.3 Capacity available for allocation:

    ```kubectl get  availablecapacities.csi-baremetal.dell.com```

    5.4 Provisioned logical volume groups:

    ```kubectl get logicalvolumegroups.csi-baremetal.dell.com```

    5.4 Provisioned volumes:

    ```kubectl get volumes.csi-baremetal.dell.com```


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
