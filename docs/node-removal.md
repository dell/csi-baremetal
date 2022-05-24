# Node remove

Last updated: 19.03.2021

## Abstract

Need to determine actions for CSI for node removing process

## Proposal

### Automated

#### User actions

- CSI - components on CSI side including CSI Operator
- Operator - application controller (or other service, which watch resources)

Removal procedure:
1. Taint a node with:
- `node.dell.com/drain=drain:NoSchedule`
2. Wait until Operator replaced PVs and destroyed pods
3. Delete a node from kubernetes cluster
4. CSI deletes all corresponding CR resources automatically (including Csibmnode)

After removal the node can't be restored as in [replacement](https://github.com/dell/csi-baremetal/blob/master/docs/proposals/node-replacement.md)!

#### Implementation

Set label:
1. CSI Operator must watch Nodes on taint changing events
2. Label the Csibmnode with `node.dell.com/drain=drain` if the Node has taint `node.dell.com/drain=drain:NoSchedule`

Unset label:
1. CSI Operator must watch Nodes on taint changing events
2. Check, if the Csibmnode has `node.dell.com/drain=drain` label and the Node with the same ID has no `node.dell.com/drain=drain:NoSchedule` taint
3. Delete `node.dell.com/drain=drain` label

Removal:
1. CSI Operator must watch Nodes on deleting events
2. Start removal procedure, if if the Csibmnode has `node.dell.com/drain=drain` label and the Node with the same ID has been deleted from kubernetes cluster
3. Delete drives, ACs
4. Patch and delete Volumes, Lvgs, Csibmnode

### Manual

If user aims to delete node from cluster, he/she must perform following steps for CSI. 

#### Node is healthy

After node drain:
1) Determine removing Node UUID
2) Delete PVCs used by Pods, which were run on deleted node

#### Node is unhealthy

Before node deletion:
1) Determine removing Node UUID

After node deletion:
1) Delete PVCs used by Pods, which were run on deleted node

#### User actions
After node deletion:
1) Patch according Volumes custom resources with empty finalizer
```
kubectl get volume | grep <node_id> | awk '{print $1}' | xargs kubectl patch volume --type merge -p '{"metadata":{"finalizers":null}}'
```
2) Delete volume CR with according node id
```
kubectl get volume | grep <node_id> | awk '{print $1}' | xargs kubectl delete volume
```
3) Patch according LVG custom resources with empty finalizer
```
kubectl get lvg | grep <node_id> | awk '{print $1}' | xargs kubectl patch lvg --type merge -p '{"metadata":{"finalizers":null}}'
```
4) Delete LVG CR with according node id
```
kubectl get lvg | grep <node_id> | awk '{print $1}' | xargs kubectl delete lvg
```
5) Delete CSI bare-metal Node custom resource
``` 
kubectl get csibmnode | grep <node_id> | awk '{print $1}' | xargs kubectl delete csibmnode
```
6) Delete available capacity CR with according node id
``` 
kubectl get ac | grep <node_id> | awk '{print $1}' | xargs kubectl delete ac
```
7) Delete drive CR with according node id 
``` 
kubectl get drive | grep <node_id> | awk '{print $1}' | xargs kubectl delete drive
```
8) Restart new created Pods to create new PVCs for them or create manually necessary PVCs for Pods

## Compatibility

There is no problem with compatibility

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Should we mark drives as OFFLINE instead of removing  |  | Open  |   
ISSUE-2 | Should we marked volumes as INOPERATIVE instead of removing?  |  | Open  |
ISSUE-3 | Should we do node removal procedure in node-controller? | Node-controller reconciles Csibmnodes and nodes already | Open |
ISSUE-4 | Should we delete related PVs and PVCs? |  | Open  |
ISSUE-5 | We don't provide any procedure to remove a node without deleting it from cluster |  | Open |
