# Proposal: Refactoring discover of AvailableCapacity and volume presence on disks 

Last updated: 16.04.2021

## Abstract

This proposal contains approaches for discovering of FS, LVG and partitions on drives and
available capacity manipulations in CSI controller

## Background

Currently, we discover volumes on drive during `discoverVolumeCRs` function in volume manager and create `AvailableCapacity` 
according to this information. Need to detect fs, lvm, partitions presence on disk, avoid creating new volume and move logic
with AC manipulation to CSI controller to avoid possible race conditions.

## Proposal

There are 3 possible approach to solve problem above. 

#### Common part for all approached

The field `IsClean` is entered for Drive CR. In the `Volume manager`, we check if the drive has lv, fs, partitions, etc. 
If the disk is dirty, we update the field `IsClean`. `Drive Controller` updates the corresponding AC in Reconcile loop . 
If the disk is clean, then drive controller tries to create a new `AvailableCapacity`, if it does not exist.
For the system disk, AC is created with size 0. So for this case `discoverVolumeCRs` doesn't create new volumes, it just updates
drives fields. `discoverAvailableCapacity` function in volume manager is removed.

#### AvailableCapacity manipulation
Add second Drive Controller in CSI Controller, which handles changes in Drive field `IsClean` and `Health`,
so it updates according `AvailableCapacity`. It also makes changes in `LVG` AvailableCapacity

## Compatibility

There is no problem with compatibility

## Implementation

#### Volume Manager
1) Remove `discoverAvailableCapacity` function.
2) Remove logic of creating volume in `discoverVolumeCRs` and add logic of changing Drive field `IsClean`.
3) Add discover of FS, LVM and partitions with such functions: `wipes`, `lvs`, `lsblk`.

#### Drive CR
1) Add `IsClean` field.

#### CSI controller
1) Add Drive controller in CSI controller.
2) Add `IsClean` field handling in reconcile loop. Controller update according AvailableCapacity with size 0.
If drive is system, controller tries to update LVG AvailableCapacity.
3) Add `Health` field handling in reconcile loop. If `Health` is not good, controller deletes AvailableCapacity and updates LVG AvailableCapacity.

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | How does Kubernetes handle 2 reconcile loop for one resource |  | Open |
