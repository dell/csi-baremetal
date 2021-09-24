# Proposal: Temporary Maintenance Mode (TMM)

Last updated: 24.09.2021


## Abstract

Temporary Maintenance Mode is a removing node temporarily. All CSI components will be handled by the CSI Operator.
`
## Background

A user might want to take a single node down for maintenance (say to repair a bad hardware component) or they might do a sequential rolling maintenance mode (MM) workflow (say to run software upgrade on all nodes). In the rolling MM case one node enters MM, finishes maintenance operation (say software upgrade), then is exited from MM and immediately the next node is put in MM without any time in between.


### Entering:

When a host is put in MM in this mode, the corresponding k8s node in supervisor cluster will get the following taint:
```
kubectl taint node <node name> drain=planned-downtime:NoSchedule
```

### Rejecting:

When operation execution is not possible CSI Operator must reject TMM by putting corresponding annotation to the related PODs:
```
emm-failure : GenericFailure # if reason for failure can't be determined
emm-failure : NotEnoughResources # if mm fails because of lack of resources
emm-failure : TooManyExistingFailures # ???  *(if mm will cause loss of read/write quorum)*
```
When TMM is rejected user must remove taint from node
```
kubectl taint node <node name> drain=planned-downtine:NoSchedule-
```
CSI Operators should run removed PODs on the node again.

### Exiting:
When the node “exits” MM the taint will disappear from the node. At this point the node will become schedulable for all POD again.

To exit TMM user must remove taint from the node:
```
kubectl taint node <node name> drain=planned-downtine:NoSchedule-
```

## Proposal

* What are prechecks needed to start MM?
  * Is node already in MM
* What can be reasons for reject? 
* Is it enought kill pods for enter to MM?
* Who handle reject annotations?

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
