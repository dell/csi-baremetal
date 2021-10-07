# Release automation

## Purpose
Reduce manually performed operation during csi release procedure.

## Proposal
Use GitHub actions for release charts and update helm repository.

Whole release workflow contains 2 parts:
1. Release workflow in [csi-baremetal-operator](https://github.com/dell/csi-baremetal-operator) repo triggered on every push to master with version tag matched pattern 'v*'
2. Release workflow in current repo triggered by workflow_dispatch event

### Detailed description

release.yml in current repo covers the following part of release flow:
* make csi-baremetal-deployment version
* package charts
* tag current head commit with release tag
* update helm repository
* attach charts package to release