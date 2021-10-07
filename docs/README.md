[![PR validation](https://github.com/dell/csi-baremetal/actions/workflows/pr.yml/badge.svg)](https://github.com/dell/csi-baremetal/actions/workflows/pr.yml)
[![codecov](https://codecov.io/gh/dell/csi-baremetal/branch/master/graph/badge.svg)](https://codecov.io/gh/dell/csi-baremetal)

Bare-metal CSI Driver
=====================

Bare-metal CSI Driver is a [CSI spec](https://github.com/container-storage-interface/spec) implementation to manage locally attached disks for Kubernetes.

- **Project status**: Beta - no backward compatibility is provided   

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
- CSI Operator

### Planned features
- User defined storage classes
- NVMeOf support
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

Installation process is documented in [Bare-metal CSI Operator](https://github.com/dell/csi-baremetal-operator)

Usage
------

* Storage classes

    * Use storage class without `lvg` postfix if you need to provision PV bypassing LVM. Size of the resulting PV will
    be equal to the size of underlying physical drive.

    * Use storage class with `lvg` postfix if you need to provision PVC based on the logical volume. Size of the
    resulting PV will be equal to the size of PVC.

* To obtain information about:

    * Node IDs assigned by CSI - `kubectl get nodes.csi-baremetal.dell.com`

    * Local Drives discovered by CSI - `kubectl get drives.csi-baremetal.dell.com`

    * Capacity available for allocation - `kubectl get  availablecapacities.csi-baremetal.dell.com`

    * Provisioned logical volume groups - `kubectl get logicalvolumegroups.csi-baremetal.dell.com`

    * Provisioned volumes - `kubectl get volumes.csi-baremetal.dell.com`
 

Contribution
------
Please refer [Contribution Guideline](https://github.com/dell/csi-baremetal/blob/master/docs/CONTRIBUTING.md) fo details
