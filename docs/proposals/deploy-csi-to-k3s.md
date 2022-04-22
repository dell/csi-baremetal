# Proposal:  K3s Kubernetes distribution 

Last updated: 22.04.22


## Abstract

Specify required changes to support working CSI on k3s, describe steps for manual patching scheduler and for automated patching.  

## Background

Currently csi is compatible with Vanilla Kubernetes, RKE2, OpenShift and Kind. However, Kind is only used for testing purposes. We want to support new platform.  

## Proposal

Need to create next config file in directory `/var/lib/rancher/k3s/agent/pod-manifests`:
```
apiVersion: kubescheduler.config.k8s.io/v1beta1
kind: KubeSchedulerConfiguration
extenders:
  - urlPrefix: "http://127.0.0.1:8889"
    filterVerb: filter
    prioritizeVerb: prioritize
    weight: 1
    enableHTTPS: false
    nodeCacheCapable: false
    ignorable: true
    httpTimeout: 15s
leaderElection:
  leaderElect: true
clientConnection:
  kubeconfig: /var/lib/rancher/k3s/server/cred/scheduler.kubeconfig
```
After that there are two ways:
1. If you haven't started the service yet, run service creation command with the following flag `k3s server --kube-scheduler-arg=config=/var/lib/rancher/k3s/agent/pod-manifests/scheduler/config.yaml`
2. If you already have a running service, modify  `/etc/systemd/system/k3s.service`. Add at the option `ExecStart` (end of the file) next string `--kube-scheduler-arg=config=/var/lib/rancher/k3s/agent/pod-manifests/scheduler/config.yaml`. Service unit will look like this:
```  
[Service]
. . .
ExecStartPre=/bin/sh -xc '! /usr/bin/systemctl is-enabled --quiet nm-cloud-setup.service'
ExecStartPre=-/sbin/modprobe br_netfilter
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/k3s start \  
    --kube-scheduler-arg=config=/var/lib/rancher/k3s/agent/pod-manifests/scheduler/config.yaml 
```
After that manually run the command for reloading daemon `systemctl daemon-reload && systemctl restart k3s`. Don't worry about installation `systemctl` because k3s doesn't work without that and install it by default.   

It's suggested to use the second method to automatically patch scheduller. We will pass `k3s.service` to patcher, change `ExecStart` parameter and remember what we add. We cann't save the state of the entire file, because this file refers to the operation of the entire service (components inscribed in it). And how this file is changed by us, it can also be changed by other people. After that user used to reload systemd manager configuration and k3s service manually.  

## Rationale

1. First method will modify the default service script with the required configuration file and start the service. If the service is already running this command will not be able to modify it. It will try to run another one in parallel with the default configuration and run into the problem of running two servers on the same host.
2. Second method is suitable for a situation with a running service. The problem of the method is the lack of automatic reload. 

## Compatibility

There is no problem with compatibility

## Implementation

Tasks for manuall patching:
1.	Create new config for scheduler 
2.	Pass it to the k3s application server with appropriate flag
3.	Reload daemon and restart k3s with new kube scheduller parameters manually 
4.	Test with temp pod
5.  For uninstall CSI need to change k3s.service file back and reload as it was in a priviouse step 

Tasks for automated patching:
1.  Need to set parameter `scheduler.patcher.enable=true` in installation step
2.  Reload daemon and restart k3s
3.  Check scheduller patcher redines with test pod
4.  Uninstall CSI without changes  

## Open issues (if applicable)

| ID      | Name | Descriptions | Status | Comments |
|---------|------|--------------|--------|----------|
| ISSUE-1 | Update installation guide | Add steps for deploying csi on k3s |   |   |
| ISSUE-2 | Parse .service file | Add function(s) to change k3s.service file  |   |   |
| ISSUE-3 | Test scheduller patcher | Test your changes  |   |   |