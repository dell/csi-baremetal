# Proposal: Node Replacement (NR)

Last update 10/12/2020

## Abstract

CSI-Baremetal plugin (CSI BM, or just BM) should handle cases when whole node or it part like motherboard or OS was replaced but drives remain the same. In these scenarios BM should detect drives on "new" node and map them to the existed custom resources (Drive, Volume, LVG, AC, ACR).

K8s node ID is not applicable here because if OS is reinstalled then node ID will be changed too.

Since we can't rely on that each platform on which BM is installed can guarantee persistent node ID across the cluster (even after node was removed from K8s cluster and then added as new one with "old" drives) BM should handle these cases itself.


## Background
At now BM use K8s node ID as ID for each resources that is managed by particular CSI node service (`node svc`).

Node UI is reported as part of Topology struct during [NodeGetInfo](https://github.com/dell/csi-baremetal/blob/master/pkg/node/node.go#L580) request by each `node svc`:
```
topology := csi.Topology{
        Segments: map[string]string{
            "baremetal-csi/nodeid": s.nodeID,
        },
}
```
Node ID also is being used as a marker for topology constraints during CreateVolume request. That means that K8s sets node ID as part of node affinity requirements for each PV:
```
# kubectl describe pv pv1
.........................
Node Affinity:    
  Required Terms: 
    Term 0:        baremetal-csi/nodeid in [567a2c26-6124-49ff-b950-eaabc2a50c6e]
```

In other words if some PV had been provisioned on some NODE_A and is used by POD_X, then POD_X could be scheduled only on NODE_A.

If node is replaced it's ID will be changed and on "new" node replica of `node svc` will be started. Then this service detect drives on the node and add them (create Drive CR, AC CR) as new drives but "old" resources, that has "old" node UID will be still present on the cluster.

## Proposal

Add new annotation (`csi-baremetal.node/id`) for each node in the cluster. Value for that annotation will be a some ID that will be unique across the K8s cluster. CSI `node svc` will be manage storage resources on the node only if node contains such annotation and uses annotation value as an ID for each managed resource. 

When node is replaced something or someone should set required annotation for the node (with "old" value) only after it `node svc` will manage drives on that node.

## Compatibility

There are no issues with compatibility.

## Implementation

`Node svc` should read annotation (`csi-baremetal.node/id`) for the node object on startup and use annotation value as unique ID for each managed CR. If node doesn't have such annotation `node svc` shouldn't start. Volume CR controller (part of `node svc`) should watch only for Volume CR that has in spec value of corresponding `csi-baremetal.node/id`.

## Open issues

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Who will be responsible for assigning annotation to the node? | When node svc starts it checks whether node, on which it's running, has annotation(`csi-baremetal.node/id`) or not | Open | There are 2 ways here: - Annotation and unique ID should be assigned by user and during NR user should assign "old" ID for "new" node. - It is be a part of CSI operator responsibilities. CSI operator will create CR for each node. In that CR information about hostname, ip and generated uid (that used as a key for annotation) will be stored. When node is replaced, operator will restore annotation based on hostname and ip.
ISSUE-2 | How does controller should handle NR procedures? | When NR is in place new CreateVolume requests can come and controller should take into account AC on replaced node. | resolved | Node state monitor removes all ACs that points on inaccessible node.
