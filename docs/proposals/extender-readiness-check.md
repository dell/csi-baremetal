# Proposal: Scheduler-extender readiness check

Last updated: 08.07.2021


## Abstract
CSI has no way to verify scheduler-extender deployment status.
The goal - create an opportunity for user to understand: is scheduler-extender installed completely and able to work or not.

## Background
Currently, there are 2 patching methods depends on platforms.

- For rke/vanilla kubernetes:
1. Deploy configuration configmap
2. Deploy csi-baremetal-patcher daemonset.
In patcher:
   - Copy configs from cm to local files on host
   - Update kubernetes-scheduler manifest
    
- For Openshift
1. Deploy specific scheduler configmap
2. Update Scheduler recourse

In the both cases patching triggers kube-scheduler restarting process. 
If kube-scheduler is not restarted, a custom scheduler-extender might not work correctly.
Restart check is implemented in E2E testing, but it's need to provide this information to user.

## Proposal
First step - implement CSI CR status field (ready/unready), which is updated in Operator.
Secondly there are some options to deliver extender deployment state:

#### Approach 1
1. Add check in Operator after both patching methods to wait kube-scheduler restart before move to ready state.
   
To trigger restart if config stacked:
2. For Openshift - recreate scheduling configmap after CSI installation
3. For Patcher - add kube-scheduler pod deletion step on first try (maybe only if not changed?)

Disadvantages:
- kube-scheduler deletion action may lead to negative consequences

#### Approach 2
For Openshift - same as in the approach 1

For Patcher: 
1. Move kube-scheduler restart check to patcher code and integrate it with readiness check.
   - if configs are changed, it checks restart and becomes ready after
   - if configs doesn't need patching, it becomes ready at once

Disadvantages:
- Patcher uses Python language, it's technically complicated to rewrite code for it
- We need to have code for checking restart in 2 places

#### Approach 3
1. Add check based only on patching state and not try to watch on kube-scheduler

For Openshift - become ready if configmap is actual and Scheduler is updated

For Patcher - become ready if all patcher-daemonset pods are running and their count equals to kube-schedulers count (control-plane nodes may not provide master labels and pods won't be created at all) 

## Rationale

Implement readiness check in scheduler-extender.

Problems:
- extender doesn't really know about patching process and can't check status itself
- need to create separate API for each extender pod (a lot of additional Kubernetes API calls)

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
