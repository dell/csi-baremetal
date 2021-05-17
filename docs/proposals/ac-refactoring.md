# Proposal: Refactoring discover of AvailableCapacity and volume presence on disks 

Last updated: 16.04.2021

## Abstract

This proposal contains approaches for discovering of FS, LVG and partitions on drives and
available capacity manipulations in CSI controller

## Background

Currently, we discover volumes on drive during `discoverVolumeCRs` function in volume manager and create `AvailableCapacity` 
according to this information. Need to detect fs, lvm, partitions presence on disk, avoid creating new volume and
allow CSI Controller exclusively manage AC CRD to avoid possible race conditions between Controller and Node.

## Proposal

The field `IsClean` is entered for Drive CR. In the `Volume manager`, we check if the drive has lv, fs, partitions, etc. 
If the disk is dirty, we update the field `IsClean`. `Capacity Controller` updates the corresponding AC in Reconcile loop .
If the disk is clean, then drive controller tries to create a new `AvailableCapacity`, if it does not exist.
For the system disk, AC is created with size 0. So for this case `discoverVolumeCRs` doesn't create new volumes, it just updates
drives fields. `discoverAvailableCapacity` function in volume manager is removed.
#### AvailableCapacity manipulation
Add second Capacity Controller in CSI Controller, which handles changes in Drive field `IsClean` and `Health`,
so it updates according `AvailableCapacity`. It also reconcile `LVG`. It sets AC size to 0, if LVG is bad or failed. It also creates or
updates AC for system LVGs, as they were discovered in Node during Discover function.

## Compatibility

There is no problem with compatibility

## Implementation

#### Volume Manager
1) Remove `discoverAvailableCapacity` function.
2) Remove logic of creating volume in `discoverVolumeCRs` and add logic of changing Drive field `IsClean`.
3) Add discover of FS, LVM and partitions with such functions: `fdisk`, `pvs`, `lsblk`.
4) Move logic to create and update system LVG AC in Capacity Controller.

#### Drive CR
1) Add `IsClean` field.

#### CSI controller
1) Add Capacity controller in CSI controller.
2) Add `IsClean` field handling in reconcile loop. Controller update according AvailableCapacity with size 0.
3) Add `Health` field handling in reconcile loop. If `Health` is not good, controller deletes AvailableCapacity and
updates LVG AvailableCapacity.
4) Add reconciliation of LVG health and status. If health is Bad/Suspect or status if failed, Controller reset AC size.
5) Add handling system LVG creation. If system LVG is created/updated, Controller creates/updates according AC.
