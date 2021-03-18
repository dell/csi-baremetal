# Proposal: Node remove

Last updated: 18.03.2021

## Abstract

Need to determine actions for CSI for node removing process

## Proposal

If user aims to delete node from cluster, he/she must perform following steps for CSI.  

####Before node removal
1) Determine removing Node UUID 

####After node deletion: 
1) Delete PVCs used by Pods, which were run on deleted node

2) Patch according Volumes custom resources with empty finalizer  
```
kubectl get volume -o jsonpath='{range .items[?(.spec.NodeId == "<node_uuid>")]}{@.metadata.name}{" "}' | xargs kubectl patch volume --type merge -p '{"metadata":{"finalizers":null}}'
```
3) Delete volume CR with according node id
```
kubectl get volume -o jsonpath='{range .items[?(.spec.NodeId == "<node_uuid>")]}{@.metadata.name}{" "}' | xargs kubectl delete volume
```
4) Patch according LVG custom resources with empty finalizer
```
kubectl get lvg -o jsonpath='{range .items[?(.spec.Node == "<node_uuid>")]}{@.metadata.name}{" "}' | xargs kubectl patch lvg --type merge -p '{"metadata":{"finalizers":null}}'
```
5) Delete LVG CR with according node id
```
kubectl get lvg -o jsonpath='{range .items[?(.spec.Node == "<node_uuid>")]}{@.metadata.name}{" "}' | xargs kubectl delete lvg
```
6) Patch according CSI bare-metal Node custom resources with empty finalizer
``` 
kubectl get csibmnode -o jsonpath='{range .items[?(.spec.UUID == "<node_uuid>")]}{@.metadata.name}{" "}' | xargs kubectl patch csibmnode --type merge -p '{"metadata":{"finalizers":null}}'
```
7) Delete CSI bare-metal Node custom resource
``` 
kubectl get csibmnode -o jsonpath='{range .items[?(.spec.UUID == "<node_uuid>")]}{@.metadata.name}' | xargs kubectl delete csibmnode
```
8) Delete available capacity CR with according node id
``` 
kubectl get ac -o jsonpath='{range .items[?(.spec.NodeId == "<node_uuid>")]}{@.metadata.name}{" "}' | xargs kubectl delete ac
```
9) Delete drive CR with according node id 
``` 
kubectl get drive -o jsonpath='{range .items[?(.spec.NodeId == "<node_uuid>")]}{@.metadata.name}{" "}' | xargs kubectl delete drive
```
10) Restart new created Pods to create new PVCs for them or create manually necessary PVCs for Pods

## Compatibility

There is no problem with compatibility

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Should we mark drives as OFFLINE instead of removing  |  | Open  |   
ISSUE-2 | Should we marked volumes as INOPERATIVE instead of removing?  |  | Open  |   