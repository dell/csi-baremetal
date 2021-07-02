# Proposal: Automation of node removal procedure

Last updated: 02.07.2021

## Abstract

Implementation of the option to delete CSI CRs from a node, which is going to be removed. 

## Background

User manual actions for node removal are described in [doc](https://github.com/dell/csi-baremetal/blob/master/docs/node-removal.md).
The goal of this proposal is automating most of the steps of that procedure.

## Proposal

#### User API

CSI - components on CSI side including CSI Operator

Operator - application controller (or other service, which watch resources)

Removal procedure:
1. Taint a node with:
- `node.dell.com/drain=drain:NoSchedule`
2. Wait until Operator replaced PVs and destroyed pods
3. Delete a node from kubernetes cluster
4. CSI deletes all corresponding CR resources automatically (including Csibmnode)

After removal the node can't be restored ad in [replacement](https://github.com/dell/csi-baremetal/blob/master/docs/proposals/node-replacement.md)! 


## Implementation

Set label:
1. CSI Operator must watch Nodes on taint changing events
2. Label the Csibmnode with `should-be-removed=yes` if the Node has taint `node.dell.com/drain=drain:NoSchedule`

Unset label:
1. CSI Operator must watch Nodes on taint changing events
2. Check, if the Csibmnode has `should-be-removed=yes` label and the Node with the same ID has no `node.dell.com/drain=drain:NoSchedule` taint
3. Delete `should-be-removed=yes` label

Removal:
1. CSI Operator must watch Nodes on deleting events
2. Start removal procedure, if if the Csibmnode has `should-be-removed=yes` label and the Node with the same ID has been deleted from kubernetes cluster
3. Delete drives, ACs
4. Patch and delete Volumes, Lvgs, Csibmnode 

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
ISSUE-1 | Should we do node removal procedure in node-controller? | Node-controller reconciles Csibmnodes and nodes already | Open | 
ISSUE-2 | Should we delete related PVs and PVCs? |  | Open  | 
ISSUE-3 | We don't provide any procedure to remove a node without deleting it from cluster |  | Open | 
