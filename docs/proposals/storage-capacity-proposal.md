# Proposal: Implement storage capacity feature

Last updated: 08.07.2022

## Abstract

Specify required changes to support storage capacity feature, compare storage tracking with ACR and CSIStorageCapacity resources.

## Background

Currently the Plugin uses AC and ACR CRD algoritm for dynamic provisioning. The scheduler-extender (SE) sidecar searches all PVCs, creates corresponding CapacityRequests and requests reservations by creating ACR objects. List of reserved ACRs is filled by reservationcontroller within csi driver controller.
This algorithm has some disadvantages:
- Every user of plugin must use the Extender
- Reservation blocks suitable AC on every requested node. It slows scheduling, because reserved ACs can't be used for other volumes until ACR is deleted
- Reservation cannot be made without pod passing scheduling cycle, which makes at least two scheduling cycles required to place a pod  

## Proposal

Implement GetCapacity, support Storage Capacity Feature, make an extra option for users to remove Extender and ACR resource.

## Rationale

Advantages:
- First of all, it is a step towards standartization of storage tracking algorithm
- Implementing Storage Capacity feature may speed up scheduling
- Generic ephemeral volumes are already supported by driver, they can be provisioned using storage capacity tracking too
- Storage Capacity objects are being produced and updated independently from PVC. This means that when the pod first goes through the scheduling cycle, it is likely that information about available capacity have already been gathered to Storage Capacity objects and the
pod can be scheduled at the same cycle

Disadvantages:
- Storage Capacity objects require more memory in etcd. In contrast, ACR are deleted after volume creation
- The feature doesn't solve races issues, two pods still may reserve same disk

Another option is to implement custom scheduler, which can increase performance even more and provide
a way for users to describe storage load requirements more precise, for example set disk affinity/anti-affinity requirement for application.

## Compatibility

ACR can be used with CSIStorageCapacity. Scheduler uses CSIStorageCapacities at the filter stage and filters out all nodes that have not enough capacity. Custom extension points work only after default ones, so 
using CSIStorageCapacity with ACR may decrease number of requests to extender and number of nodes in requests.

## Implementation

To enable the feature, controller must expose `GET_CAPACITY` capability which requires changes in `ControllerGetCapabilities` function. 
Secondly, supported Kubernetes version should be bumped to such that supports storage capacity. The feature is stable since Kubernetes v1.24.
To enable producing CSIStorageCapacity objects such steps are required:
- `POD_NAME` and `NAMESPACE` environment variables for external-provisioner must be set
like this
```
env:
- name: NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
- name: POD_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.name
```
- Pass `--enable-capacity=true` to external-provisioner.
- To enable ownership for CSIStorageCapacity objects and guarantee deleting objects when the driver is uninstalled, `--capacity-ownerref-level=2` argument must be passed to external-provisioner. The level indicates the number of objects that need to be traversed starting from the pod to reach the owning object for CSIStorageCapacity objects, which is `Deployment` now.
- Add `storage.k8s.io` RBAC group
- Following RBAC rules must be added to controller-rbac
```
  # Permissions for CSIStorageCapacity are only needed enabling the publishing
  # of storage capacity information.
  - apiGroups: ["storage.k8s.io"]
    resources: ["csistoragecapacities"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # The GET permissions below are needed for walking up the ownership chain
  # for CSIStorageCapacity. They are sufficient for deployment via
  # StatefulSet (only needs to get Pod) and Deployment (needs to get
  # Pod and then ReplicaSet to find the Deployment).
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get"]
  - apiGroups: ["apps"]
    resources: ["replicasets"]
    verbs: ["get"]
```
To enable using these object during scheduling `storageCapacity: true` must be added to CSIDriver spec.

To implement `GetCapacity` it is possible to use AC. `GetCapacity` types look like this

```

type GetCapacityRequest struct {
  // These are the same
  // `volume_capabilities` the CO will use in `CreateVolumeRequest`.
  VolumeCapabilities []*VolumeCapability
  // These are the same `parameters` the CO will
  // use in `CreateVolumeRequest`.
  Parameters map[string]string
  // This is the same as the
  // `accessible_topology` the CO returns in a `CreateVolumeResponse`.
  AccessibleTopology   *Topology
}

type GetCapacityResponse struct {
  // The available capacity, in bytes, of the storage that can be used
  AvailableCapacity int64
  // The largest size that may be used in a
  // CreateVolumeRequest.capacity_range.required_bytes field
  // to create a volume with the same parameters as those in
  // GetCapacityRequest.
  MaximumVolumeSize *wrappers.Int64Value
  // The smallest size that may be used in a
  // CreateVolumeRequest.capacity_range.limit_bytes field
  // to create a volume with the same parameters as those in
  // GetCapacityRequest.
  MinimumVolumeSize    *wrappers.Int64Value
}
```

For each segment and each storage class, CSI GetCapacity is called once with the topology of the segment and the parameters of the class. For each request we should list all
AC in cluster, filter out AC that do not satisfy segment (node), parameters and volume capacilities of request and find total size of remaining AC for `AvailableCapacity`, max AC
for `MaximumVolumeSize` and, in case of non-LVG SC, min AC for `MinimumVolumeSize` or minimum size of logical disk (10 Mi) for LVG SC.

Removing ACR require changes in `CreateVolume` function (and others). Storage Capacity helps scheduler
to pick node for pod, but drive selection algorithm must be implemented by developers and may depend on AC as well. 

## Assumptions (if applicable)

| ID      |     Name     | Descriptions | Comments |
|---------|--------------|--------------|----------|
| ASSUM-1 |  Performance | Scheduling time may decrease             |  Decreasing of scheduling time was noted in test Kind cluster with disabled Extender |

## Open issues (if applicable)

| ID      | Name | Descriptions | Status | Comments |
|---------|------|--------------|--------|----------|
| ISSUE-1 |    Scheduling can fail permanently if Pod uses multiple volumes  | If Pod uses multiple volumes, one volume can be created in a topology segment which then does not have enough capacity left for another volume. This requires manual intervention.   |    Open    |    https://kubernetes.io/docs/concepts/storage/storage-capacity/#limitations      |   
