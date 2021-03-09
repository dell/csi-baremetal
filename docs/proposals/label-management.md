# Proposal: Node label management

Last updated: 09.03.2021


## Abstract

Removing additional node labels/annotations to prevent possible errors with non-consistent information

## Background

Currently `csi-baremetal-operator` and `node-driver-registrar` sidecar container from `csi-baremetal-driver` use other ways to get special nodeID.

All labels/annotations with UUID on a node, which was created with csi-baremetal:

```
Labels:
	nodes.csi-baremetal.dell.com/uuid=98ad16dd-5702-4dfe-bbca-008b65422831
Annotations:
	csi.volume.kubernetes.io/nodeid: {"csi-baremetal":"98ad16dd-5702-4dfe-bbca-008b65422831"}
	nodes.csi-baremetal.dell.com/uuid: 98ad16dd-5702-4dfe-bbca-008b65422831
```

csi-baremetal operator:

- Annotations: **nodes.csi-baremetal.dell.com/uuid: 98ad16dd-5702-4dfe-bbca-008b65422831**

node-driver-registrar + csi-provisioner:

- Labels: **nodes.csi-baremetal.dell.com/uuid=98ad16dd-5702-4dfe-bbca-008b65422831**

- Annotations: csi.volume.kubernetes.io/nodeid: {"csi-baremetal":"98ad16dd-5702-4dfe-bbca-008b65422831"}

There are 2 copies of the same information in label section and annotation section.

Problem:

```
csi-baremetal-operator/controller : create csibmnode
csi-baremetal-operator/controller : mark k8sNode with annotation
csi-baremetal-node/node : check k8sNode annotation
csi-baremetal-node/node : send nodeID to node-driver-registrar
csi-baremetal-node/node-driver-registrar : get nodeID from node
csi-baremetal-node/node-driver-registrar : create or check k8sNode label with nodeID
---Error if nodeID from node and nodeID in label are different---
csi-baremetal-node/node-driver-registrar : use nodeID in topology
```

csi-baremetal-operator has to remove k8sNode UUID-annotation after its uninstallation. UUID-label have no responsible controller and it remains on a node.

## Proposal

1. Use only csibmnodes instead of annotations to mark work nodes in a cluster. node-driver-registrar will receive nodeID via parsing csibmnodes CR list.

Proposed algorithm:

```
csi-baremetal-operator/controller : create csibmnode
csi-baremetal-node/node : get csibmnode-CR list and find matching hostname-uuid
csi-baremetal-node/node : create topology with {topologyKey : UUID}
```

2. Rename topologyKey:

`nodes.csi-baremetal.dell.com/uuid -> nodes.csi-baremetal.dell.com/external-topology-key`

```
csi.Topology{
	Segments: map[string]string{
		"nodes.csi-baremetal.dell.com/external-topology-key": 98ad16dd-5702-4dfe-bbca-008b65422831,
	}
```

3. Remove label `nodes.csi-baremetal.dell.com/uuid` after `csi-baremetal-controller` and `node-daemonset` uninstallation.

It can be done in `csi-baremetal-node/node` after receiving SIGTERM with special handler.

## Rationale

Removing label `nodes.csi-baremetal.dell.com/uuid`:

- in `csi-baremetal-node/node` after receiving SIGTERM (if node-container has error and is deleted, label will be removed)
- in `csi-baremetal-operator` after uninstalling csibmnode (node-components may exist, when user or other service delete csibmnode)
- in `csi-baremetal-CR-operator`? after uninstalling csi-baremetal CR

## Compatibility

There is no problem with compatibility

## Implementation

In csi-baremetal-operator/controller (remove annotation with nodeID):

```
updateNodeLabelsAndAnnotation -> updateNodeLabels
removeLabelsAndAnnotation -> removeNodeLabels
```

In csi-baremetal-node/node, extender, loopbackmanager:

```
bmNodeCRs := new(nodecrd.NodeList)
if err := client.ReadList(context.Background(), bmNodeCRs); err != nil {
	return "", fmt.Errorf("Unable to read Node CRs list: %v", err)

}
bmNodes := bmNodeCRs.Items

for i := range bmNodes {
	if bmNodes[i].Spec.Addresses["Hostname"] == nodeName {
		return bmNodes[i].Spec.Addresses["Hostname"], nil
	}
}

return "", fmt.Errorf("csibmnode for %s hadn't been created", nodeName)
```

## Open issues

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | What means csibmnode delete action? | Now csibmnode-CR can be deleted by user, when node-components are running | Open | Node-component can't be restarted, if there is no csibmnode for this k8sNode