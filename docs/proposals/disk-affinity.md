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
affinity.drives.csi-baremetal.dell.com/type: <pod-bound> - mount PVs for the same pod on the same disk;
antiaffinity.drives.csi-baremetal.dell.com/type: <pod-label> - specify disk antiaffinity type: currently there is only one option - by pod labels;  
antiaffinity.drives.csi-baremetal.dell.com/labels: "${list of the labels}" - do not mount PVs on the disks that already have some PVs from other specific pods (specified by labels).

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
            affinity.drives.csi-baremetal.dell.com/type: pod-bound                                # here we're specifying to use only one disk for each pod of nginx
            antiaffinity.drives.csi-baremetal.dell.com/type: pod-label                            # here we're specifying the type of antiaffinity
            antiaffinity.drives.csi-baremetal.dell.com/labels: ecs-cluster-ss, ecs-cluster-pvg    # here we're specifying not to use the same disks as for ss and pvg 
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

At Baremetal CSI Driver we should improve Reservation flow (at ReservationController component) for supporting new type of volumes 
planning (currently we support planning, only depending on capacity). For this we should make the following improvements:
1. Improve AvailableCapacityReservation CRD. For the improvement we should update AvailableCapacityReservation proto message:
1.1. Currently, it's structure is as follows:
```protobuf
message AvailableCapacityReservation {
   string Namespace = 1;
   string Status = 2;
   NodeRequests NodeRequests = 3;
   repeated ReservationRequest ReservationRequests = 4;
}
```
where NodeRequests - are requests by nodes, ReservationRequests - requests per volumes. 
1.2. We should update it to support new type of requests: based on affinity annotations:
```protobuf
message AvailableCapacityReservation {
   string Namespace = 1;
   string Status = 2;
   NodeRequests NodeRequests = 3;
   repeated ReservationRequest ReservationRequests = 4;
   DriveRequests DriveRequests = 5;
}

message DriveRequests {
   // affinity requests - filled by scheduler/extender
   repeated DriveAffinityRequests DriveAffinityRequests = 1;
   // antiaffinity requests - filled by scheduler/extender
   repeated DriveAntiaffinityRequests DriveAntiaffinityRequests = 2; 
}

message DriveAffinityRequests {
   repeated DriveAffinityRequest DriveAffinityRequest = 1;
}

message DriveAffinityRequest {
   DriveAffinityRequestType Type = 1;
}

enum DriveAffinityRequestType {
   POD_BOUND = 1;
} 

message DriveAntiaffinityRequests {
   repeated DriveAntiaffinityRequest DriveAntiaffinityRequest = 1;
}

message DriveAntiaffinityRequest {
   DriveAntiaffinityRequestType Type = 1;
   oneof Request {
      DriveAntiaffinityPodLabelRequest PodLabelRequest = 2;
   }  
}

enum DriveAntiaffinityRequestType {
   POD_LABEL = 1;
}

message DriveAntiaffinityPodLabelRequest {
  repeated string Labels = 1;
}
```
















#### Moving creation csi's sa logic to csi-baremetal-operator
1. Currently, service accounts are persisting at helm charts. In the considering approach csi's service accounts should be
   created:
   1. csi-node-sa: right before node daemonset creation (https://github.com/dell/csi-baremetal-operator/blob/master/pkg/node/node.go#L38);
   2. csi-baremetal-extender-sa: right before scheduler extender creation (https://github.com/dell/csi-baremetal-operator/blob/master/pkg/scheduler_extender.go#L38);
   3. other service accounts should be created by the analogue.
2. Right after csi-node-sa/csi-baremetal-extender-sa creation, operator should create additional rolebindings as described
   in the previous section and also create the current needed ones.
3. After the above preparations daemonsets can be created like before. 

#### Implementing separate helm post-install hook
1. For separate post-install hook creation we should implement the separate component (e.g. _csi-postconfigurator_), which will be run as a separate job
   via helm post-install hook right after all k8s resources creation. Currently this component will run only for Openshift platform and
   will only create described above rolebindings for _csi-node-sa_ and _csi-baremetal-extender-sa_ service accounts.
2. As post-install hook will be the separate component it should have the separate service account, only bounded to following
   resources: rolebindings, roles, securitycontextconstraints in deployed namespace.
3. Helm charts should be prepared for described component.  

#### Pros/Cons
There are several pros/cons in each approach. We will consider them further.

##### _Moving creation csi's sa logic to csi-baremetal-operator:_
_Pros:_
1. Simple and fast implementing solution.
2. This solution will hide any platform dependent post configurations from the customer.

_Cons:_
1. This approach make code base more complex introducing more platform dependent code to it.
2. Operator's service account needs permission's scope extension to support scc.

##### _Implementing separate helm post-install hook:_
_Pros:_
1. This solution needs only restricted scope of logic: only create the additional rolebindings to support scc. 
2. This component's service account only needs restricted scope of permissions by using only rolebindings, roles, 
   securitycontextconstraints resources in the necessary namespace.

_Cons:_
1. Due to the possible time needed to deploy this component after helm post-installation, pods, that needs the scc permissions 
   will fail to deploy while the corresponding rolebindings will not be created. Due to the deployment retry backoff, 
   overall time, needed to deploy pods will be increased, which may affect deployment requirements.
2. For this approach we are binding directly to helm lifecycle management mechanisms, and there may be the problem
   in case of possible supporting other configuration managers such as kustomize if there will be such requirement.

##### Considerations
Due to possible customer impact the second approach will have more possible ambiguities at deployment process.
Currently, the first approach looks more transparent and simple for implementation but the described approach for postconfiguration
may be useful later on.