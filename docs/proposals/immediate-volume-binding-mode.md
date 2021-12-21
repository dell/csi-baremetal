[This is a template for CSI-Baremetal's changes proposal. ]
# Proposal: Immediate Volume Binding Mode 

Last updated: 14-Dec-2021


## Abstract

CSI doesn't support `Immediate` volumeBindingMode. It relies on capacity reservation request issued from scheduler extender.

## Background

Some applications might want to use `Immediate` volumeBindingMode mode. However this is not recommended for topology-constrained
volumes since PersitentVolumes will be created without knowledge of the Pod's scheduling requirements.

## Proposal

CSI Controller must check `volumeBindingMode` on CreateVolume request. When mode is set to `Immediate` it should pick target
Node and AvailableCapacity.

## Rationale

It's not clear what is the use case for the `Immediate` volumeBindingMode. We shouldn't proceed with the implementation at the curremt moment.

## Compatibility

No issues with the compatibility - no new APIs and Kubernetes features are required to support this feature.

## Implementation

CSI Controller shouldn't check for AvailableCapacityReservation on CreateVolume request when volumeBindingMode is set to `Immediate`.

## Assumptions (if applicable)

ID | Name | Descriptions | Comments
---| -----| -------------| --------
ASSUM-1 |   |   |


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
