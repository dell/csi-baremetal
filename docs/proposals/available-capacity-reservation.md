# Proposal: Available Capacity Reservation improvements

Last updated: 3/30/2021


## Abstract

Proposal to improve Available Capacity Reservation by:
* Getting rid of race conditions and stuck reservations.
* Suggesting uniform reservation algorithm for Scheduler and Scheduler Extender.

## Background

To guarantee placement decisions CSI reserves storage space on nodes with help of [AvailableCapacityReservation](https://github.com/dell/csi-baremetal/blob/master/charts/csi-baremetal-driver/crds/csi-baremetal.dell.com_availablecapacityreservations.yaml) CRD.
Reservation is created in Scheduler Extender and removed in Controller when corresponding Volume provisioning started.

Since AvailableCapacityReservation or simply ACR doesn't have reference to the PVC name several unexpected behaviors observed:
* ACR is used for another PVC with similar storage requirements (non critical)
* ACR is created twice for the same PVC and never removed when volume provision failed for some reason (for example, defective disk). This prevents from further allocations for reserved storage space (critical)

## Proposal

To have a clear reference between Pod PVCs and ACR suggest to:
* Set ACR name to **namespace** + **pod name**
* Combine all storage allocation requests for a pod to a single ACR CR
* Add PVC name and namespace fields in ACR for usual volumes
* Add volume name and namespace fields in ACR for inline volumes

Additional improvements:
* Add list of the nodes received from Filter stage to the ACR to avoid redundant reservations and optimize work for Scheduler on Reserve stage.

To avoid race conditions keep reservation logic in Controller:
* Scheduler Extender volume provision flow
  * Extender creates ACR CR on Filter stage with status _REQUESTED_
  * Controller reconciles ACR and puts ACs to the list, changes status to _RESERVED_ or _REJECTED_ when no AC found
* Scheduler volume provision flow
  * Scheduler creates ACR CR on Reserve stage with status _REQUESTED_
  * Controller reconciles ACR and puts ACs to the list, changes status to _RESERVED_ or _REJECTED_ when no AC found
  * Scheduler can cancel reservation on Unreserve stage by setting status to _CANCELED_

## Rationale

Reservation logic through CRD has some performance penalties but guarantees placement.

## Compatibility

No compatibility with the previous versions.

## Implementation

1. Add new fields to ACR CRD
2. Implement logic in Controller
3. Refactor Scheduler Extender logic to use new algorithm
    - For usual volumes
    - For inline volumes
4. Add support for inline volumes in Node
5. Future - implement logic in Scheduler


## Assumptions (if applicable)

ID | Name | Descriptions | Comments
---| -----| -------------| --------
ASSUM-1 | Performance | Deployment time shouldn't increase significantly | Need to collect benchmarks


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Inline volumes handling | Closed | Will node be able to obtain pod name on NodePublish stage? | Volume name can be used instead
ISSUE-2 | Scoring | Closed | Scores in ACR | Will handle scoring separately
