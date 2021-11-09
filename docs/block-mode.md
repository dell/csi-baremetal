# Block mode usage

### Default block mode

1. Set VolumeMode on PVC

```
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      storageClassName: csi-baremetal-sc
      accessModes: [ "ReadWriteOnce" ]
      volumeMode: Block
```

2. Use volumeDevices instead of volumeMounts

```
    volumeDevices:
    - name: data
      devicePath: /data
```

Example:

```
root@vm-037:~# lsblk /dev/sdb
NAME MAJ:MIN RM SIZE RO TYPE MOUNTPOINT
sdb    8:16   0  10G  0 disk 
```

### LVG block mode

All LVG storage classes allows creating Block Volumes too. In this case device will be LVG partition.

Example:

```
root@vm-037:~# lsblk /dev/sdd
NAME                                                                                     MAJ:MIN RM SIZE RO TYPE MOUNTPOINT
sdd                                                                                        8:48   0  10G  0 disk 
├─fe44c368--a4a4--4424--b113--db3526d055e4-pvc--3a87a913--2d8e--4321--b954--fd24cd8b94a0 253:3    0  20M  0 lvm  
├─fe44c368--a4a4--4424--b113--db3526d055e4-pvc--f0e44041--877f--4427--bbb7--7bcae3bf4b13 253:4    0  20M  0 lvm  
└─fe44c368--a4a4--4424--b113--db3526d055e4-pvc--03f5ad88--5332--4cba--8bc0--3534e360ae77 253:5    0  20M  0 lvm  
```

### Raw Part Mode

If it is necessary, CSI Baremetal can create partition for drive and set partition as volume device.

1. Create new storage class and add `isPartitioned=true` parameter

```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-baremetal-sc-raw-part
parameters:
  fsType: xfs
  storageType: ANY
  isPartitioned: true
provisioner: csi-baremetal
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```

Note! KubeAPI forbids to edit existing storage classes
```
The StorageClass "csi-baremetal-sc" is invalid: parameters: Forbidden: updates to parameters are forbidden.
```

2. Follow `Default block mode`

Example:

```
root@vm-037:~# lsblk /dev/sdb
NAME   MAJ:MIN RM SIZE RO TYPE MOUNTPOINT
sdb      8:16   0  10G  0 disk 
└─sdb1   8:17   0  10G  0 part 
```
