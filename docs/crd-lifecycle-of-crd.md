# Proposal: Lifecycle of a CRD (CustomResourceDefinition)

Last updated: 13.05.22


## Abstract

### Custom Resource Definitions (CRDs)

Kubernetes provides a mechanism for declaring new types of Kubernetes objects. Using CustomResourceDefinitions (CRDs), Kubernetes developers can declare custom resource types.

In Helm 3, CRDs are treated as a special kind of object. They are installed before the rest of the chart, and are subject to some limitations.

CRD YAML files should be placed in the crds/ directory inside of a chart. Helm will attempt to load all of the files in the CRD directory into Kubernetes.

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
If the CRD was managed as part of the chart it would fix a lot of confusion **related to CRDs not being updated when there are changes** and not being deleted with the chart

## Proposal

Note about upgrading an existing Release to a new minor version (like from v1.0.x to v1.1.x).

The command `helm upgrade ...` MUST trigger a hook with a special docker container for patching CRDs.
MUST have a way to build a new version by running `REGISTRY=docker-registry-goes-here make build-pre-upgrade-crds-image`.
The build image MUST contains `kubectl` executable in order to perform CRDs pathing.
Customize kubectl image with `KUBECTL_IMAGE=bitnami/kubectl:1.23.6 REGISTRY=docker-registry-goes-here make build-pre-upgrade-crds-image`

In high level design the plain use case is next:

1. The command `helm upgrade ...` will trigger a hook.
2. Hook will create all resources.
3. Hook complete replacing.
4. Hook perform cleanup after upgrade.

All related information is present in the [`csi-baremetal-operator`](https://github.com/dell/csi-baremetal-operator#upgrade-process) repository. 

## Implementation

In order to deliver new CRDs, next things MUST be implemented:

* A hook template with all resources and the job [pre-upgrade-crds.yaml](https://github.com/dell/csi-baremetal-operator/blob/master/charts/csi-baremetal-operator/templates/pre-upgrade-crds.yaml)
* A new make target for build container with `kubectl` and CRDs files.
* A new make target for push container this container to remote docker registry.

## Open issues (if applicable)

| ID      | Name | Descriptions | Status | Comments |
|---------|------|--------------|--------|----------|
|  |     |         |  |  |   
