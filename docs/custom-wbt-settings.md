# Proposal: Support option to change WBT settings

Last updated: 07.12.2021


## Abstract

CSI should have an opportunity to automate managing of WBT (writeback throttling) value for specific SCs.

## Background
WBT feature was introduced in 4.10 kernel and have a [bug in some versions](https://access.redhat.com/solutions/4904681).
Storage Systems, which use Baremetal CSI, may have performance issues due to incorrect WBT settings.
Recommended workaround in disabling this feature.
So it is specific for each device (sdb, sdc, ...), CSI should have an opportunity to control it on Volume or Pod creation stage.

## Proposal

#### Operator part

Add ConfigMap to manage WBT settings in csi-baremetal-deployment helm chart.
CSI Node will check it every 60 seconds and update existing one.

```
data:
  config.yaml: |-
    enable: true
    # Value to set (uint32), 0 means disabling
    wbt_lat_usec_value: 0
    acceptable_volume_options:
    # Block volumes don't take any impact from WBT
      modes:
        - FS
      # It is risky to change WBT settings for LVG Volumes
      storage_types:
        - HDD
        - SSD
        - NVME

# The list of acceptable kernel versions
# Is some nodes are not in, user should add values manually via editting CM
  acceptable_kernels.yaml: |-
    node_kernel_versions:
      - 4.18.0-193.65.2.el8_2.x86_64
```

#### Node Part

- Node Stage ->
 1. Check WBT configuration
 2. Scan current value (`cat /sys/block/<drive>/queue/wbt_lat_usec`)
 3. Set passed one, if it's not equal (`echo <value> > /sys/block/<drive>/queue/wbt_lat_usec`)
 4. Add `wbt-changed=yes` annotation on Volume
 5. Send `WBTValueSetFailed` Event if error  

- Node UnStage ->
1. Check `wbt-changed=yes` annotation on Volume
2. Restore default (`echo "-1" > /sys/block/<drive>/queue/wbt_lat_usec`) (It was performed even if unmount/removeDir errors)
3. Delete annotation
4. Send `WBTValueRestoreFailed` Event if error

## Rationale

We could trigger WBT disabling via PVC annotation as for fake-attach feature. 

## Compatibility

WBT is supported only for 4.10 kernel version and above. Link - https://cateee.net/lkddb/web-lkddb/BLK_WBT.html

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Should we enable WBT disabling by default for specific kernel version? | We could describe the kernel version map, where it is necessary  |   |  The kernel version map is not clarified
ISSUE-2 | Is it correct to switch WBT settings on Stage/UnStage? | Other candidates - CreateVolume/DeleteVolume, Publish/UnPublish | RESOLVED  | In Create/DeleteVolume case settings won't be persisted on node reboot. When volume is shared across the pods you will have to manipulate WBT multiple times on PublishVolume
