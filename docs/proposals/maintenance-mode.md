# Proposal: Node Maintenance Mode (MM)

Last updated: 8.10.2021


## Abstract

Node Maintenance is a removing node temporarily. All CSI components, except DaemonSets (csi-baremetal-node, csi-baremetal-se), will be delete by the CSI Operator.

## Background

A user might want to take a single node down for maintenance (say to repair a bad hardware component) or they might do a sequential rolling maintenance mode (MM) workflow (say to run software upgrade on all nodes). In the rolling MM case one node enters MM, finishes maintenance operation (say software upgrade), then is exited from MM and immediately the next node is put in MM without any time in between.

### Entering MM:

When a host is put in MM in this mode, the corresponding k8s node in supervisor cluster will get the following taint:
```
kubectl taint node <node name> drain=planned-downtime:NoSchedule
```

### Exiting MM:
When the node “exits” MM the taint will disappear from the node. At this point the node will become schedulable for all POD again.

To exit TMM user must remove taint from the node:
```
kubectl taint node <node name> drain=planned-downtine:NoSchedule-
```

## Proposal

If Node has taint ```node.dell.com/drain=planned-downtime:NoSchedule``` CSI Operator deletes all CSI components, except DaemonSets (csi-baremetal-node, csi-baremetal-se).


## Compatibility

There is no problem with compatibility


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
