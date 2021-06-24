# Proposal: Automation of node removal procedure

Last updated: 24.06.2021

## Abstract

Implementation of the option to delete CSI CRs from a node, which going to be removed. 

## Background

User manual actions for node removal are described in [doc](https://github.com/dell/csi-baremetal/blob/master/docs/node-removal.md). The goal of this POC is automating most of the steps of that procedure. 

## Proposal

CSI CRs deletion(Volumes, AC, ACR, Drives, LVGs) will be triggerred on deletion of appropriate csibmnode and we be performed by node-controller.
Before implementation, we need to replace node-uuid annotation with label and add node-affinity `uuid label is exists` to csi-baremetal-node daemonset.

Updated node removal steps performed by user:
1. Get node uuid mapped to node name
2. Delete all related Pods and PVCs
3. Delete csibmnode CR with same node uuid

Node removal steps performed by node-controller:
1. Check csibmnode deletion timestamp and start node removal if needed
2. Delete Volume CRs and LVGs (patch finalizers if stacked)
3. Remove uuid label from node
4. Wait until csi-baremetal-node pod will be destroyed (to not restore Drive, LVG and AC CRs)
5. Delete Drive, LVG and AC CRs

The approach may work if k8sNode is a part of the cluster yet or if it is deleted (skip 3,4 steps).
Csibmnode deletion may be performed in CSI Operator or other Operator and triggerred after taint, label or annotate node with specific value.

## Rationale

#### Alternative approach 1

Perform CSI CRs deletion in CSI Operator. Trigger will be taint `NoShedule` on Node. Node removal steps will be the same. In 3 Operator must delete node-platform label.

Problem - if node was removed from cluster before taint with `NoShedule`, there is no opportunity for user to clean CSI Resources in automated way.

#### Alternative approach 2

Delete CRs (Drive, LVG) in `Reconcile` method in appropriate controller if node has no `uuid label` or has `node removal` label/annotation

Problem - if node was removed from cluster before and csi-baremetal-node pod is unavailable, CSI resources will be stacked as currently

## Compatibility

It has no issues with compatibility

## Implementation

TBD

## Open issues (same as [here](https://github.com/dell/csi-baremetal/blob/master/docs/node-removal.md))

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Should we mark drives as OFFLINE instead of removing?  |  | Open  |
ISSUE-2 | Should we marked volumes as INOPERATIVE instead of removing?  |  | Open  |   
