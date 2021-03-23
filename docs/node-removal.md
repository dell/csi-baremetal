# Proposal: Node remove

Last updated: 19.03.2021

## Abstract

Need to determine actions for CSI for node removing process

## Proposal

If user aims to delete node from cluster, he/she must perform following steps for CSI.  

###Node is healthy

####After node drain
1) Determine removing Node UUID 
2) Delete PVCs used by Pods, which were run on deleted node

###Node is unhealthy

####Before node deletion: 
1) Determine removing Node UUID 

####After node deletion: 
1) Delete PVCs used by Pods, which were run on deleted node

###Common actions
####After node deletion: 
1) Patch according Volumes custom resources with empty finalizer  
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
4) Delete LVG CR with according node id
```
kubectl get lvg | grep <node_id> | awk '{print $1}' | xargs kubectl delete lvg
```
5) Patch according CSI bare-metal Node custom resources with empty finalizer
``` 
kubectl get csibmnode | grep <node_id> | awk '{print $1}' | xargs kubectl patch csibmnode --type merge -p '{"metadata":{"finalizers":null}}'
```
6) Delete CSI bare-metal Node custom resource
``` 
kubectl get csibmnode | grep <node_id> | awk '{print $1}' | xargs kubectl delete csibmnode
```
7) Delete available capacity CR with according node id
``` 
kubectl get ac | grep <node_id> | awk '{print $1}' | xargs kubectl delete ac
```
8) Delete drive CR with according node id 
``` 
kubectl get drive | grep <node_id> | awk '{print $1}' | xargs kubectl delete drive
```
9) Restart new created Pods to create new PVCs for them or create manually necessary PVCs for Pods

## Compatibility

There is no problem with compatibility

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Should we mark drives as OFFLINE instead of removing  |  | Open  |   
ISSUE-2 | Should we marked volumes as INOPERATIVE instead of removing?  |  | Open  |   