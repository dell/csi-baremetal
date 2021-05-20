# Proposal: Specific kernel version for Node daemonset

Last updated: 04.05.2021


## Abstract

Logic to select a csi-baremetal-node image with specific kernel version based on reconciliation of k8sNodes metainfo.

## Background

We used specific system calls in the node part, which depends on kernel version. It is need to deploy different types of csi-baremetal-node daemonset to avoid compatibility issues.

## Proposal

- Create Map (NodeMap) in CSI Operator:

Example:
```
Struct NodeDescription: {
	name,
	meta: (ImageName, DaemonSetName),
	isFit: func(kernel-version) bool //Parse kernel-version and decide if it is fit
	}
	
NodeMap: {
{
	default: {default NodeDescription, isDeployed: False}
	5.4: {NodeDescription:
			name: kernel-5.4
			image: node-kernel-5.4, 
			daemonset: csi-baremetal-node-kernel-5.4,
			isFit: kernel-version > 5.4
		,isDeployed: False}
	...
}
```

If we want to add new Platform:
	1. Create Dockerfile with specific reference, build and push image to repo
	2. Implement new NodeDescription and add field in NodeMap

NodeMap will be updated each time when CSI Deployment reconciles. We don't need to create specific recource, becouse kernel information for all nodes can be recieved in any moment.

- Move logic of updating k8sNodes' labels to operator. Steps in `Node.Update` in reconciliation loop:

	1. Get list of all k8sNodes

	2. Parse kernel-version for each node and update NodeMap (Check if at least one k8sNode with specific kernel-version exists -> isDeployed=true)

	3. Update CSI labels with kernel-version on k8sNodes

	4. Deploy required daemonsets with specific node-image and nodeSelector (for all value with isDeployed=true in NodeMap we need to create DaemonSet with passed name and node-image)
	
Using label:

```
nodes.csi-baremetal.dell.com/kernel-version=<NodeKernelVersion>
```

- Add reconciliation loop running on cluster resize events caused by adding or updating a k8sNode

Events:

	1. Create a k8sNode
	
	2. Update kernel-version status field for any k8sNode


## Rationale

Alternative approach:

Separate node-controller marks k8sNodes with labels with kernel version. Operator creates csi-baremetal-node daemonsets mapping `image: node-kernel-5.x` with `nodeAffinity label: kernel-version:5.x`.

Problem: How to check if node-controller finishes work with setting nodeSelectors? How to identify case if one csibmnode is deleted?

## Compatibility

This logic would work only for deploying CSI with Operator

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Errors while csi-baremetal-node creation  | Currently we create csi-baremetal-node-controller and csi-baremetal-node in the same time. It leads to errors while node images creation until csibmnodes haven't been reconciled. We can wait updating labels on all k8sNodes before csi-baremetal-node installing | Open  | Some labels cannot be set because of nodeSelectors or csibmnode deleting. How to get actual list of available nodes?    
