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

#### Local build
Setup all required dependencies:
```
make install-compile-proto
make install-controller-gen
make generate-deepcopy
make dependency
export DRIVE_MANAGER_TYPE=loopbackmgr
```
Build binaries:
```
make build
```
Build docker images:
```
export REGISTRY="myregistry.com"
make images
```
Run Unit tests:
```
make test
```
| Action                | Command       | Comment                                                              |
|-----------------------|---------------|----------------------------------------------------------------------|
| clean build artifacts | `make clean`  | [`build/_output/baremetal_csi`](./build/_output/baremetal_csi/) directory with all artifacts will be removed |
| build plugin binary   | `make build`  | artifacts can be found in the [`build/_output/baremetal_csi`](./build/_output/baremetal_csi/) directory.     |
| build plugin image    | `make images`  | |
| run linter            | `make lint`  | results will be printed to your terminal|



##### Running Baremetal CSI E2E tests locally

* Install `lvm2` package on your machine
* Create kind (version >= v0.7.0) cluster with the specified config. *Note that kind workers must be run with host IPC*
```
kind create cluster --config  test/kind/kind.yaml
```
* KIND can't pull images from remote repository, to load images to local docker repository on nodes:
```
export CSI_VERSION=`make version`
export REGISTRY="asdrepo.isus.emc.com:9042"

make kind-pull-images TAG=${CSI_VERSION} REGISTRY=${REGISTRY}
make kind-tag-images TAG=${CSI_VERSION} REGISTRY=${REGISTRY}
make kind-load-images TAG=${CSI_VERSION} REGISTRY=${REGISTRY}
```
* E2E tests need yaml files with baremetal-csi resources (plugin, controller, rbac). To create yaml files use helm command:
```
helm template charts/csi-baremetal-driver \
    --output-dir /tmp --set image.tag=${CSI_VERSION} \
    --set env.test=true --set drivemgr.type=loopbackmgr \
    --set drivemgr.deployConfig=true \
    --set image.pullPolicy=IfNotPresent
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
    app: baremetal-csi-node
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
 
* Set kubernetes context to kind:
```
kubectl config set-context "kind-kind"
```
* Run e2e tests:
```
go run cmd/tests/baremetal_e2e.go -ginkgo.v -ginkgo.progress --kubeconfig=<kubeconfig path>
```
* Delete KIND cluster:
```
kind delete cluster
```
## Contacts
If you have any questions, please, open [GitHub issue](https://github.com/dell/csi-baremetal/issues/new) in this repository with the ***question*** label.
