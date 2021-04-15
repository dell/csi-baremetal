# Proposal: Refactoring discover of AvailableCapacity and volume presence on disks 

Last updated: 14.04.2021

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

To avoid possible race conditions with `AvailableCapacity` manipulations in `Node` and `Scheduler-Extender` there are 3 approach:
1) `Using the Reconcile loop` 
   A reconcile loop for AC is created in the controller. When change request is received from the AC, 
   the controller checks for the presence of volumes, LVG, and drive and adjusts the AC size if it does not match the values. 
   For AC, we introduce a finalizer, which does not allow removing AC in case of a good disk.
2) `Moving the drive controller to the controller`
   Drive reconcile loop is create in CSI controller side, not in the Node side.
   If a request comes in drive reconcile that drive is not clean, controller checks if there is LVG for it and update 
   the corresponding ACs. If there is a request that Drive is bad/suspect, then we update the corresponding AC for drive and LVG.
3) `Using gRPC/HTTP server (preferable)`
   A gRPC/HTTP server is added to the CSI controller that handles get/create/update/delete requests for the AC. 
   `Node` and `Extender` send requests to this server.

## Rationale

1) The first 2 approaches are easy to implement and doesn't affect performance of CSI highly. But it is unlikely that they
will help to completely get rid of the race condition. 
2) The last approach more likely will help to solve race condition problem. But it may lead to performance issues. If 
   the request is not processed for a long time and the node has to retry it, then this can seriously affect the performance.
   It also requires to perform scalability and stress tests for this case. So this approach need considerations about optimization.


## Compatibility

There is no problem with compatibility

## Implementation

#### Volume Manager
1) Remove `discoverAvailableCapacity` function.
2) Remove logic of creating volume in `discoverVolumeCRs` and add logic of changing Drive field `IsClean`.
3) Add discover of FS, LVM and partitions with such functions: `wipes`, `lvs`, `lsblk`.

#### Drive controller
1) Add `IsClean` field handling in reconcile loop.

#### CSI controller
1) Add gRPC/HTTP server in `Controller` with get/delete/update/create procedure.
2) Add structure, which calls Kubernetes API for get/delete/update/create `AvailableCapacity` using read-write mutex with key.
   Key is a name of `AvailableCapacity`.
3) Use this structure in `server`, `volume_operations`, `state_monitor` to manipulate `AvailableCapacity`.

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |  Performance issue in server | Using one server for all components can lead to performance problem   | Open  | May be solved by adding limitation of requests, timeouts and workers pool.  
