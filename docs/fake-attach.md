# Proposal: Fake Attach Volume

Last updated: July 1 2021


## Abstract

Fake Attach Volume feature should allow Pod to come up after a restart even when a volume is unhealthy.

## Background

Some storage and streaming systems consolidate access to the persistent volumes by mounting them to a single Pod.
Thus single volume failure might prevent service to run and cause data unavailability on remaining.
 
## Proposal

If a volume is inaccessible when a Pod is being restarted, the BM CSI will fake the attach temporary volume in
read-write mode.

## Rationale

Alternative approach might be implementing custom logic in Pod Controller to delete PVC from the requirements on mount
failure.

## Compatibility

This feature will require application Operator to put specific annotation on PVC
`pv.attach.kubernetes.io/ignore-if-inaccessible: yes`

If fake-attach volume is successfully created, CSI Volume CR will be annotated with `fake-attach:yes` and `Operational Status` will equal to `MISSING`. The pod must be restarted after `Operational Status` returns to `OPERATIVE`.


## Implementation

### NodeStageVolume
When `pv.attach.kubernetes.io/ignore-if-inaccessible: yes` annotation is set CSI must:
- ignore NodeStageVolume errors
- put `fake-attach: yes` annotation on CSI Volume CR if error is and annotation wasn't set before
- delete `fake-attach: yes` annotation on CSI Volume CR if error is not and annotation was set before

Event FakeAttachInvolved generated when `fake-attach: yes` annotation is setting.

Event FakeAttachCleared generated when `fake-attach: yes` annotation is deleting.

### NodePublishVolume 
CSI must check `fake-attach` annotation and mount tmpfs volume in read-write mode.

Command to mount tmpfs volume: `mount -t tmpfs -o size=1M,rw <volumeID> <destination folder>`

rw option is used as workaround for issue-906 (OpenShift 4.8)

### NodeUnpublishVolume 
tmpfs volume must be unmounted usually.

### NodeUnstageVolume 
Do nothing.


## Assumptions (if applicable)

ID | Name | Descriptions | Comments
---| -----| -------------| --------
ASSUM-1 |   |   |


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
