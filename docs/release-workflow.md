# Release automation

## Purpose
Reduce manually performed operation during csi release procedure.

## Proposal
Use GitHub actions for release charts and update helm repository.

Whole release workflow contains 2 parts:
1. Release workflow in current repo triggered on release or pre-release creation
2. Release workflow in [csi-baremetal-operator](https://github.com/dell/csi-baremetal-operator) repo

### Detailed description

![Getting Started](./images/release_workflow.png)

## Restrictions
* release branches in both repos should have the same names
* release description should has the following format:
`csi_version: <version>, csi_operator_version: <version>`

## Testing strategy
Linting is performed on the [actionlint](https://github.com/rhysd/actionlint) base.
Local testing can be performed with [Act](https://github.com/nektos/act).

Steps to run tests:
1. `make prepare-env` will download and install mentioned tools
2. `make workflows-lint` will lint all workflows in `.github/workflows` dir
3. Create file `.github/workflows/tests/wf.secrets` with GITHUB_TOKEN variable definition. Token is generated on [token page of GitHub user settings](https://github.com/settings/tokens). Required permissions: repo, workflow.
3. `make test-release-workflow` will run workflow locally and execute all steps except marked as `${{ !env.ACT }}`

csi-baremetal workflow test is started by the following cmd:
* test GA release: `act release -e .github/workflows/tests/release.json --secret-file .github/workflows/tests/wf.secrets`
* test pre-release: `act release -e .github/workflows/tests/pre-release.json --secret-file .github/workflows/tests/wf.secrets`

csi-baremetal-operator workflow test is started by the following cmd:
* test GA release: `act workflow_dispatch -e .github/workflows/tests/release.json --secret-file .github/workflows/tests/wf.secrets`
* test pre-release: `act workflow_dispatch -e .github/workflows/tests/pre-release.json --secret-file .github/workflows/tests/wf.secrets`