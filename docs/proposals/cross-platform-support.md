# Proposal: Cross platform support

Last updated: 19.04.2021


## Abstract

Logic to select a csi-baremetal-node image based on reconciliation of k8sNodes metainfo.

## Background

We used specific system calls in the node part, which depends on kernel version. It is need to deploy different types of csi-baremetal-node daemonset to avoid compatibility issues.

## Proposal

- Create Map:

```
{
	default: {image: node, daemonset: csi-baremetal-node, isDeployed: False}
	5.x: {image: node-kernel-5.x, daemonset: csi-baremetal-node-kernel-5.x, isDeployed: False}
	...
}
```

- Move logic of updating k8sNodes' labels to operator. Steps in `Node.Update`:

	1. Get list of all k8sNodes

	2. Parse kernel-version and update k8sNodes' labels

	3. Update kernel-version Map (isDeployed -> true if version detected)

	4. Deploy required daemonsets

- Run reconciliation loop on cluster resize events caused by adding or deleting a k8sNode

- Merge labels

Old: 

```
{
	os-name: x
	os-version: x
	kernel-version: x
}
```

New:

```
{
	node-daemonset-image: x
}
```

## Rationale

Alternative approach:

Separate node-controller marks k8sNodes with labels with kernel version. Operator creates csi-baremetal-node daemonsets mapping `image: node-kernel-5.x` with `nodeAffinity label: kernel-version:5.x`.

Problem: How to check if node-controller finishes work with set nodeSelector or one deleted csibmnode?

## Compatibility

There is no problem with compatibility

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Errors while csi-baremetal-node creation  | Currently we create csi-baremetal-node-controller and csi-baremetal-node in the same time. It leads to errors while node images creation until csibmnodes haven't been reconciled. We can wait updating labels on all k8sNodes before csi-baremetal-node installing | Open  | Some labels cannot be set because of nodeSelectors or csibmnode deleting. How to get actual list of available nodes?    
