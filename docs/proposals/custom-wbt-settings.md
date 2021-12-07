# Proposal: Support option to change WBT settings

Last updated: 07.12.2021


## Abstract

CSI should have an opportunity to automate WBT (writeback throttling) disabling for specific Volumes.

## Background

Some Storage Systems, which use Baremetal CSI, may have performance issues due to incorrect WBT settings.
So it is specific for each device (sdb, sdc, ...), CSI should have an opportunity to control it on Volume or Pod creation stage.

## Proposal

#### Operator part

Add parameters to csi-baremetal-deployment helm chart

- `WBTDisableOnHDDSC` -> set `WBTDisable=on` for `csi-baremetal-sc-hdd`
- `WBTDisableOnSSDSC` -> set `WBTDisable=on` for `csi-baremetal-sc-ssd`
- `WBTDisableOnNVMESC` -> set `WBTDisable=on` for `csi-baremetal-sc-nvme`

#### Node Part

- Node Stage ->
 1. Check SC Parameter for `WBTDisable=on`
 2. Scan current value (`cat /sys/block/<drive>/queue/wbt_lat_usec`)
 3. Set "0", if it's not equal (`echo "0" > /sys/block/<drive>/queue/wbt_lat_usec`)

- Node UnStage ->
1. Check SC Parameter for `WBTDisable=on`
2. Scan current value (`cat /sys/block/<drive>/queue/wbt_lat_usec`)
3. Restore default, if it's "0" (`echo "-1" > /sys/block/<drive>/queue/wbt_lat_usec`)

## Rationale

We could trigger WBT disabling via PVC annotation as for fake-attach feature. 

## Compatibility

WBT is supported only for 4.10 kernel version and above. Link - https://cateee.net/lkddb/web-lkddb/BLK_WBT.html

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Should enable WBT disabling by default for specific kernel version? | We could describe kernel version map, where it is enable  |   |  Kernel version map is not clarify
ISSUE-1 | Is it correct to switch WBT settings on Stage/UnStage | Other candidates - CreateVolume/DeleteVolume, Publish/UnPublish |   | 
