# Proposal: Automation of node removal procedure

Last updated: 24.06.2021

## Abstract

Implementation of the option to delete CSI CRs from a node, which is going to be removed. 

## Background

User manual actions for node removal are described in [doc](https://github.com/dell/csi-baremetal/blob/master/docs/node-removal.md).
The goal of this proposal is automating most of the steps of that procedure.

## Proposal

#### User API

CSI CRs deletion(Volumes, AC, ACR, Drives, LVGs, Csibmnode) may be triggerred on taint a Node:
- `node.dell.com/drain=drain:NoSchedule` - removal (node should be deleted from the cluster)
- `node.dell.com/drain=planned-downtime:NoSchedule` - maintenance (node may be restored with the same hostname/ip)

## Implementation

#### Removal

1. CSI Operator must reconcile Nodes and check `node.dell.com/drain=drain:NoSchedule` taint
2. Label Csibmnode with `node-should-be-removed`
3. Check that there are no PV for the Node (reconcile Csibmnode with `node-should-be-removed`) [ISSUE-1]
4. If the Node has been deleted, go forward 
5. Delete `platform` label from Node (skip if 4)
6. Wait until `csi-baremetal-node` pod is destroyed (to not restore Drive, AC)
8. Delete all remaining CRs(Drives, ACs, ACRs, Csibmnode)
9. Check stacked Volumes and LVGs, patch finalizers and delete CRs [ISSUE-2]

#### Maintenance

Disable node:
1. CSI Operator must reconcile Nodes and check `node.dell.com/drain=planned-downtime:NoSchedule` taint
2. Label Csibmnode with `node-disabled`
3. Check that there are no PV for the Node (with nodeID)
4. Delete `platform` label from Node
5. Wait until `csi-baremetal-node` pod is destroyed (to not restore Drive, AC)
6. Delete all remaining CRs(Drives, ACs, ACRs) excluding Csibmnode
7. Check stacked Volumes and LVGs, patch finalizers and delete CRs

Restore:
1. Start restore if Csibmnode has `node-disabled` label, but k8sNode has no `node.dell.com/drain=planned-downtime:NoSchedule` taint
2. Return `platform` label on Node
3. Delete `node-disabled` label

## Rationale

#### Alternative approach 1

Perform CSI CRs deletion in CSI Node-controller. Trigger will be deleting the corresponding `csibmnode` CR.

#### Alternative approach 2

Delete CRs (Drive, LVG) in `Reconcile` method in appropriate controller if node has no `uuid label` or has `node removal` label/annotation

Problem - if node was removed from cluster before and csi-baremetal-node pod is unavailable, CSI resources will be stacked as currently

## Compatibility

This proposal is valid for CSI deployed with CSI Operator

## Open issues (same as [here](https://github.com/dell/csi-baremetal/blob/master/docs/node-removal.md))

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | How can we be sure that all related pods and PVs are replaced? | Is deleting of all corresponding PVs enough? | Open |
ISSUE-2 | What CSI Operator should do if removal-taint was deleted? | After `related PVs == 0` and CSI has deleted all resources we can't restore. | Open  |
