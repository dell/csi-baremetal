# Bare-metal CSI Plugin Contribution Guide

## Workflow overview

### Issues and Pull requests

#### Issue
Before you file an issue, make sure you've checked the following for existing issues:
    - Before you create a new issue, please do a search in [open issues](https://github.com/dell/csi-baremetal/issues) to see if the issue or feature request has already been filed.
    - If you find your issue already exists, make relevant comments and add your [reaction](https://github.com/blog/2119-add-reaction-to-pull-requests-issues-and-comments). Use a reaction:
        - 👍 up-vote
        - 👎 down-vote
#### You can submit the following issues:

- ***Bug***: You've found a bug with the code, and want to report it, or create an issue to track the bug.
- ***Enhancement***: New feature or request.
- ***Proposal***: Used for items that propose a new idea or functionality. This allows feedback from others before code is written.
- ***Discussion***: You have something on your mind, which requires input form others in a discussion, before it eventually manifests as a proposal.
- ***Question***: Use this issue type, if you need help or have a question.

### Pull Requests

All contributions come through pull requests. To submit a proposed change, we recommend following this workflow:

1. Make sure there's an issue (bug or enhancement) raised, which sets the expectations for the contribution you are about to make.
2. Fork the relevant repo and create a new branch
   - The branch name should be **bugfix** or **feature** based on the issue type: ```feature/bugfix-<Issue ID>-<short descriptionj```
3. Create your change
    - Code changes require tests (Unit and Kubernetes e2e)
4. Update relevant documentation for the change
5. Commit and open a PR with title "[ISSUE-ID] <short description>"
6. Fill "Purpose" of Pull Request Template:
    - Add detailed description of the issue/feature and how it was solved/developed to simplify review process
    - Make actions from PR Checklist
7. Choose label for your Pull Request:
    - "Enhancement" - for PRs with feature or some enhancement
    - "Bug" - for PRs with bugfix    
    - "Documentation" - for PRs with something that require documentation update
8. Make sure that unit and e2e tests are passed
9. A maintainer of the project will be assigned, and you can expect a review within a few days
10. Merge commit title with "[ISSUE-ID] <short description>"

#### Use draft PRs for early feedback

A good way to communicate before investing too much time is to create a draft PR and share it with your reviewers. The standard way of doing this is to click on "Create Draft Pull Request". This will let people looking at your PR know that it is not well baked yet.

### Code style
Bare-metal CSI Plugin is written in Golang. Our plugin uses [Effective Go](https://golang.org/doc/effective_go.html) .
#### Imports
- Imports statement should be divided into 4 blocks each block is separated from others by empty line.
  * First block - only imports from standard library. 
  * Second block - external libraries imports.
  * Third block - our internal imports that don't relate to that repository (csi-baremetal).
  * Forth block - internal imports that relates to that repository (csi-baremetal).
- If there are no imports from some block, that block should be omitted.
- If some structure have a field with logger, that field should be the last in the structure declaration.
#### Linter
For auto-detecting code style inconsistencies we use [golangci-lint](https://github.com/golangci/golangci-lint).
Run `make lint` if you want to check your changes.
#### Comments
  Ensure that your code has reasonable comments. 
  * For functions: 
    ```
    // Name of the method, its purpose
    // Description of specific input parameters
    // Description of returned values
    ```
  * For structs, interfaces, constants, variables:
    ```
    // Name of the struct/interface/constant/variable , its purpose
    ```
  * At least one file in package should have package comment like:
    ```
    // Package "package name" ...
    ```
#### Dependency management
We use Go modules to manage dependencies on external packages.

To add or update a new dependency, use the go get command:

##### Pick the latest tagged release.
```
go get example.com/some/module/pkg
```

##### Pick a specific version.
```
go get example.com/some/module/pkg@vX.Y.Z
```
Tidy up the go.mod and go.sum files and copy the new/updated dependency to the vendor/ directory:
```
go mod tidy
```

You have to commit the changes to go.mod, go.sum and before submitting the pull request.

### Preparing Build Environment

#### Use devkit

[This guide is intended](devkit/README.md) as a quickstart on how to use devkit for CSI development env. 

#### Requirements
- go v1.16
- lvm2 packet installed on host machine
- kubectl v1.16+
- helm v3

#### Build

##### Set Variables

```
export REGISTRY=<your_docker_hub>
export CSI_BAREMETAL_DIR=<full_path_to_csi_baremetal_src>
export CSI_BAREMETAL_OPERATOR_DIR=<full_path_to_csi_baremetal_operator_src>
```

##### Build csi-baremetal

```
cd ${CSI_BAREMETAL_DIR}
export CSI_VERSION=`make version`

# Get dependencies
make dependency

# Compile proto files
make install-compile-proto

# Generate CRD
make install-controller-gen
make generate-deepcopy

# Run unit tests
make test

# Run sanity tests
make test-sanity

# Run linting
make lint

# Clean previous artefacts
make clean

# Build binary
make build
make DRIVE_MANAGER_TYPE=loopbackmgr build-drivemgr

# Build docker images
make download-grpc-health-probe
make images REGISTRY=${REGISTRY}
make DRIVE_MANAGER_TYPE=loopbackmgr image-drivemgr REGISTRY=${REGISTRY}
```

##### Build csi-baremetal-operator

```
cd ${CSI_BAREMETAL_OPERATOR_DIR}
export CSI_OPERATOR_VERSION=`make version`

# Run unit tests
make test

# Run linting
make lint

# Build docker image
make docker-build REGISTRY=${REGISTRY}
```

##### Prepare kind cluster

```
cd ${CSI_BAREMETAL_DIR}

# Build custom kind binary
make kind-build

# Create kind cluster
make kind-create-cluster

# Check cluster
kubectl cluster-info --context kind-kind

# If you use another path for kubeconfig or don't set KUBECONFIG env
kind get kubeconfig > <path_to_kubeconfig>

# Prepare sidecars 
# If you have sidecar images pushed into your registary
make kind-pull-sidecar-images
# If you have no ones
make deps-docker-pull
make deps-docker-tag

# Retag CSI images and load them to kind
make kind-tag-images TAG=${CSI_VERSION} REGISTRY=${REGISTRY}
make kind-load-images TAG=${CSI_VERSION} REGISTRY=${REGISTRY}
make load-operator-image OPERATOR_VERSION=${CSI_OPERATOR_VERSION} REGISTRY=${REGISTRY}
```

##### Install on kind

```
cd ${CSI_BAREMETAL_OPERATOR_DIR}

# Install Operator
helm install csi-baremetal-operator ./charts/csi-baremetal-operator/ \
    --set image.tag=${CSI_OPERATOR_VERSION} \
    --set image.pullPolicy=IfNotPresent

#Install Deployment
helm install csi-baremetal ./charts/csi-baremetal-deployment/ \
    --set image.tag=${CSI_VERSION} \
    --set image.pullPolicy=IfNotPresent \
    --set scheduler.patcher.enable=true \
    --set driver.drivemgr.type=loopbackmgr \
    --set driver.drivemgr.deployConfig=true \
    --set driver.log.level=debug \
    --set scheduler.log.level=debug \
    --set nodeController.log.level=debug
```

You can configure Loopback DriveManager's devices through ConfigMap. The default one is in charts.
For example:
```
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
  name: loopback-config
  labels:
    app: csi-baremetal-node
data:
  config.yaml: |-
    defaultDrivePerNodeCount: 8
    nodes:
    - nodeID: mynode.com
      driveCount: 10
      drives:
        - serialNumber: LOOPBACK1318634239
          size: 200Mi
        - serialNumber: RANDOMDEVICE
          vid: vid
        - serialNumber: LOOPBACK1462456734
          removed: true
```

Because of DriveManager is deployed on each node, if you want to set configuration of specified DriveManager, you need to
add its NodeID to `nodes` field of configuration. Loopback DriveManager is able to update devices according to
 configuration in runtime. If `drives` field contains existing drive (check by serialNumber) then configuration of this
drive will be updated and missing fields will be filled with defaults. If `drives` field contains new drive then this 
drive will be appended to DriveManager if it has free slot. It means that if DriveManager already has `driveCount` devices
then the new drive won't be appended without increasing of `driveCount`. If you increase `driveCount` in runtime then
DriveManager will add missing devices from default or specified drives. If you decrease `driveCount` in runtime then nothing
will happen because it's not known which of devices should be deleted (some of them can hold volumes/LVG). To fail
specified drive you can set `removed` field as true (See the example above). This drive will be shown as `Offline`.

##### Validation

```
# Install test app
cd ${CSI_BAREMETAL_DIR}
kubectl apply -f test/app/nginx.yaml

# Check all pods are Running and Ready
kubectl get pods

# And all PVCs are Bound
kubectl get pvc
```

## Perform E2E

TODO - add information about CI after https://github.com/dell/csi-baremetal/issues/562

## Contacts
If you have any questions, please, open [GitHub issue](https://github.com/dell/csi-baremetal/issues/new) in this repository with the ***question*** label.
