# Proposal: Deploy CSI on K3s cluster 

Last updated: 8.04.22


## Abstract

Deploy CSI on K3s cluster, test it work and add steps for manual patching scheduler 

## Background

Currently csi is compatible with Vanilla Kubernetes, RKE2, OpenShift and Kind, but only for test purpose. We want to deploy csi to new platform.  

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
For passing it to k3s server component use `--kube-scheduler-arg=config`


## Compatibility

There is no problem with compatibility

## Implementation

Tasks:
1.	Create new config for scheduler 
2.	Pass it to the k3s application server with appropriate flag
3.	Reload daemon 
4.	Test with temp pod


## Open issues (if applicable)

| ID      | Name | Descriptions | Status | Comments |
|---------|------|--------------|--------|----------|
| ISSUE-1 | Update installation guide | Add steps for deploying csi on k3s |   |   |
