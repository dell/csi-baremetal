# Proposal: Lifecycle of a CRD (CustomResourceDefinition)

Last updated: 4.05.22


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

During helm upgrade upgrade the new CRDs will be applied manually with a shell command running `kubectl patch ...` of the crd.

Currently we're researching upgrade a minor chart version change (like from v1.0.x to v1.1.x) indicates that there is no incompatible breaking change needing.

## Implementation

```helm upgrade [RELEASE_NAME] [CHART] --install```

See [helm upgrade](https://helm.sh/docs/helm/helm_upgrade/) for command documentation.

Upgrading an existing Release to a new minor version (like from v1.0.x to v1.1.x):

* Note about Upgrade
  > There is no support at this time for upgrading or deleting CRDs using [Helm](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/).

  In order to upgrade CRD use `kubectl patch crd -p <crd resource>` command. chart crd's must be downloaded from the actual source.

  ```bash
  export CSI_OPERATOR_VERSION=v1.1.0
  # this is an example how to download charts from remote registry
  export ARTIFACTORY_SOURCE_PATH="http://artifactory/" 
  wget "$ARTIRACTRY_SOURCE_PATH/$CSI_OPERATOR_VERSION/csi-baremetal-operator-$CSI_OPERATOR_VERSION.tgz"
  tar -xzvf csi-baremetal-operator-$CSI_OPERATOR_VERSION.tgz
  kubectl patch crd -p csi-baremetal-operator/crds/
  ```

## Open issues (if applicable)

| ID      | Name | Descriptions | Status | Comments |
|---------|------|--------------|--------|----------|
| ISSUE-1 | Deliver CRD      |         | Open | How to update CRD if no there is no access to chart files |   
