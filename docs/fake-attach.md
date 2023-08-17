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
- put `fake-attach: yes` annotation on CSI Volume CR if there is stageVolume error and annotation wasn't set before, i.e. the volume turns from healthy to unhealthy
- if there is stageVolume error on block-mode volume without `fake-device: <device path>` annotation, try to setup a fake loop device mapped to regular file, add `fake-device: <device path>` annotation to volume, and then try to mount this fake device to stagingTargetPath as normal stage volume
- if there is stageVolume error on block-mode volume with `fake-device: <device path>` annotation, check whether <device path> is really the path of fake device of this volume first. if the check passed, also try to mount this fake device to stagingTargetPath as normal stage volume
- delete `fake-attach: yes` annotation on CSI Volume CR if there is no stageVolume error and annotation was set before, i.e. the volume turns from unhealthy back to healthy
- In this scenario, for block-mode volume with `fake-device: <device path>` annotation, if <device path> is really the path of fake device of this volume, then try to clean this fake device. By the way, the removal of block-mode volume with annotations `fake-attach: yes` and `fake-device: <device path>` will also trigger the clean of fake device.

Event FakeAttachInvolved generated when `fake-attach: yes` annotation is setting.

Event FakeAttachCleared generated when `fake-attach: yes` annotation is deleting.

### NodePublishVolume 
CSI must check `fake-attach` annotation and mount tmpfs volume in read-write mode for fs-mode volume

Command to mount tmpfs volume: `mount -t tmpfs -o size=1M,rw <volumeID> <destination folder>`

rw option is used as workaround for issue-906 (OpenShift 4.8)

### NodeUnpublishVolume 
tmpfs volume must be unmounted usually for fs-mode volume.

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
