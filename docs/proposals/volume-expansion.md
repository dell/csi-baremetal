# Proposal: Volume Expansion

Last updated: 09.02.21

## Abstract

This proposal contains design and implementation considerations about volume expansion pf LVG volumes

## Background

Currently CSI doesn't support volume expansion feature, this proposal aims to solve this problem for LVG volumes.

## Proposal

There are two requests in CSI spec, that can be implemented: `ControllerExpandVolume` and `NodeExpandVolume`. Also there are 
2 types of Volume Expansion: `ONLINE` and `OFFLINE`. The differences is that online volume expansion indicates that volumes may be expanded
 when published to a node. `NodeExpandVolume` always perform online volume expansion, so `NodeExpandVolume` must be called after NodeStage request and may be called 
after NodePublishVolume. `ControllerExpandVolume` can perform both type of volume expansion. In case of offline, if volume is published, `ControllerExpandVolume` must be called
after ControllerUnpublishVolume/NodeUnstageVolume/NodeUnpublishVolume depends on supported capabilities. 

To support online and offline volume expansion we can:
 - Implement both `ControllerExpandVolume` and `NodeExpandVolume`.
`NodeExpandVolume` is typically used to resize file system. So `ControllerExpandVolume` can perform lvm command:
 ```
 lvextend --size <requiredSize> LV 
 ``` 
 - `NodeExpandVolume` will perform file system resizing:
    - `fsadm resize` is for XFS file system. We can't use `xfs_growfs` because it doesn't work for unmounted XFS
    - `resize2fs` or `fsadm resize` is for EXT3 and EXT4 file system. For offline mode we should also use `fsck` or `e2fsck` before these commands.

## Rationale

Another approach to support online and offline volume expansion is:
 - Implement only `ControllerExpandVolume`. We can use lvm command:
 ```
 lvextend --resizefs --size <requiredSize> LV
 ``` 
This command allow extend LV size and resize fs (using fsadm) at the same time. 

1) What to do in case of insufficient space on vg? Should CSI throw error of try to create another PV? In this case 
we need to use free driver and reservation, which can cause additional complexity in volume expansion logic.
2) What to do with potential data loss for both type of volume expansion? Should CSI prepare backup somehow?

## Compatibility

* Min K8S Version is 1.16 
* Min CSI Spec version is 1.1.0 (1.2.0 if we want to get staging_path in `NodeExpandVolume` request)
* Min external-resizer version is 0.2 
* Modern kernels support file system resize

## Implementation

Helm charts:
1) Add new sidecar container: `external-resizer` and package it
2) Add new property in CSI StorageClass `allowVolumeExpansion:true`

Identity Server:
1) Add following capabilities:
2) csi.PluginCapability_VolumeExpansion_ONLINE
3) csi.PluginCapability_VolumeExpansion_OFFLINE

Controller Service:
1) Add capabilities csi.ControllerServiceCapability_RPC_EXPAND_VOLUME
2) Implement `ControllerExpandVolume` request
3) Add additional statuses for volume - `expanding` and `expanded`
4) ControllerExpandVolume update status of volume

Volume Controller:
1) Optional: if VG space is less than required capacity, try to reserve AvailableCapacity if we want to extend volume group and create new PV
2) Volume controller handle new status and call:
 ```
 lvextend --size <requiredSize> LV
 ``` 
3) Optional: LVG controller also must extend VG in case of new PV added by calling `vgextend`

Node Service:
1) Add capability csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
2) Implement `NodeExpandVolume` request
3) Determine file system type by using standard utilities like `lsblk` (can we store file system in volume spec?)
4) Call `fsadm resize` in case of XFS
5) Call `fsadm resize` or `resize2fs` in case of EXT3 and EXT4 (for offline expansion we need to call `e2fsck` or `fsck` before)

## Testing
* E2E framework contains testsuites for volume expansion. We need to add capability `CapControllerExpansion` to testing driver and add that test suited in our setup
* Acceptance test, which can be implemented based on updating PVC size

## Open issues

ID      | Name    | Descriptions | Status | Comments
--------| --------| -------------| ------ | --------
ISSUE-1 |What to do in case  of insufficient space on vg | If VG size is less than requiredCapacity CSI can return error or try to use free driver| Open | In this case we need to use free driver and use reservation, which can cause additional complexity in volume expansion logic. 
ISSUE-2 |What to do with potential data loss for both type of volume expansion? | Should CSI prepare backup| Open |                            
ISSUE-3 |How to determine file system type? | To resize file system we need to determine its type, because we use different utilities| Open | Can we store fs type in volume?                           

