[This is a template for CSI-Baremetal's changes proposal. ]
# Proposal: Lifecycle of a CRD (CustomResourceDefinition)

Last updated: 20.04.22


## Abstract

Helm is unable to handle the lifecycle of a CustomResourceDefinition.
If CRD was already installed helm will not upgrade it with plain `helm install\upgrade` routine.
We need a mechanism for CRD upgrades.

## Background

Helm provides a hook mechanism to allow chart developers to intervene at certain points in a release's life cycle.
Hooks work like regular templates, but they have special annotations that cause Helm to utilize them differently.
The main focus of these proposal is to turn the CRD into a managed part of the chart and not just an item that is added at install time and then forgotten.
If the CRD was managed as part of the chart it would fix a lot of confusion **realted to CRDs not being updated when there are changes** and not being deleted with the chart

## Proposal

In this proposal we're research a way to use [`helm` chart hooks](https://helm.sh/docs/topics/charts_hooks/) in order to update CRDs.

## Rationale

Pros:

 - CRDs managed by helm
 - CRDs can be installed/updated at any point in the charts lifecycle

Cons:

 - Issues with hooks are harder for a user to detect/debug
 - The developer needs to have a good understanding of how all the helm hooks work


## Compatibility

Currently we're researching upgrade from 1.0.x to 1.1.x.

## Implementation

Hooks are just Kubernetes manifest files with special annotations in the metadata section. Because they are template files, you can use all of the normal template features, including reading `.Values`, `.Release`, and `.Template`.

 - `pre-upgrade` - 	Executes on an upgrade request after templates are rendered, but before any resources are updated

This template for example, stored in `templates/pre-upgrade-job.yaml`, declares a job to be run on pre-upgrade

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: "{{ .Release.Name }}"
  labels:
    app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
    app.kubernetes.io/instance: {{ .Release.Name | quote }}
    app.kubernetes.io/version: {{ .Chart.AppVersion }}
    helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version }}"
  annotations:
    # This is what defines this resource as a hook. Without this line, the
    # job is considered part of the release.
    "helm.sh/hook": pre-upgrade
    # "helm.sh/hook-weight": "-5"
    "helm.sh/hook-delete-policy": hook-succeeded,hook-failed
spec:
  template:
    metadata:
      name: "{{ .Release.Name }}"
      labels:
        app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
        app.kubernetes.io/instance: {{ .Release.Name | quote }}
        helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version }}"
    spec:
      restartPolicy: Never
      containers:
      - name: pre-upgrade-job
        image: "alpine:3.3"
        command: ["/bin/sleep","{{ default "10" .Values.sleepyTime }}"]
```

It is possible to define policies that determine when to delete corresponding hook resources. This will be helpful to choose `hook-succeeded` or `hook-failed`

## Open issues (if applicable)

| ID      | Name | Descriptions | Status | Comments |
|---------|------|--------------|--------|----------|
| ISSUE-1 |      |              |        |          |   
