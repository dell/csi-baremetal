# Proposal: Fake Attach Volume

Last updated: June 11 2011


## Abstract

Fake Attach Volume feature should allow Pod to come up after a restart even when a volume is unhealthy.

## Background

Some storage and streaming systems consolidate access to the persistent volumes by mounting them to a single Pod.
Thus single volume failure might prevent service to run and cause data unavailability on remaining.
 
## Proposal

If a volume is inaccessible when a Pod is being restarted, the BM CSI will fake the attach temporary volume in
read-only mode.

## Rationale

Alternative approach might be implementing custom logic in Pod Controller to delete PVC from the requirements on mount
failure.

## Compatibility

This feature will require application Operator to put specific annotation on PVC
`pv.attach.kubernetes.io/ignore-if-inaccessible: yes`

## Implementation

When `pv.attach.kubernetes.io/ignore-if-inaccessible: yes` annotation is set CSI must ignore NodeStage errors and invoke
Fake Attach logic by mounting tmpfs volume in read-only mode and generating event to notify user about this.

On NodeUnstage tmpfs volume must be deleted.

## Assumptions (if applicable)

ID | Name | Descriptions | Comments
---| -----| -------------| --------
ASSUM-1 |   |   |


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
