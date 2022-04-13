# Proposal:  K3s Kubernetes distribution 

Last updated: 12.04.22


## Abstract

Specify required changes to support working CSI on k3s, describe steps for manual patching scheduler 

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
2. If you already have a running service, modify  `/etc/systemd/system/k3s.service`. Add at the end of the file `--kube-scheduler-arg=config=/var/lib/rancher/k3s/agent/pod-manifests/scheduler/config.yaml` and manually run the command for reloading daemon `systemctl daemon-reload && systemctl restart k3s`.

## Rationale

1. First method will modify the default service script with the required configuration file and start the service. If the service is already running this command will not be able to modify it. It will try to run another one in parallel with the default configuration and run into the problem of running two servers on the same host.
2. Second method is suitable for a situation with a running service. The problem of the method is the lack of automatic reload. 

## Compatibility

There is no problem with compatibility

## Implementation

Tasks:
1.	Create new config for scheduler 
2.	Pass it to the k3s application server with appropriate flag
3.	Reload daemon 
4.	Test with temp pod
5.  Uninstall CSI


## Open issues (if applicable)

| ID      | Name | Descriptions | Status | Comments |
|---------|------|--------------|--------|----------|
| ISSUE-1 | Update installation guide | Add steps for deploying csi on k3s |   |   |
