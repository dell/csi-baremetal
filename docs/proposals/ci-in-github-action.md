# Proposal: CI in Github Action

Last updated: 01.11.2021

## Abstract

Description of triggers and user actions to run CSI Baremetal CI.

## Background

We need to create Github Action to perform CSI Baremetal CI tests, 
because running tests locally requires much hardware resources and developers' time.
It is also making simpler to validate Pull Requests from fork-repositories.

## Proposal

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

## Implementation

#### Keyword to trigger CI
```
  issue_comment:
    types: [created]
```

**Note!** As action is not triggered by `pull_request`, it won't block PR merge.

#### CI details
- Suite - `test-short-ci` (community tests only)
- Kind workers number - 2

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Script for CI  | We need to have script for CI testing  |   |
ISSUE-2 | Parsing CI results  | It is better to have a human readable report with info about number of passed/failed/skipped tests  |   |   
