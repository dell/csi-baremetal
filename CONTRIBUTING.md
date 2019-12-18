# Baremetal CSI Plugin Contributing Guide

## Workflow overview
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
7. Merge your pull request after getting 2 approvals.

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

## Contacts
If you have any questions, please, contact Baremetal CSI Plugin team in our Slack [channel](https://dellstorage.slack.com/archives/CM7RQQ29X)

