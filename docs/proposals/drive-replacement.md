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

## Proposal
To perform replacement of failed drive negotiation between CSI, Operator and User is required:

- BM CSI
  1. Detects drive health
  2.  Triggers replacement process
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