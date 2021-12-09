# Title: Pre-provisioned Volumes support

Last updated: 9-Dec-2021 


## Abstract

CSI must be able to work with pre-provisioned volumes.

## Background

Pre-provisioned volumes are volumes whuch we created manually by administrator. 

## Proposal

1. Choose the drive which you plan to use
2. Make sure that it has available capacity fits to your volume
3. Generate volume UUID
```
uuidgen
```
4. Create Volume custom resource
```
apiVersion: csi-baremetal.dell.com/v1
kind: Volume
metadata:
  finalizers:
  - dell.emc.csi/volume-cleanup
  name: pvc-<UUID>
  namespace: default
spec:
  CSIStatus: CREATING
  Health: GOOD
  Id: pvc-<UUID>
  Location: <CSI drive UUID>
  LocationType: DRIVE
  Mode: <FS/RAW/RAW_PART>
  NodeId: <CSI node UUID>
  OperationalStatus: OPERATIVE
  Size: <CSI drive size>
  StorageClass: <Storage Class>
  # For FS Mode only  
  Type: <FS TYPE>
  Usage: IN_USE
```
5. Wait for CSIStatus to be CREATED
6. Create Persistent Volume
```
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pvc-<UUID>
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: <SIZE> 
  csi:
    driver: csi-baremetal
    fsType: <FS TYPE> 
    volumeAttributes:
      csi.storage.k8s.io/pv/name: pvc-<UUID>
      csi.storage.k8s.io/pvc/namespace: <NAMESPACE>
      fsType: <FS TYPE> 
      storageType: <Storage Class> 
    volumeHandle: pvc-<UUID>
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: nodes.csi-baremetal.dell.com/uuid
          operator: In
          values:
          - <CSI Node UUID>
  persistentVolumeReclaimPolicy: Delete
  storageClassName: <csi-baremetal sc name>
  volumeMode: <Filesystem/Raw>
```
7. Create Kubernetes Persistent Volume Claim
```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: <claim name>
  namespace: <NAMESPACE>
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: <size>
  storageClassName: <csi-baremetal sc name>
  volumeMode: <Filesystem/Raw>
  volumeName: pvc-<UUID>
```
8. Use PVC for your application

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Compatibility

[A discussion of the change with regard to the compatibility with previous product and Kubernetes versions.]

## Implementation

[A description of the steps in the implementation, who will do them, and when.]

## Assumptions (if applicable)

ID | Name | Descriptions | Comments
---| -----| -------------| --------
ASSUM-1 |   |   |


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
