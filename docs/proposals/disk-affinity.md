# Proposal: Supporting disk affinity/anti-affinity feature

Last updated: 28.01.2022

## Abstract

This proposal contains approach for supporting disk affinity/anti-affinity feature, which is crucial for data intensive applications and
which are latency sensitive (e.g. caching services, which are also using some kind of ephemeral storage in case of memory lack).
So for such kind of applications it's sensitive, when they share disk with other applications (especially which are data intensive too).
That's why we need approach to bind data intensive applications to different disks and also in some cases directly dedicate disks for them.

## Background and user stories

Current lvg storageclass cannot mount physical disks to PVs with the awareness of the requesting pods. So, it's possible
for different applications to share the same disk. But in case of data intensive applications, it's necessary to bind them to
different disks and, in some cases, directly dedicate disks for them.
That's why CSI should provide a way to support the following user stories:
* mount PVs for the same pod on the same disk (disk affinity);
* do not mount PVs on the disks that already have some PVs from other specific pods (disk antiaffinity);
* directly dedicate disks for specific pods (dedicated disks).

## Use cases

In the following proposal we will describe how we are going to solve user stories listed above. Basically we will describe the following use cases:
* mount PVs for the same pod on the same disk by setting special annotations on this specific pod;
* do not mount PVs on the disks that already have some PVs from other specific pods (which are specified by pod labels) by setting special annotations on this specific pod;
* directly dedicate disks for specific pods - here we will introduce two approaches:
  * by setting special annotations on the certain pod, followed by automatic dedication of disks by CSI;
  * by manual dedication of disks by setting labels on them, followed by setting special annotation on the certain pods, which should use these dedicated disks.

## Proposal

### Rationale for annotations usage

Usually for supporting affinity/antiaffinity feature - CSI topology feature is applied. But the minimal domain, on which this feature operates, are nodes, not disks.
So we should use a different approach for supporting use cases listed above. This approach, which we introduce, is to use pod annotations for pointing out how this pod/application is going to use disk affinity/antiaffinity.

### Use cases workflow

As we have already described in previous section (Rationale for annotations usage) - we are going to use annotations for use cases supporting.
In order to specify how client is going to use disk/affinity the following pod annotation is introduced:
```yaml
affinity.volumes.csi-baremetal.dell.com/types: ${list of specified affinity types}
antiaffinity.volumes.csi-baremetal.dell.com/types: ${list of specified antiaffinity types}
```
The list of supported **_affinity_** types to use in the above annotation:
* ```pod-bound-(required|preferred)``` - mount PVs for the same pod on the same disk, depending on the suffix - the behaviour is required/preferred;
* ```dedicated-(required|preferred)``` - place volumes on the certain disks, which were dedicated for this application, depending on the suffix - the behaviour is required/preferred.

The list of supported **_antiaffinity_** types to use in the above annotation:
* ```pod-label-(required|preferred)``` - specify pods by their labels, which should not use the same drives as the current one, depending on the suffix - the behaviour is required/preferred;

#### Mount PVs for the same pod on the same disk
For supporting mounting PVs for the same pod on the same disk the following annotation should be specified on the specific pod before its creation:
```yaml
affinity.volumes.csi-baremetal.dell.com/types: <pod-bound-required|pod-bound-preferred>
```

#### Do not mount PVs on the disks that already have some PVs from other specific pods
For ignoring disks, that already have some PVs from other specific pods the following annotation should be specified on our application (pod) before its creation:
```yaml
antiaffinity.volumes.csi-baremetal.dell.com/types: <pod-label-required|pod-label-preferred>
```
Also, the following annotation should be specified on the same pod as well:
```yaml
antiaffinity.volumes.csi-baremetal.dell.com/labels: ${pod-label1}:${pod-namespace1},...,${pod-labelN}:${pod-namespaceM}
```
In the above annotation we should specify labels of specific pods, whose disks in usage we do not want to consume by PVs of our application (pod).

#### Directly dedicate disks for specific pods
Generally, we have two options to support directly dedicated disks (which one is preferable is the space for discussion):
* Set the following annotation on the certain pod before its creation to request dedicated disks for it:
  ```yaml
  affinity.volumes.csi-baremetal.dell.com/dedicated: true
  ```
  Based on this annotation, disk (more precisely: corresponding AvailableCapacity CR) will be annotated automatically (during capacity reservation process) by CSI by the following label:
  ```yaml
  affinity.csi-baremetal.dell.com/taint: ${pod-label}:${pod-namespace}
  ```
  When disk will be annotated - no PVs of applications, whose labels are different will be mounted on this disk.
* Set the following label on the preferred AvailableCapacity CR manually:
  ```yaml
  affinity.csi-baremetal.dell.com/taint: ${value}
  ```
  Then set the following annotation on the certain pod before its creation:
  ```yaml
  affinity.volumes.csi-baremetal.dell.com/tolerations: ${value} - set the same value, which was used while labeling the required AvailableCapacity.
  ```
It is worth noticing, that first approach is good enough about disk dedication for only one application.
But if we need to dedicate disk for several predefined applications - it will not work as expected.

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
            affinity.volumes.csi-baremetal.dell.com/types: pod-bound-required                                              # here we're specifying to use only one disk for each pod of nginx
            antiaffinity.volumes.csi-baremetal.dell.com/types: pod-label-preferred                                         # here we're specifying the type of antiaffinity
            antiaffinity.volumes.csi-baremetal.dell.com/labels: app=ecs-cluster-ss:default,app=ecs-cluster-pvg:default     # here we're specifying not to use the same disks as for ss and pvg
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

Let's consider another example, this time about dedicated disks:
* We have node with 2 SSDs and X HDDs;
* Application 1 with 1 PVCs on each pod wants to have dedicated SSD using SSD LVG SC.
* X other applications to use generic ephemeral volumes on remaining SSD.

For satisfying this case let's use the first of considered approaches, which we suggested above. So, let's annotate the first
application with the corresponding label (```affinity.volumes.csi-baremetal.dell.com/dedicated```):
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
        affinity.volumes.csi-baremetal.dell.com/dedicated: "true"
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
  volumeClaimTemplates:
    - metadata:
        name: www-1
      spec:
        accessModes: [ "ReadWriteOnce" ]
        storageClassName: "csi-baremetal-sc-ssdlvg"
        resources:
          requests:
            storage: 1Gi
```
As we have already discussed: after some time, the selected AvailableCapacity resources will be annotated as dedicated for this pod:
```yaml
affinity.csi-baremetal.dell.com/taint: app=nginx:default
```
No other applications will not be scheduled on this disk.
The declaration of remaining applications will be just as regular ones: specify generic ephemeral volumes at pod spec with SSD LVG SC and CSI driver will deploy it
as regular, but will filter out the dedicated earlier SSD disk.

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
      AffinityDedicatedRequest AffinityDedicatedRequest = 3;
   }
}

enum AffinityRequestType {
   POD_BOUND_REQUIRED = 1;
   POD_BOUND_PREFERRED = 2;
   DEDICATED_REQUIRED = 5;
   DEDICATED_PREFERRED = 6;
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
   }  
}

enum AntiaffinityRequestType {
   POD_LABEL_REQUIRED = 1;
   POD_LABEL_PREFERRED = 2;
}

message AntiaffinityPodLabelRequest {
   repeated string Labels = 1;
}
```
2. At scheduler extender (in case client set the drive affinity annotations) while creating capacity reservation - fill it with AffinityRules specified in p1.2 above.
3. At reservation controller refactor planning logic:
   3.1. Create unified interface for filtering applicable capacities on nodes for volumes planning:
```go
type VolumesPlanFilter interface {
Filter(ctx context.Context, volumesPlanMap VolumesPlanMap) (filteredVolumesPlanMap VolumesPlanMap, err error)
} 
```
3.2. Implement planning filters for affinity and capacity (currently already implemented in planner.go logic). \
3.3. Affinity planning filter can also be implemented as two separate filters (for affinity and antiaffinity) - as described at ChainOfResponsibility pattern.
Depending on their type (affinity/antiaffinity) perform the corresponding filtering operations (e.g. for antiaffinity pod-label type - select requested pods by labels and filter out drives, used by them)
over ACs in inputted VolumesPlanMap. \
3.4. Rename CapacityManager to Manager and iterating logic over planning filters to it. Before iteration - create the initial VolumesPlanMap structure with available AC at requested nodes. \
3.5. Dynamically create needed filters (e.g. affinity filter only needed in case if affinity requests were set at AffinityRules) at reservation controller.

#### Future planning

* Framework based scheduler - in future it's planned to replace scheduler extender with framework based scheduler. \
  In case of framework based scheduler it will be no need it AvailableCapacityReservation CRD. All reservations will be stored in the cache of a leader plugin instance. \
  In general there will be 3 stages:
  * Filter stage. Watches for the list of ACs and filter out those nodes which don't have resources to satisfy PVCs.
  * Score stage. Assign scores for different nodes - node with less capacity consumed must have the highest score to provide balance across the nodes.
  * Reserve stage. Tries to reserve specific ACs in the cache. If they busy - fail operation and return to the Filter stage. \
    Disk affinity feature will work at Filter and Reserve stage. The difference with scheduler extender approach will consist in that, there will be only p3 described above (no need in p2). \
    Also, as there will be no AvailableCapacityReservation CRD, we will need to mark each PVC with annotation with reserved AC to it.
    So at controller side we will know which AC to use while creating the volume.
* Storage capacity feature - in future it's planned to use storage capacity feature to simplify reservation logic and try to get rid of scheduler extender. \
  In case of storage capacity usage - we can get rid of resources calculating and using this link of the filtering chain (look above for more information).
  But we cannot get rid of scheduler extender/framework based scheduler completely for disk affinity feature usage, or we should write out reservation hooks by ourselves.
  It is because of requirements nature: we should make a reserving decision by application factors (e.g. pod-label disk antiaffinity type).
  So, basically there are following options:
  * Use scheduler extender/framework based scheduler with storage capacity feature as we are doing currently (and discussed above in this proposal), but without resources calculations (calculations will be done while reporting the capacity as described in CSIStorageCapacity CRD).
  * Use custom hooks for volume reservation and so do not use scheduler extender/framework based scheduler. This hook will be called by controller while CreateVolume API will be called.
    This approach has its disadvantages because scheduler will have already scheduled the deployment at specific node - so a lot reschedule operations will be possible.
    Also, there appear synchronous calls to this hook - which is an anti-pattern for controller/operator cases.
  * Just make reservation at controller side while CreateVolume API will be called. This approach is similar to the previous but has no synchronous calls.
    But instead it puts extra burden to controller, which should not be there. \
    So, from them above options, remaining of scheduler extender/framework based scheduler looks more perspective and correct.
