# Proposal: Volume Expansion

Last updated: 17.02.21

## Abstract

This proposal contains design and implementation considerations about volume expansion of LVG volumes

## Background

Currently CSI doesn't support volume expansion feature, this proposal aims to solve this problem for LVG volumes.

## Proposal

There are two requests in CSI spec, that can be implemented: `ControllerExpandVolume` and `NodeExpandVolume`. Also there are 
2 types of Volume Expansion: `ONLINE` and `OFFLINE`. The differences is that online volume expansion indicates that volumes may be expanded
 when published to a node. `NodeExpandVolume` always perform online volume expansion, so `NodeExpandVolume` must be called after NodeStage request and may be called 
after NodePublishVolume. `ControllerExpandVolume` can perform both type of volume expansion. In case of offline, if volume is published, `ControllerExpandVolume` must be called after ControllerUnpublishVolume/NodeUnstageVolume/NodeUnpublishVolume depends on supported capabilities. 

To support online and offline volume expansion we can:
 - Implement both `ControllerExpandVolume` and `NodeExpandVolume`.
`NodeExpandVolume` is typically used to resize file system. So `ControllerExpandVolume` can perform lvm command:
 ```
 lvextend --size <requiredSize> LV 
 ``` 
 - `NodeExpandVolume` will perform file system resizing:
    - `fsadm resize` is for XFS file system. We can't use `xfs_growfs` because it doesn't work for unmounted XFS
    - `resize2fs` or `fsadm resize` is for EXT3 and EXT4 file system. For offline mode we should also use `fsck` or `e2fsck` before these commands.

##### ControllerExpandVolume request and response
```protobuf
message ControllerExpandVolumeRequest {
  // The ID of the volume to expand. This field is REQUIRED.
  string volume_id = 1;

  // This allows CO to specify the capacity requirements of the volume
  // after expansion. This field is REQUIRED.
  CapacityRange capacity_range = 2;

  // Secrets required by the plugin for expanding the volume.
  // This field is OPTIONAL.
  map<string, string> secrets = 3 [(csi_secret) = true];

  // Volume capability describing how the CO intends to use this volume.
  // This allows SP to determine if volume is being used as a block
  // device or mounted file system. For example - if volume is
  // being used as a block device - the SP MAY set
  // node_expansion_required to false in ControllerExpandVolumeResponse
  // to skip invocation of NodeExpandVolume on the node by the CO.
  // This is an OPTIONAL field.
  VolumeCapability volume_capability = 4;
}

message ControllerExpandVolumeResponse {
  // Capacity of volume after expansion. This field is REQUIRED.
  int64 capacity_bytes = 1;

  // Whether node expansion is required for the volume. When true
  // the CO MUST make NodeExpandVolume RPC call on the node. This field
  // is REQUIRED.
  bool node_expansion_required = 2;
}
```

##### ControllerExpandVolume Errors

| Condition | gRPC Code | Description | Recovery Behavior |
|-----------|-----------|-------------|-------------------|
| Exceeds capabilities | 3 INVALID_ARGUMENT | Indicates that CO has specified capabilities not supported by the volume. | Caller MAY verify volume capabilities by calling ValidateVolumeCapabilities and retry with matching capabilities. |
| Volume does not exist | 5 NOT FOUND | Indicates that a volume corresponding to the specified volume_id does not exist. | Caller MUST verify that the volume_id is correct and that the volume is accessible and has not been deleted before retrying with exponential back off. |
| Volume in use | 9 FAILED_PRECONDITION | Indicates that the volume corresponding to the specified `volume_id` could not be expanded because it is currently published on a node but the plugin does not have ONLINE expansion capability. | Caller SHOULD ensure that volume is not published and retry with exponential back off. |
| Unsupported `capacity_range` | 11 OUT_OF_RANGE | Indicates that the capacity range is not allowed by the Plugin. More human-readable information MAY be provided in the gRPC `status.message` field. | Caller MUST fix the capacity range before retrying. |

##### NodeExpandVolume request and response
```protobuf
message NodeExpandVolumeRequest {
  // The ID of the volume. This field is REQUIRED.
  string volume_id = 1;

  // The path on which volume is available. This field is REQUIRED.
  // This field overrides the general CSI size limit.
  // SP SHOULD support the maximum path length allowed by the operating
  // system/filesystem, but, at a minimum, SP MUST accept a max path
  // length of at least 128 bytes.
  string volume_path = 2;

  // This allows CO to specify the capacity requirements of the volume
  // after expansion. If capacity_range is omitted then a plugin MAY
  // inspect the file system of the volume to determine the maximum
  // capacity to which the volume can be expanded. In such cases a
  // plugin MAY expand the volume to its maximum capacity.
  // This field is OPTIONAL.
  CapacityRange capacity_range = 3;

  // The path where the volume is staged, if the plugin has the
  // STAGE_UNSTAGE_VOLUME capability, otherwise empty.
  // If not empty, it MUST be an absolute path in the root
  // filesystem of the process serving this request.
  // This field is OPTIONAL.
  // This field overrides the general CSI size limit.
  // SP SHOULD support the maximum path length allowed by the operating
  // system/filesystem, but, at a minimum, SP MUST accept a max path
  // length of at least 128 bytes.
  string staging_target_path = 4;

  // Volume capability describing how the CO intends to use this volume.
  // This allows SP to determine if volume is being used as a block
  // device or mounted file system. For example - if volume is being
  // used as a block device the SP MAY choose to skip expanding the
  // filesystem in NodeExpandVolume implementation but still perform
  // rest of the housekeeping needed for expanding the volume. If
  // volume_capability is omitted the SP MAY determine
  // access_type from given volume_path for the volume and perform
  // node expansion. This is an OPTIONAL field.
  VolumeCapability volume_capability = 5;
}

message NodeExpandVolumeResponse {
  // The capacity of the volume in bytes. This field is OPTIONAL.
  int64 capacity_bytes = 1;
}
```

##### NodeExpandVolume Errors

| Condition             | gRPC code | Description           | Recovery Behavior                 |
|-----------------------|-----------|-----------------------|-----------------------------------|
| Exceeds capabilities | 3 INVALID_ARGUMENT | Indicates that CO has specified capabilities not supported by the volume. | Caller MAY verify volume capabilities by calling ValidateVolumeCapabilities and retry with matching capabilities. |
| Volume does not exist | 5 NOT FOUND | Indicates that a volume corresponding to the specified volume_id does not exist. | Caller MUST verify that the volume_id is correct and that the volume is accessible and has not been deleted before retrying with exponential back off. |
| Volume in use | 9 FAILED_PRECONDITION | Indicates that the volume corresponding to the specified `volume_id` could not be expanded because it is node-published or node-staged and the underlying filesystem does not support expansion of published or staged volumes. | Caller MUST NOT retry. |
| Unsupported capacity_range | 11 OUT_OF_RANGE | Indicates that the capacity range is not allowed by the Plugin. More human-readable information MAY be provided in the gRPC `status.message` field. | Caller MUST fix the capacity range before retrying. |

## Rationale

Another approach to support online and offline volume expansion is:
 - Implement only `ControllerExpandVolume`. We can use lvm command:
 ```
 lvextend --resizefs --size <requiredSize> LV
 ``` 
This command allow extend LV size and resize fs (using fsadm) at the same time. 

Currently there are no filesystems, which are support only offline expansion. But if we want to support for example 
btrfs filesystem (only online expansion), we need to omit --resizefs in `lvextend` because fsadm supports only ext2, ext3, ext4, ReiserFS and XFS. 
In case of btrfs we can run `btrfs filesystem resize`. But we need to know
mountpoint, because btrfs only support online grow. So if we are going to support another fs,
we need to use additional commands to extend filesystem in Controller and determine mountpoint. 

External-resizer sidecar tries to repeat volume expansion in case of any returned error, except for `volume_in_use` error. If status code is `Failed_Precondition`,
resizer considers this error as `volume_in_use`, so it doesn't repeat the call.
External-resizer also has inner storage of used PVC with Pods, so it can determine if PVC is in use by Pods. In this case 
user needs to restart Pod first, then tries to expand PVC again, then external-resizer repeats the call.
Also sidecar send the events about successful or failed expansion (including cause error) for particular PVC, so user can see it in 
PVC description. User can also see warning event about `volume_in_use` error in PVC.

CSI can also send alerts about volume expansion process similar to disk replacement or put annotations on Volume with expansion process status.

Also PV size is changed only after successful resizing, so operator can determine if volume expansion was successful either by according 
events or new PV size. 

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
3) Add additional statuses for volume - `resizing` and `resized`
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

