# Proposal: Temporary Maintenance Mode (TMM)

Last updated: 23.09.2021


## Abstract

Temporary Maintenance Mode is a removing node temporarily. All CSI components will be handled by the CSI Operator.

## Background

A user might want to take a single node down for maintenance (say to repair a bad hardware component) or they might do a sequential rolling maintenance mode (MM) workflow (say to run software upgrade on all nodes). In the rolling MM case one node enters MM, finishes maintenance operation (say software upgrade), then is exited from MM and immediately the next node is put in MM without any time in between.

When a host is put in MM in this mode, the corresponding k8s node in supervisor cluster will get the following taint:
```
kubectl taint nodes node1 drain=planned-downtime
```

When the node “exits” MM the taint will disappear from the node. At this point the node will become schedulable for all POD again

## Proposal

[A precise statement of the proposed change.]

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Compatibility

[A discussion of the change with regard to the compatibility with previous product and Kubernetes versions.]

## Implementation

[A description of the steps in the implementation, who will do them, and when.]

## Assumptions (if applicable)

ID | Name | Descriptions | Comments
---| -----| -------------| --------
ASSUM-1 |   |   |


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
