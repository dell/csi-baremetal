# Proposal: Update CI procedures with csi-baremetal-operator 

Last updated: 12.04.2021


## Abstract

csi-baremetal-operator (https://github.com/dell/csi-baremetal-operator) must be integrated with CI procedures using in development at the moment.

## Proposal

#### csi-baremetal-operator-build

Linting & unity testing & bulding stages

Promoted artifacts:

- csi-baremetal-operator docker image

- csi-baremetal-operator helm chart

- csi-baremetal-CR helm chart

The artifacts have equal version gotten from 'make version'

#### csi-baremetal-build

Linting & unity testing & bulding stages

Promoted artifacts:

- csi-baremetal-controller docker image

- csi-baremetal-node docker image

- csi-baremetal-node-controller docker image

- csi-baremetal-scheduler-extender docker image

- csi-baremetal-scheduler-extender-patcher docker image

- csi-baremetal-##mgr docker image

The artifacts have equal version gotten from 'make version'

#### csi-baremetal-operator-ci

e2e testing stage: deploy csi-baremetal-CR with operator + perform base tests

Versions:

- csi-baremetal-operator helm chart -> ${new_version}

- csi-baremetal-CR helm chart -> ${new_version}


- csi-baremetal-operator docker image -> ${new_version}

- csi-baremetal-controller docker image -> green

- csi-baremetal-node docker image -> green

- csi-baremetal-node-controller docker image -> green

- csi-baremetal-scheduler-extender docker image -> green

- csi-baremetal-scheduler-extender-patcher docker image -> green

- csi-baremetal-##mgr docker image -> green

#### csi-baremetal-operator-ci

e2e testing stage: deploy csi-baremetal-CR with operator + perform all e2e tests

Versions:

- csi-baremetal-operator helm chart -> green

- csi-baremetal-CR helm chart -> green


- csi-baremetal-operator docker image -> green

- csi-baremetal-controller docker image -> ${new_version}

- csi-baremetal-node docker image -> ${new_version}

- csi-baremetal-node-controller docker image -> ${new_version}

- csi-baremetal-scheduler-extender docker image -> ${new_version}

- csi-baremetal-scheduler-extender-patcher docker image -> ${new_version}

- csi-baremetal-##mgr docker image -> ${new_version}

#### csi-custom-build and csi-custom-ci

It can be used for testing changes in one or in both repositories.

Vars:

- csi-baremetal version (empty -> build)

- csi-baremetal-operator version (empty -> build)

- csi-baremetal branch (default master)

- csi-baremetal-operator branch (default master)

## Implementation

1. Create and test new csi-baremetal-operator-build job

2. Update e2e tests in csi-baremetal repo and create ones in csi-baremetal-operator repo

3. Create csi-baremetal-operator-ci job

4. Update csi-baremetal-build, csi-baremetal-ci, csi-custom-build/ci jobs

5. Update other affected jobs

## Open issues

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | csi-baremetal-scheduler-ci  |  ci for https://github.com/dell/csi-baremetal-scheduling may be included to this proposal | Open  | 
ISSUE-2 | csi-baremetal-node-controller renaming  |  Can we will have conflicts in artifactory until renaming? | Open  | It will be solved if csi-baremetal-operator is tagged with specific version
