# Drive replacement procedure
<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
- [Proposal](#proposal)  
- [Design Details](#design-details)  
<!-- /toc -->
## Summary
Disk replacement is a feature which allows user to physically replace defective drive.

## Motivation
Once specific disk goes offline CSI driver should provide a way to gracefully detach it from the system and insert new healthy device instead.

Single drive can be used to store one or more persistent volumes. Persistent volumes can be owned by different applications which, in turn, can be managed by different Operators.

### Goals
- Develop algorithm for reactive and proactive drive replacement.
- Come up with the API between CSI, Operator and User.

## Proposal
To perform replacement of failed drive negotiation between CSI, Operator and User is required:

- BM CSI
  1. Detects drive health
  2. Triggers replacement process
  3. Sends alerts
  4. Prepares drive for physical replacement
- Operator
  1. Subscribes for procedure support by setting corresponding PVC annotation.
  2. Instantiates recovery process and monitor its progress if needed
  3. Might reject procedure if system cannot release corresponding volume(s)
  4. Deletes persistent volumes
- User
  1. Triggers physical drive replacement process since drive might not be shipped yet when recovery of data is completed
## Design Details
### Drive health detection
Drive health is detected by drive manager and stored in the health field of [Drives CRD](https://github.com/dell/csi-baremetal/blob/master/charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_drives.yaml): 
- `GOOD` - drive is healthy. Application can safely use it.
- `SUSPECT` - drive might not be healthy. Replacement is recommended.
- `BAD` - drive is not healthy. Replacement is required.
- `UNKNOWN` - drive health not detected. Application should rely on IO errors.
### Drive operational statuses
[Drives CRD](https://github.com/dell/csi-baremetal/blob/master/charts/baremetal-csi-plugin/crds/baremetal-csi.dellemc.com_drives.yaml) will be extended by the new field - operational status: 
- `OPERATIVE` - drive in use
- `RELEASING` - releasing in progress
- `RELEASED` - drive ready for removal
- `FAILED` - drive failed to remove
- `REMOVING` - removing in progress (PVs deletion, Safe remove, LED locate)
- `REMOVED` - drive removed
### API
#### Operator
Negotiation between CSI and Operator is optional and will be done though annotation on different objects.
* Storage class annotation:
  - `volumerelease.csi-baremetal/support: yes` - inform CSI that volume release feature is supported by Operator
* [Volume CRD]() annotations:
  - `volumerelease.csi-baremetal/process: start|pause|stop` inform to start|pause|stop volume release process
  - `volumerelease.csi-baremetal/release: processing` system is working on data recovery/graceful IO shutdown
#### CSI
#### User
## Test plans