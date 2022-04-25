[This is a template for CSI-Baremetal's changes proposal. ]
# Proposal: Lifecycle of a CRD (CustomResourceDefinition)

Last updated: 20.04.22


## Abstract

### Custom Resource Definitions (CRDs)

Kubernetes provides a mechanism for declaring new types of Kubernetes objects. Using CustomResourceDefinitions (CRDs), Kubernetes developers can declare custom resource types.

In Helm 3, CRDs are treated as a special kind of object. They are installed before the rest of the chart, and are subject to some limitations.

CRD YAML files should be placed in the crds/ directory inside of a chart. Multiple CRDs (separated by YAML start and end markers) may be placed in the same file. Helm will attempt to load all of the files in the CRD directory into Kubernetes.

CRD files cannot be templated. They must be plain YAML documents.

When Helm installs a new chart, it will upload the CRDs, pause until the CRDs are made available by the API server, and then start the template engine, render the rest of the chart, and upload it to Kubernetes. Because of this ordering, CRD information is available in the .Capabilities object in Helm templates, and Helm templates may create new instances of objects that were declared in CRDs.

### Limitations on CRDs
Unlike most objects in Kubernetes, CRDs are installed globally. For that reason, Helm takes a very cautious approach in managing CRDs. CRDs are subject to the following limitations:

CRDs are never reinstalled. If Helm determines that the CRDs in the crds/ directory are already present (regardless of version), Helm will not attempt to install or upgrade.
CRDs are never installed on upgrade or rollback. Helm will only create CRDs on installation operations.
CRDs are never deleted. Deleting a CRD automatically deletes all of the CRD's contents across all namespaces in the cluster. Consequently, Helm will not delete CRDs.

**Operators who want to upgrade or delete CRDs are encouraged to do this manually and with great care**.

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
        image: "{{ .Values.hooks.registry }}/{{.Values.hooks.repository }}:{{ .Values.hooks.tag }}"
        command: ["kubectl", "-n", "{{ .Release.Namespace }}", "patch", "<crd>", "-p", '-f <crd filename.yaml']
```

It is possible to define policies that determine when to delete corresponding hook resources. This will be helpful to choose `hook-succeeded` or `hook-failed`
Deployment process must involve a special image with new version of CRDs, those image built at release step and integrated in the CSI upgrade routine. 


## Open issues (if applicable)

| ID      | Name | Descriptions | Status | Comments |
|---------|------|--------------|--------|----------|
| ISSUE-1 | Deliver CRD to container     | Need to test how we pass in the filename for the crd for the patch              | Open |          |   
