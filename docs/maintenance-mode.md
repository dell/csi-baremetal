# Proposal: Node Maintenance Mode (MM)

Last updated: 8.10.2021


## Abstract

The node is placed in Maintenance Mode - CSI pods are temporarily deliting on the node. 

## Proposal

To move the node to the maintenance mode, need to set the taint ```node.dell.com/drain=planned-downtine:NoSchedule```.
All CSI components will be delete by the CSI Operator, except DaemonSets (```csi-baremetal-node, csi-baremetal-se```).

```csi-baremetal-node``` and ```csi-baremetal-se``` remain on the node for the execution of service procedures.

### Entering MM:

When a host is put in MM in this mode, the corresponding k8s node in supervisor cluster will get the following taint:
```
kubectl taint node <node name> `node.dell.com/drain=planned-downtime:NoSchedule
```

### Exiting MM:
When the node “exits” MM the taint will disappear from the node. At this point the node will become schedulable for all POD again.

To exit TMM user must remove taint from the node:
```
kubectl taint node <node name> node.dell.com/drain=planned-downtine:NoSchedule-
```

## Compatibility

There is no problem with compatibility


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
