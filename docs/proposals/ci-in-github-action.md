# Proposal: CI in Github Action

Last updated: 01.11.2021

## Abstract

Description of triggers and user actions to run CSI Baremetal CI.

## Background

We need to create Github Action to perform CSI Baremetal CI tests, 
because running tests in the developer's environment requires much hardware resources and time.
It is also making simpler to validate Pull Requests from fork-repositories.

## Proposal

We can make our tests running on Github Actions.

Environment:
- Ubuntu 20
- Kind 0.11.1
- kubernetes v1.19

Suite:
- Community tests only (test-short-ci)

#### User interactions scenario
Keyword to trigger CI action - comment in Pull Request. Developer can see tests result in the next comment written by Action.

1. Developer: Add a comment to PR with the following content
```
/ci
operator_branch=branch1
```

2. Action: Add a comment to PR
```
Start CI. Parameters:
operator_branch=branch1
```

3. Action: Perform CI suite

4. Action: Add a comment to PR
```
CI tests passed/failed
attachments: <log.txt>
```

#### New repo for e2e tests

Advantages:
- Remove `k8s.io/kubernetes` dependency from CSI Baremetal (with all replaces)
- CI contains tests for CSI Operator too, it is strange to have them in the main repo
- e2e tests have simple dependencies from source code (they can be replaced)

Disadvantages:
- More complexity in jobs/actions

## Implementation

Tasks:
1. Create Github Action in CSI Baremetal repo with testing flow described in CONTRIBUTING guidelines (ETA - 3)
2. Add a way to call action from comment (ETA - 2)
3. Add human-readable report and info about passed/failed/skipped tests in result massage (ETA - 2)
4. Investigate to the requirements for a script to build and test CSI locally & common testing process for all jobs/actions (ETA - 1)
5. Create separate Github Action in CSI Baremetal Operator repo (ETA - 1)
6. Investigate to the way to create new repo for e2e tests with new information about actions usability (ETA - 1)

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Script for CI  | We need to have script for CI testing  |   |
ISSUE-2 | Parsing CI results  | It is better to have a human readable report with info about number of passed/failed/skipped tests  |   |
ISSUE-3 | Should we have separate repo for e2e tests?  |   |   |   
ISSUE-4 | Github-hosted runners time constraints | We need to understand, how much CI runs can we perform in month due to Github Actions time limitations. What will happen if we break constraints?
