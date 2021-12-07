# Supported Mount Options

## Usage
User can set options in Storage Class in mountOptions section. 
They will be applied for all Volumes with the following SC.

Example:
```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: sc
provisioner: csi-baremetal  
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
parameters:
  storageType: ANY
  fsType: xfs
mountOptions:
  - noatime
```

## List of supported options

- Name: noatime
  
    Effect: Add "noatime" option to mount command on NodePublishRequest
    
    Example: `mount -o noatime /src /dst`
