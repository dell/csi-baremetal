# Proposal: Automation of node removal procedure

Last updated: 24.06.2021

## Abstract

Implementation of the option to delete CSI CRs from a node, which going to be removed. 

## Background

User manual actions for node removal are described in [doc](https://github.com/dell/csi-baremetal/blob/master/docs/node-removal.md).
The goal of this proposal is automating most of the steps of that procedure. 

## Proposal

#### User API

CSI CRs deletion(Volumes, AC, ACR, Drives, LVGs, Csibmnode) may be triggerred on annotating a Node. The annotation has 2 options:
- `csi-baremetal/node-removal-request=yes` - CSI waits until there are no PVC associated with Node
- `csi-baremetal/node-removal-request=force` - CSI deletes all PVC

CSI puts `csi-baremetal/node-removal-process` label on Node, which has to be removed. Statuses:
- `wait` - wait until all related PVCs will be deleted
- `in-progress` - CSI is deleting Custom Resources
- `completed` - removal is finished with success
- `failed` - removal is finished with failure

*Failed status can be set if CSI is not in Running state completely. For example `node-controller` is not ready and can't delete Csibmnode CR in the normal way.*

Updated node removal steps performed by user:
- for safe mode
1. Delete all related Pods and PVCs
2. Annotate corresponding Node
3. Wait `completed` (or `failed`) status
4. Disable Node
- for force mode
1. Annotate corresponding Node with `force`
2. Wait `completed` (or `failed`) status
3. Disable Node

*For node replacement procedure user should edit csibmnode CR and not annotate Node.*  

## Rationale

#### Alternative approach 1

Perform CSI CRs deletion in CSI Node-controller. Trigger will be deleting the corresponding `csibmnode` CR.

#### Alternative approach 2

Delete CRs (Drive, LVG) in `Reconcile` method in appropriate controller if node has no `uuid label` or has `node removal` label/annotation

Problem - if node was removed from cluster before and csi-baremetal-node pod is unavailable, CSI resources will be stacked as currently

## Compatibility

This proposal is valid for CSI deployed with CSI Operator

## Implementation

Algorithm for CSI Operator:
1. CSI Operator must reconcile Nodes and check `csi-baremetal/node-removal-request` annotation
2. Put `csi-baremetal/node-removal-process=wait` label after `safe` and check related PVCs
3. Delete PVSs after `force`
4. Put `csi-baremetal/node-removal-process=in-progress` label
5. Delete CRs with finalizers (Volumes, LVGs)
6. Delete `platform` label from Node
7. Wait until `csi-baremetal-node` pod is destroyed (to not restore Drive, AC)
8. Delete all remaining CRs(Drives, ACs, ACRs, Csibmnode)
9. Check stacked Volumes and LVGs, patch finalizers and delete CRs
10. Put `csi-baremetal/node-removal-process=completed`

## Open issues (same as [here](https://github.com/dell/csi-baremetal/blob/master/docs/node-removal.md))

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Should user annotate Csibmnode instead of k8sNode | If node was removed from cluster before annotating, there is no opportunity for user to clean CSI Resources in automated way. | Open |
ISSUE-2 | How can CSI Operator wait `related PVCs == 0` condition?  | Should CSI Operator reconcile PVCs? | Open  |
ISSUE-3 | Should we mark drives as OFFLINE instead of removing?  |  | Open  |
ISSUE-4 | Should we marked volumes as INOPERATIVE instead of removing?  |  | Open  |   
