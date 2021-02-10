# Proposal: Caching support

Last updated: 10.02.2021

## Proposal

We should cache some information to decrease API call count.
This will helps us to achieve required scalability and performance.

### Kubernetes recourses used by CSI driver

- AvailableCapacity (AC)
- AvailableCapacityReservation (ACR)
- Drive
- LVG
- Volume
- Pod
- Node
- PersistentVolumeClaim
- StorageClass

### CSI Driver components

- csi-controller
- csi-node
- scheduler-extender


### Current resources usage by CSI components

#### scheduler-extender

| Resource                     | Get | List | Create | Update | Delete | Safe to Cache                                                 |
| ---------------------------- | --- | ---- | ------ | ------ | ------ | ------------------------------------------------------------- |
| AvailableCapacity            | -   | +    | -      | -      | -      | NO. Race condition possible with csi-controller               |
| AvailableCapacityReservation | -   | +    | +      | -      | -      | NO. Race condition possible with csi-controller               |
| PersistentVolumeClaim        | +   | -    | -      | -      | -      | YES. Cache updates should happen in the background with WATCH API |
| Volume                       | -   | +    | -      | -      | -      | YES. Cache updates should happen in the background with WATCH API |
| StorageClass                 | -   | +    | -      | -      | -      | YES. Cache updates should happen in the background with WATCH API |

Caching in scheduler extender will not impact scalability a lot, but it can improve extender performance without breaking data consistency.


#### csi-controller
| Resource                     | Get | List | Create | Update | Delete | Safe to Cache                                                     |
| ---------------------------- | --- | ---- | ------ | ------ | ------ | ----------------------------------------------------------------- |
| Pod                          | -   | +    | -      | -      | -      | YES. Cache updates should happen in the background with WATCH API |
| Node                         | -   | +    | -      | -      | -      | YES. Cache updates should happen in the background with WATCH API |
| Drive                        | -   | +    | -      | +      | -      | NO. No need.                                                      |
| AvailableCapacity            | -   | +    | +      | +      | +      | NO. Race condition possible with scheduler-extender               |
| AvailableCapacityReservation | -   | +    | -      | +      | +      | NO. Race condition possible with scheduler-extender               |
| Volume                       | +   | +    | +      | +      | +      | NO. Race condition possible with csi-node                         |
| LVG                          | -   | -    | +      | -      | -      | NO. No need.                                                      |

In the csi-controller, only a few resources can be safely cached. Caching will not impact performance or scalability a lot.
Most API calls that csi-controller do in the idle state can be removed if add auto-updated cache (based on Watch API).

#### csi-node
| Resource          | Get | List | Create | Update | Delete | Safe to Cache                                                                                |
| ----------------- | --- | ---- | ------ | ------ | ------ | -------------------------------------------------------------------------------------------- |
| AvailableCapacity | -   | +    | +      | +      | +      | YES. Safe to cache when used in Discovery loop, but not when used for inline volume creation |
| Volume            | +   | +    | +      | +      | +      | NO. Race condition possible with csi-controller. Caching can be used in node.Discover loop   |
| LVG               | +   | +    | +      | +      | -      | NO. Race condition possible with csi-controller.                                             |
| Drive             | -   | +    | +      | +      | -      | YES.                                                                                         |

The main goal is to decrease API call count during the periodic drive discovery loop in csi-node. This change will have the most impact on the driver scalability.

## Compatibility

Implementation should not change or break public APIs

## Implementation

*Tasks sorted by priority*
1. Write own generic cached client for kube-api or reuse existing from controller-runtime
2. Add Volume, Dive, and AvailableCapacity CRs caching to Discovery loop in csi-node.
3. Add caching for PersistentVolumeClaim and Volume and StorageClass CRs in scheduler-extender
4. Add POD and Node resources caching in csi-controller.