# Proposal: Supporting for CSI volume health feature 

Last updated: 24.12.2021

## Abstract

This proposal contains approaches for supporting CSI volume health feature, using Kubernetes volume health monitor mechanism. 

## Background

Currently, Baremetal CSI does not support any general conventional way to monitor Persistent Volumes after they are provisioned in Kubernetes.
This makes it very hard to debug and detect root causes when something happens to the volumes. 
An application may detect that it can no longer write to volumes. However, the root cause happened at the underlying storage 
system level. This requires several teams jointly debug and find out what has triggered the problem. 
It could be that the volume runs out of capacity and needs an expansion. 
It could be that the volume was deleted by accident outside of Kubernetes. 
In the case of local storage, the local PVs may not be accessed any more due to the nodes break down. 
In either case, it will take lots of effort to find out what has happened at the infrastructure layer. 
It will also take complicated process to recover. Admin usually needs to intervene and fix the problem.

At this moment, Baremetal CSI supports drive replacement mechanism, which is triggered by unhealthy drive state, 
but this feature is not directly related with Persistent Volumes monitoring. With k8s volume health monitoring mechanism,
unhealthy volumes can be detected and fixed early and therefore could prevent more serious problems to occur.

## Proposal

The main purpose of this proposal consists in implementation details of CSI volume health function provided by Kubernetes.
By communicating Baremetal CSI driver with this function, Kubernetes can retrieve any errors detected by the underlying storage system. 
Kubernetes reports an event and log an error about this PVC so that user can inquire this information and decide how to handle them. 
For example, if the volume is out of capacity, user can request a volume expansion to get more space. 

There could be conditions that cannot be reported by a CSI driver. One or more nodes where the volume is attached to may be down.
This can be monitored and detected by the volume health controller so that user knows what has happened.

Basically we should support the following enhancements:
* Extend Kubelet's existing volume monitoring capability to also monitor volume health on each kubernetes worker node.
  In addition to gathering existing volume stats, Kubelet also watches volume health of the PVCs on that node. 
  If a PVC has an abnormal health condition, an event will to reported on the pod object that is using the PVC. 
  If multiple pods are using the same PVC, events will be reported on multiple pods.
  To support this feature we should implement ```NodeGetVolumeStats``` with setting abnormal conditions if such.
* An external monitoring controller on the master node. Monitoring controller reports events on the PVCs.
  To support this feature we should implement ```ListVolumes``` and ```ControllerGetVolume``` methods with setting abnormal conditions if such.

## Compatibility

Kubernetes volume health monitoring feature was first introduced in Kubernetes 1.19, but there was an External Health Monitor Agent that monitors volume health from the node side. 
In the Kubernetes 1.21 release, the node side volume health monitoring logic was moved to Kubelet to avoid duplicate CSI RPC calls.
So, the Kubernetes minimum version for supporting this feature becomes 1.19 instead of 1.18 as it is currently.
Also, it is alpha feature currently - certain changes can be make, so we should support the corresponding compatibility. 

## Implementation

#### Use cases 
Many things could happen to the underlying storage system after a volume is provisioned in Kubernetes.

* Volume could be deleted by accident outside of Kubernetes.
* The disk that the volume resides on could be removed temporarily for maintenance.
* The disk that the volume resides on could fail.
* Volume could be out of capacity.
* The disk may be degrading which affects its performance.

If the volume is mounted on a pod and used by an application, the following problems could also happen.
* There may be read/write I/O errors.
* The file system on the volume may be corrupted.
* Filesystem may be out of capacity.
* Volume may be unmounted by accident outside of Kubernetes.

#### High level design

Kubernetes provides a mechanism for CSI drivers to report volume health problems at the controller and node levels.
Two main parts are involved here in the architecture.

External Controller:
* The external controller is deployed as a sidecar together with the CSI controller driver, similar to how the external-provisioner
  sidecar is deployed.
* Kubernetes triggers controller RPC to check the health condition of the CSI volumes.
* The external controller sidecar will also watch for node failure events. This component can be enabled via a separate flag.

Kubelet:
* Kubelet already collects volume stats from CSI node plugin by calling CSI function NodeGetVolumeStats.
* In addition to existing volume stats collected already, Kubelet also checks volume condition collected from the same CSI function
  and log events to Pods if volume condition is abnormal.

Basically we should support the following enhancements:
* Extend Kubelet's existing volume monitoring capability to also monitor volume health on each kubernetes worker node.
  In addition to gathering existing volume stats, Kubelet also watches volume health of the PVCs on that node.
  If a PVC has an abnormal health condition, an event will to reported on the pod object that is using the PVC.
  If multiple pods are using the same PVC, events will be reported on multiple pods.
  To support this feature we should implement ```NodeGetVolumeStats``` with setting abnormal conditions if such.
* An external monitoring controller on the master node. Monitoring controller reports events on the PVCs.
  To support this feature we should implement ```ListVolumes``` and ```ControllerGetVolume``` methods with setting abnormal conditions if such.

#### Implementation details

As it was already stated we should support the following features:
* Support ```ListVolumes``` controller RPC, which is called by external monitoring controller to find out existing volumes if supported by the CSI driver.
* Support ```GetVolume``` controller RPC, which is called by external monitoring controller to check health condition of a particular volume if it is supported and ListVolumes is not supported.
* Support ```NodeGetVolumeStats``` RPC, which is called by kubelet for any PV that is mounted to check if volume is mounted and usable, e.g., filesystem corruption, bad blocks, etc.

Also, for support Kubernetes volume health monitor mechanism at Baremetal CSI we need to update version of github.com/container-storage-interface/spec to the latest: v1.5.0

_External Monitoring Controller integration_
1. ```ListVolumes``` controller RPC: ```ListVolumes(context.Context, *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error)```
   1. For supporting ```ListVolumes``` we should add ```LIST_VOLUMES``` and ```VOLUME_CONDITION``` capabilities for supporting list
   2. While supporting ```ListVolumes``` we should support pagination over incoming tokens:
      ```
      type ListVolumesRequest struct {
         MaxEntries    int32
         StartingToken string
      }
      ```
   3. In response, we should support list of volumes: id and capacity with volume conditions:
      ```go
      type ListVolumesResponse struct {
         Entries   []*ListVolumesResponse_Entry
         NextToken string
      }
      ```
      ```go
      type ListVolumesResponse_Entry struct {
         Volume *Volume
      }
      ```
      ```go
      type Volume struct {
         CapacityBytes int64
         VolumeId string
      }
      ```
   4. The overall algorithm will look like this:
      1. List volumes with income ```StartingToken``` and in range of ```MaxEntries```.
      2. For each found volume - output it's id and size, also output it's published node_id and next token for paginate.
2. ```ControllerGetVolume``` controller RPC: ```ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error)```
   1. For supporting ```ControllerGetVolume``` we should add ```GET_VOLUME``` and ```VOLUME_CONDITION``` capabilities for supporting get
   2. While supporting ```ControllerGetVolume``` we should support get volume information by it's id:
      ```go
      type ControllerGetVolumeRequest struct {
          // The ID of the volume to fetch current volume information for.
          volumeId string
      }
      ```
   3. In response, we should support volume information as well as its status:
      ```go
      type ControllerGetVolumeResponse struct {
          Volume Volume
          Status VolumeStatus
      }
      ```
      ```go
      type VolumeStatus struct {
          // A list of all the `node_id` of nodes that this volume is controller published on.
          PublishedNodeIds []string
          // Information about the current condition of the volume.
          VolumeCondition VolumeCondition
      }
      ```
      ```go
      type VolumeCondition struct {
          // Normal volumes are available for use and operating optimally.
          // An abnormal volume does not meet these criteria.
          Abnormal bool

          // The message describing the condition of the volume.
          Message string
      }
      ```
   4. The overall algorithm will look like this:
       1. Get volume with income id. If not found - return error: ```NotFound```
       2. Check volume Health, CSIStatus, OperationalStatus and Usage parameters:
          1. If Health != GOOD - set abnormal value to true with corresponding message.
          2. If CSIStatus == FAILED - set abnormal value to true with corresponding message.
          3. If OperationalStatus == MISSING - set abnormal value to true with corresponding message.
          4. If Usage == FAILED - set abnormal value to true with corresponding message.
3. Deploy external health monitor controller will be deployed as a sidecar together with the CSI controller driver, 
   similar to how the external-provisioner sidecar is deployed: see https://github.com/kubernetes-csi/external-health-monitor#csi-external-health-monitor-controller-sidecar-command-line-options.
4. Set enable-node-watcher of external health monitor controller sidecar command line's option to true for enabling node-watcher. 
   Node-watcher evaluates volume health condition by checking node status periodically. \
   Node down event workflow:
   1. External monitoring controller will check if node is marked as unresponsive by the node controller.
   2. The external monitoring controller will track which pods are using which PVCs and what nodes they got scheduled to. 
   3. In the case that a node goes down, the controller will report an event for all PVCs on that node. 
   4. The external monitoring controller reports node down events on the PVCs.

_Kubelet integration_ \
Node Volume Health checks if a volume is still mounted and usable. To check whether a volume is usable, we should check 
if filesystem is corrupted, whether there are bad blocks, etc. in this RPC.

```NodeGetVolumeStats``` Node RPC: ```NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error)```
1. For supporting ```NodeGetVolumeStats``` we should add ```GET_VOLUME_STATS``` and ```VOLUME_CONDITION``` capabilities for supporting volume stats
2. While supporting ```NodeGetVolumeStats``` we should support get volume information by it's id:
   ```go
   type NodeGetVolumeStatsRequest struct {
       VolumeId string 

       // It can be any valid path where volume was previously
       // staged or published.
       // It MUST be an absolute path in the root filesystem of
       // the process serving this request.
       VolumePath string

      // The path where the volume is staged, if the plugin has the
      // STAGE_UNSTAGE_VOLUME capability, otherwise empty.
      // If not empty, it MUST be an absolute path in the root
      // filesystem of the process serving this request.
       StagingTargetPath string
   }
   ```
3. In response, we should support volume information as well as its status:
   ```go
   type NodeGetVolumeStatsResponse struct {
      Usage []*VolumeUsage
      // Information about the current condition of the volume.
      VolumeCondition VolumeCondition 
   }
   ```
   ```go
   type VolumeUsage struct {
       // The available capacity in specified Unit.
       Available int64
       // The total capacity in specified Unit.
       Total int64
       // The used capacity in specified Unit.
       Used int64
       // Units by which values are measured. This field is REQUIRED. 
       Unit VolumeUsage_Unit
   }
   type VolumeUsage_Unit int32
   const (
       VolumeUsage_UNKNOWN VolumeUsage_Unit = 0
       VolumeUsage_BYTES   VolumeUsage_Unit = 1
       VolumeUsage_INODES  VolumeUsage_Unit = 2
   )
   ```
   ```go
   type VolumeCondition struct {
       // Normal volumes are available for use and operating optimally.
       // An abnormal volume does not meet these criteria.
       Abnormal bool

       // The message describing the condition of the volume.
       Message string
   }
   ```
   4. The overall algorithm will look like this:
      1. Get volume with income id. If not found - return error: ```NotFound```. 
      2. Check volume Health, CSIStatus, OperationalStatus and Usage parameters:
          1. If Health != GOOD - set abnormal value to true with corresponding message.
          2. If CSIStatus == FAILED - set abnormal value to true with corresponding message.
          3. If OperationalStatus == MISSING - set abnormal value to true with corresponding message.
          4. If Usage == FAILED - set abnormal value to true with corresponding message.
      3. Check staging target path (StagingTargetPath) is mounted (if it is not empty). If mount no found - set abnormal value to true with corresponding message. 
      4. Check if target path (VolumePath) is mounted. If mount no found - set abnormal value to true with corresponding message.
      5. Check if volume path (VolumePath) is accessible just by reading dir:
         ```go
         _, err = ioutil.ReadDir(volumePath)
         if err != nil {
             ...
         }
         ```
         If check was failed - set abnormal value to true with corresponding message.
      6. If Volume mode == FS, then check whether fs resides at volume path (VolumePath) is corrupted via ```fsck``` or ```xfs_repair``` commands (depends on fs type).
         Commands must be executed in readonly mode (e.g. ```xfs_repair -nfv```).
         **Note:** because they will be executed in read only mode on mounted fs, there can be deviations in results.
         So, we should handle this situations, e.g. by making several runs of the command and setting the abnormal value to true only if 
         all the results of this runs show that fs is corrupted.
      7. If Volume mode == FS, then get volume metrics for mounted volume path (VolumePath) with ```Info(volumePath)``` function from ```"k8s.io/kubernetes/pkg/volume/util/fs"``` package.
         If Volume mode == RAW/RAW_PART, then just get volume size in bytes.
         After obtaining these metrics - fill ```Usage``` field with them and return abnormal value to false.
