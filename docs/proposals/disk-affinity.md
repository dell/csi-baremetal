# Proposal: Supporting disk affinity/anti-affinity feature 

Last updated: 18.01.2022

## Abstract

This proposal contains approach for supporting disk affinity/anti-affinity feature.

## Background

Current lvg storageclass cannot mount physical disks to PVs with the awareness of the requesting pods. So, it's possible
for different applications to share the same disk.
CSI should provide a way to support:
* mount PVs for the same pod on the same disk (disk affinity);
* do not mount PVs on the disks that already have some PVs from other specific pods (disk antiaffinity).

## Proposal

Usually for affinity/antiaffinity topology feature is used. But the minimal domain, on which this feature operates, is nodes, not disks.
So we should use a different approach for this one. Another approach, which is proposed, is to use annotations for pointing out how to use disk affinity/antiaffinity:
affinity.volumes.csi-baremetal.dell.com/types: <pod-bound-required|pod-bound-preferred|volume-bound-required|volume-bound-preferred|dedicated-required|dedicated-preferred> - specify disk affinity type:
* pod-bound-(required|preferred) - mount PVs for the same pod on the same disk, depending on the suffix - the behaviour is required/preferred;
* volume-bound-(required|preferred) - place the specific PVs on the same disks, depending on the suffix - the behaviour is required/preferred;
* dedicated-(required|preferred) - place volumes on the certain disks, which were dedicated for this application, depending on the suffix - the behaviour is required/preferred; 
affinity.volumes.csi-baremetal.dell.com/volumes: "[pv-1, pv-2], [pv-3, pv4], ..." - specify which pvs should be placed on the same disks.
affinity.volumes.csi-baremetal.dell.com/tolerations: nginx - specify acs for placement, which were dedicated to volumes of this pod.

antiaffinity.volumes.csi-baremetal.dell.com/types: <pod-label-required|pod-label-preferred|volume-bound-required|volume-bound-preferred> - specify disk antiaffinity type:
* pod-label-(required|preferred) - specify pods by their labels, which should not use the same drives as the current one, depending on the suffix - the behaviour is required/preferred;
* volume-bound-(required|preferred) - specify which volumes should not be placed together on the same drives, depending on the suffix - the behaviour is required/preferred.
antiaffinity.volumes.csi-baremetal.dell.com/labels: "${list of the labels}" - do not mount PVs on the disks that already have some PVs from other specific pods (specified by labels).
antiaffinity.volumes.csi-baremetal.dell.com/volumes: "[pv-1, pv-2], [pv-1, pv3], ..." - specify which pvs should not be placed on the same disks.

If we want to dedicate drive/lvg for the certain application, then we should place label on it's ac:
affinity.csi-baremetal.dell.com/taint: nginx

## Implementation

#### Client interface for injecting disk affinity/antiaffinity feature

In order to inject disk affinity/antiaffinity feature client should prepare the corresponding annotations for the application.
The following is a simple example for nginx StatefulSet:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
   name: web
spec:
   selector:
      matchLabels:
         app: nginx
   serviceName: "nginx"
   replicas: 3
   minReadySeconds: 10
   template:
      metadata:
         annotations:
            affinity.volumes.csi-baremetal.dell.com/types: pod-bound-required                       # here we're specifying to use only one disk for each pod of nginx
            antiaffinity.volumes.csi-baremetal.dell.com/types: pod-label-preferred                  # here we're specifying the type of antiaffinity
            antiaffinity.volumes.csi-baremetal.dell.com/labels: ecs-cluster-ss, ecs-cluster-pvg     # here we're specifying not to use the same disks as for ss and pvg
         labels:
            app: nginx
      spec:
         terminationGracePeriodSeconds: 10
         containers:
            - name: nginx
              image: k8s.gcr.io/nginx-slim:0.8
              ports:
                 - containerPort: 80
                   name: web
              volumeMounts:
                 - name: www-1
                   mountPath: /usr/share/nginx/html.1
                 - name: www-2
                   mountPath: /usr/share/nginx/html.2
   volumeClaimTemplates:
      - metadata:
           name: www-1
        spec:
           accessModes: [ "ReadWriteOnce" ]
           storageClassName: "csi-baremetal-sc-hddlvg"
           resources:
              requests:
                 storage: 1Gi
      - metadata:
           name: www-2
        spec:
           accessModes: [ "ReadWriteOnce" ]
           storageClassName: "csi-baremetal-sc-hddlvg"
           resources:
              requests:
                 storage: 1Gi

```

#### Baremetal CSI Driver's improvements:

At Baremetal CSI Driver we should improve Reservation flow (at reservation controller component) for supporting new type of volumes 
planning (currently we support planning, only depending on capacity). For this we should make the following improvements:
1. Improve AvailableCapacityReservation CRD. For the improvement we should update AvailableCapacityReservation proto message: \
1.1. Currently, it's structure is as follows:
   ```protobuf
   message AvailableCapacityReservation {
      string Namespace = 1;
      string Status = 2;
      NodeRequests NodeRequests = 3;
      repeated ReservationRequest ReservationRequests = 4;
   }
   ```
   where NodeRequests - are requests by nodes, ReservationRequests - requests per volumes. \
1.2. We should update it to support new type of requests: based on affinity annotations:
```protobuf
message AvailableCapacityReservation {
   string Namespace = 1;
   string Status = 2;
   NodeRequests NodeRequests = 3;
   repeated ReservationRequest ReservationRequests = 4;
   AffinityRules AffinityRules = 5;
}

message AffinityRules {
   // affinity requests - filled by scheduler/extender
   repeated AffinityRequests AffinityRequests = 1;
   // antiaffinity requests - filled by scheduler/extender
   repeated AntiaffinityRequests AntiaffinityRequests = 2;
}

message AffinityRequests {
   repeated AffinityRequest AffinityRequest = 1;
}

message AffinityRequest {
   AffinityRequestType Type = 1;
   oneof Request {
      AffinityVolumeBoundRequest AffinityVolumeBoundRequest = 2;
      AffinityDedicatedRequest AffinityDedicatedRequest = 3;
   }
}

enum AffinityRequestType {
   POD_BOUND_REQUIRED = 1;
   POD_BOUND_PREFERRED = 2;
   VOLUME_BOUND_REQUIRED = 3;
   VOLUME_BOUND_PREFERRED = 4;
   DEDICATED_REQUIRED = 5;
   DEDICATED_PREFERRED = 6;
}

message AffinityVolumeBoundRequest {
   repeated VolumesBound VolumesBound = 1;
}

message AffinityDedicatedRequest {
   repeated string Tolerations = 1;
}

message AntiaffinityRequests {
   repeated AntiaffinityRequest AntiaffinityRequest = 1;
}

message AntiaffinityRequest {
   AntiaffinityRequestType Type = 1;
   oneof Request {
      AntiaffinityPodLabelRequest PodLabelRequest = 2;
      AntiaffinityVolumeBoundRequest AntiaffinityVolumeBoundRequest = 3;
   }  
}

enum AntiaffinityRequestType {
   POD_LABEL_REQUIRED = 1;
   POD_LABEL_PREFERRED = 2;
   VOLUME_BOUND_REQUIRED = 3;
   VOLUME_BOUND_PREFERRED = 4;
}

message AntiaffinityPodLabelRequest {
   repeated string Labels = 1;
}

message AntiaffinityVolumeBoundRequest {
   repeated VolumesBound VolumesBound = 1;
}

message VolumesBound {
   repeated string Volumes = 1;
}
```
2. At scheduler extender (in case client set the drive affinity annotations) while creating capacity reservation - fill it with DriveRequests specified in p1.2 above.
3. At reservation controller refactor planning logic:
3.1. Create unified interface for filtering applicable capacities on nodes for volumes planning:
```go
type VolumesPlanFilter interface {
	Filter(ctx context.Context, volumesPlanMap VolumesPlanMap) (filteredVolumesPlanMap VolumesPlanMap, err error)
} 
```
3.2. Implement planning filters for affinity and capacity (currently already implemented in planner.go logic). 
3.3. Affinity planning filter can also be implemented as two separate filters (for affinity and antiaffinity) - as described at ChainOfResponsibility pattern.
Depending on their type (affinity/antiaffinity) perform the corresponding filtering operations (e.g. for antiaffinity pod-label type - select requested pods by labels and filter out drives, used by them)
over ACs in inputted VolumesPlanMap.
3.4. Rename CapacityManager to Manager and iterating logic over planning filters to it. Before iteration - create the initial VolumesPlanMap structure with available AC at requested nodes.
3.5. Dynamically create needed filters (e.g. affinity filter only needed in case if affinity requests were set at DriveRequests) at reservation controller.
