# Release automation

## Purpose
Reduce manually performed operation during csi release procedure.

## Proposal
Use GitHub actions for release charts and update helm repository.

Whole release workflow contains 2 parts:
1. Release workflow in current repo triggered on every push to master/release banch with version tag matched pattern 'v*'
2. Release workflow in [csi-baremetal-operator](https://github.com/dell/csi-baremetal-operator) repo

### Detailed description

![Getting Started](./images/release_workflow.png)

release.yml in current repo covers the following part of release flow:
* release issue creation (descriptions can be added manually)
* make csi-baremetal-deployment version
* tag current head commit with release tag
* update helm repository
* trigger release workflow in csi-baremetal-operator repo