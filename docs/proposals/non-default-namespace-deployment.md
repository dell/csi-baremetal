# Proposal: Supporting non default namespace CSI deployment at Openshift 

Last updated: 09.12.2021

## Abstract

This proposal contains approaches for supporting CSI deployment in non default namespace at Openshift platform, 
introducing Privileged SCC at CSI ServiceAccounts.`

## Background

In case of CSI deployment at Openshift platform in non default namespace, CSI pods have Restricted SCC by default.   
But restricted denies access to all host features and resources like disk. Must be Privileged SCC because it allows
access to all privileged and host features and resources.
For give privileged SCC to custom user, the following command need to be executed:
`oc adm policy add-scc-to-user privileged -z <user name> -n <namespace>`
And so, we should give this scc for CSI's service accounts, that need it: 
 * csi-node-sa;
 * csi-baremetal-extender-sa.

## Proposal

In basic there are two approaches that allows us to support this feature:
1. Move logic of creation csi's sa to csi-baremetal-operator and in case of deployment to Openshift platform - give the
   necessary permissions to the corresponding csi's service accounts as well.
2. Create separate job, which will be executed as post-install helm hook only for Openshift platform, which will give the
   necessary permissions to the corresponding csi's service accounts.

## Compatibility

For the second approach we are binding directly to helm lifecycle management mechanisms, and there may be the problem 
in case of possible supporting other configuration managers such as kustomize if there will be such requirement.

## Implementation

#### Necessary permissions for scc granting service account (operator's one or helm's post-install hook)
1. In order to give permissions to certain service accounts, the granting user/service account should have been bound 
   with the following role and rolebinding:
```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
   name: role
   namespace: test-csi
rules:
   - apiGroups:
        - "rbac.authorization.k8s.io"
     resources:
        - rolebindings
        - roles
     verbs:
        - get
        - create
        - update
   - apiGroups:
        - "security.openshift.io"
     resourceNames:
        - privileged
     resources:
        - securitycontextconstraints
     verbs:
        - use
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
   name: rolebinding
   namespace: test-csi
roleRef:
   apiGroup: rbac.authorization.k8s.io
   kind: Role
   name: role
subjects:
   - kind: ServiceAccount
     name: assignment
     namespace: test-csi
```
2. The corresponding role should be persisted at helm charts and be bound dynamically to csi-node-sa and 
   csi-baremetal-extender-sa service accounts right after their creation:
```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
   name: role-scc
   namespace: test-csi
rules:
   - apiGroups:
        - "security.openshift.io"
     resourceNames:
        - privileged
     resources:
        - securitycontextconstraints
     verbs:
        - use
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