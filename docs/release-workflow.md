# Release automation

## Purpose
Reduce manually performed operation during csi release procedure.

## Proposal
Use GitHub actions for release charts and update helm repository.

Whole release workflow contains 2 parts:
1. Release workflow in current repo triggered on release creation
2. Release workflow in [csi-baremetal-operator](https://github.com/dell/csi-baremetal-operator) repo

### Detailed description

![Getting Started](./images/release_workflow.png)

## Restrictions:
* release branches in both repos should have the same names
* release description should has the following format:
`csi_version: <version>, csi_operator_version: <version>`
