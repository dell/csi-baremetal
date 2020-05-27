# Baremetal CSI Plugin Contribution Guide

## Workflow overview

### Code style convention
1. Imports statement should be divided into 3 blocks each block is separated from others by empty line.
 - First block should contain only imports from standard library. 
 - Second - external libraries imports.
 - Third - our internal imports that don't relate to that repository (baremetal-csi-plugin).
 - Forth - internal imports that relates to that repository (baremetal-csi-plugin).
If there are no imports from some block, that block should be omitted.

2. If some structure have a field with logger, that field should be the last in the structure declaration.

### Before PR Creation
1. If there is no JIRA issue for the problem you're going to solve, create one under []() project.
2. The branch type should be **bugfix** or **feature** based on the JIRA issue type:
```
    feature/bugfix-<JIRA ID>-<short description>
```
### While working on PR
1. Once changes are ready commit them and push to the server:
```
    git commit -a -v -m "commit message"
    git push origin <JIRA branch name>
```
2. Add Unit Tests for your changes.
3. Cover your by Kubernetes e2e tests.
4. Create Pull Request with title:
    ```
        [JIRA-ID] <short description>
    ```
5. Choose label for your Pull Request:
    - "Feature" - for PRs with feature.
    - "Bugfix" - for PRs with bugfix.
    - "Enhancement" - for PRs with some enhancement like refactoring or test adding.
    - "Critical" - for PRs with something that should be merge as soon as possible.
6. Fill Pull Request template:
    - Fill "Purpose" of Pull Request Template:
        - Add detailed description of the issue/feature and how it was solved/developed to simplify review process.
    - Make actions from PR Checklist.
    - Run custom CI (link) and attach link to your Pull Request.
7. Wait approve or verbal consent from a person who commented your PR
8. Merge your pull request after getting 2 approvals.

### After PR merging
1. Delete your branch.
2. Check Bare-metal CI job ().
3. If the tests fail and it can't be fixed within one day, then revert your changes with new PR.

#### Commit reverting

To revert a commit, perform the following:
 * Open the [Baremetal CSI Plugin](https://eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin) repository.
 * Go to the closed PR which you want to revert.
 * Click the button `Revert`.
 * Fill Pull Request template.
 * Merge your pull request after getting 2 approvals.

### Code style
  Baremetal CSI Plugin is written in Golang. Our plugin uses common Golang code style.
  For auto-detecting code style inconsistencies we use [golangci-lint](https://github.com/golangci/golangci-lint).
  Run `make lint` if you want to check your changes.
  
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
  

### Source code overview
TODO - Add after changing project structure

### Preparing Build Environment
#### Local build
Developer can either configure his machine to build Baremetal CSI Plugin package or use Infra-Devkit. The recommended way is to use Infra-Devkit.
Please see Infra-Devkit [README.md](https://eos2git.cec.lab.emc.com/ECS/infra-devkit/blob/master/README.md).

| Action                | Command       | Comment                                                              |
|-----------------------|---------------|----------------------------------------------------------------------|
| clean build artifacts | `make clean`  | [`build/_output/baremetal_csi`](./build/_output/baremetal_csi/) directory with all artifacts will be removed |
| build plugin binary   | `make build`  | artifacts can be found in the [`build/_output/baremetal_csi`](./build/_output/baremetal_csi/) directory.     |
| build plugin image    | `make image`  | |
| run linter            | `make lint`  | results will be printed to your terminal|


#### Remote build
- Push your branch into baremetal-csi repo.
- Run Jenkins job [bare-metal-csi-custom-build](https://asd-ecs-jenkins.isus.emc.com/jenkins/job/bare-metal-csi-custom-build/) passing your branch as a parameter.
  When build finishes the resulting image will be pushed to Harbor project with tag which you specified.


#### Manual running
##### Running Baremetal CSI CI
1. TBD

##### Running Baremetal CSI Acceptance
1. TDB


#### Automated running
##### Running Baremetal CSI CI
1. TBD

##### Running Baremetal CSI Acceptance
1. TDB

##### Running Baremetal CSI E2E tests locally
* Set environment variables to use KIND in  devkit: 
```
export DEVKIT_DOCKER_NETWORK_HOST_BOOL=true
export DEVKIT_KIND_KERNEL_MODULE_SHARED_BOOL=true
export DEVKIT_KIND_SRC_SHARED_BOOL=true
export DEVKIT_CACHE_VAR_LIB_DOCKER_SHARED_BOOL=true
export DEVKIT_USER_NAME=root
export DEVKIT_DEVICES_SHARED_BOOL=true
export DEVKIT_UDEV_SHARED_BOOL=true
```
* Run devkit:
```
devkit --hal no
```
* Create kind (version >= v0.7.0) cluster with the specified config
```
kind create cluster --kubeconfig <kubeconfig path> --config  test/kind/kind.yaml
```
* KIND can't pull images from remote repository, to load images to local docker repository on nodes:
```
make kind-load-images
kind load docker-image busybox:1.29
```
* E2E tests need yaml files with baremetal-csi resources (plugin, controller, rbac). To create yaml files use helm command:
```
helm template charts/baremetal-csi-plugin/ 
--output-dir /tmp --set image.tag=`make version`
--set drivemgr.type=LOOPBACK // test with loopback drivemgr
--set drivemgr.deployConfig=true // deploy config for loopback drivemgr
--set busybox.image.tag=1.29  // e2e tests need this busybox for testing pods
--set image.pullPolicy=IfNotPresent /*KIND can't work with imagePullPolicy <Always> 
                                      because it can pull only from local repository*/
``` 
If you set `--output-dir` to another directory, you should change this line in [code](https://eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin/blob/feature-FABRIC-8422-implement-base-csi-e2e-tests-with-Kind/test/test/csi-volume.go#L22) to your directory, so framework can find yaml files.

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
    - nodeID: layton-oil.ecs.lab.emc.com
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
If you have any questions, please, contact Baremetal CSI Plugin team in our Slack [channel](https://dellstorage.slack.com/archives/CM7RQQ29X)

