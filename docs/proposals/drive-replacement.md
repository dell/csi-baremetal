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

* BM CSI
  - Detects drive health
  - Triggers replacement process
  - Sends alerts
  - Prepares drive for physical replacement
* Operator
  - Subscribes for procedure support by setting corresponding PVC annotation.
  - Instantiates recovery process and monitor its progress if needed
  - Might reject procedure if system cannot release corresponding volume(s)
  - Deletes persistent volumes
* User
  - Triggers physical drive replacement process since drive might not be shipped yet when recovery of data is completed
## Design Details
### Drive health detection
Drive health is detected by drive manager and stored in the health field of [Drives CRD](https://github.com/dell/csi-baremetal/blob/master/charts/csi-baremetal-driver/crds/csi-baremetal.dell.com_drives.yaml): 
- `GOOD` - drive is healthy. Application can safely use it.
- `SUSPECT` - drive might not be healthy. Replacement is recommended.
- `BAD` - drive is not healthy. Replacement is required.
- `UNKNOWN` - drive health not detected. Application should rely on IO errors.
### Drive operational statuses
[Drives CRD](https://github.com/dell/csi-baremetal/blob/master/charts/csi-baremetal-driver/crds/csi-baremetal.dell.com_drives.yaml) will be extended by the new field - operational status: 
- `OPERATIVE` - drive in use
- `RELEASING` - releasing in progress
- `RELEASED` - drive ready for removal
- `FAILED` - drive failed to remove
- `REMOVING` - removing in progress (PVs deletion, Safe remove, LED locate)
- `REMOVED` - drive removed
### API
#### Operator
Negotiation between CSI and Operator is optional and will be done though annotation on different objects.
* Persistent volume claim annotation:
  - `volumerelease.csi-baremetal/support: yes` - inform CSI that volume release feature is supported by Operator
* [Volume CRD](https://github.com/dell/csi-baremetal/blob/master/charts/csi-baremetal-driver/crds/csi-baremetal.dell.com_volumes.yaml) annotations:
  - `volumehealth.csi-baremetal/health: good/unknown/suspect/bad` - health of the underlying drive(s) 
  - `volumerelease.csi-baremetal/process: start|pause|stop` - inform to start|pause|stop volume release process
  - `volumerelease.csi-baremetal/release: processing` - system is working on data recovery/graceful IO shutdown
  - `volumerelease.csi-baremetal/release: completed` - volume is released
  - `volumerelease.csi-baremetal/release: failed` - system failed to release volume
  - `volumerelease.csi-baremetal/recovery: [0:100]` - percent of recovery progress
  - `volumerelease.csi-baremetal/status: <status description>` - extra information. can be used to provide description for the release process. For example, *release=failed, recovery=50, status="recovery failed by timeout"*
#### User
To trigger physical drive replacement user must put the following annotation on the corresponding Drive custom resource:
  - `driveremove.csi-baremetal/replacement: ready` - informs that drive replacement is ready

*User can trigger drive releasing himself. If user annotates driveCR with `health=<SUSPECT/BAD>`, drive health will be overridden with passed value.*

*DR procedure can be repeated if drive Usage is `FAILED` or `RELEASED`. User should annotate driveCR with `replacement=restart` and drive-controller will switch Usage value to IN_USE. After restart the annotation will be deleted.* 
### Detailed workflow
* When drive health changed from `GOOD` to `SUSPECT` or `BAD` CSI will:
  - Set drive operational status to `RELEASING`
  - Put `volumehealth.csi-baremetal/health: suspect/bad` and `releasevolume.process: start` annotations on corresponding volumes custom resources.    
* If `volumerelease.csi-baremetal/support: yes` annotation is set for corresponding PVC(s) CSI waits for recovery completion
  - During recovery being in progress Operator can expose recovery status via `volumerelease.csi-baremetal/recovery: %` annotation on volume CR(s)
* Once recovery completed Operator must put `volumerelease.csi-baremetal/release: completed` annotation on volume CR(s)
  - CSI sets drive operational status to `RELEASED`
* If recovery failed due to some reason Operator must put `volumerelease.csi-baremetal/release: failed` annotation on volume CR(s)
  - In addition Operator can provide additional information about error `volumerelease.csi-baremetal/status: <status description>`
  - CSI sets drive operational status to `FAILED`
* When drive operation status is `RELEASED` user can initiate physical drive replacement by setting `driveremove.csi-baremetal/replacement: ready` annotation on drive CR
  - CSI sets drive operational status to `REMOVING`
  - Operator deletes PV(s)
  - CSI prepares drive for safe removal and starts LED locate
  - CSI sets drive operational status to `REMOVED` if all operations are passed successfully and `FAILED` otherwise
* When drive operation status is `REMOVED`
  - User does physical replacement
## Test plans
Drive replacement workflow must be covered by E2E tests in CI.
