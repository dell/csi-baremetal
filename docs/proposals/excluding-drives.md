# Proposal: Specific drives excluding

Last updated: 07.02.2022


## Abstract

CSI Baremetal needs to have API for excluding specific drives from the common storage pool. 
Excluded drives won't be used for allocating new Volumes or LVGs.

## Background

User stories:
1. Deploy another CSI driver in kube-cluster.
2. Reserve space for increasing root partition.
3. A kube-node is used for 2 or more kube-clusters (or not only for cluster).

So in these cases CSI haven't to allocate Volumes for some drives.

## Proposal

### User API

The rules to include/exclude drives based on:
- Size
- Type (HDD, SSD, ...)
- Serial Number
- PID
- VID
```yaml
drive_rule:
  size:
    less_then: 1Gi
    more_then: 100Mi
  # include drives with the following parameters only
  in:
    type:
      - HDD
    SN:
      - sn1
      - sn2
    PID:
      - ...
    VID:
      - ...
  # exclude drives with the following parameters
  not_in:
    type:
      - SSD
    SN:
      - ...
    PID:
      - ...
    VID:
      - ...
```

Settings can be set for all nodes in the cluster or for only one node. 
In the node section user chooses is global rules be applied or not.
```yaml
drive_options:
  global:
    drive_rule: ...
  nodes:
    - name: node1
      enable_global: true/false
      drive_rule:
    - name: ...
```

#### Pass settings
1. Set global drive rule on CSI installation.

Helm command for `csi-baremetal-driver` should be modified with:
```yaml
--set driver.node.global_drive_rule.size.more_then="1Gi" \
--set driver.node.global_drive_rule.in.SN={sn1, sn2, sn3}
```

The option to modify rules for specific nodes is not supported due to complex formatting.

2. After installing CSI user could edit a ConfigMap with CSI nodes setting to change included/excluded drives list

## Rationale

n/a

## Compatibility

n/a

## Implementation

#### Discover flow modification

![Screenshot](images/drive_including.png)

## Open issues (if applicable)

| ID      | Name                                                                                                   | Descriptions                                                          | Status  | Comments                                      |
|---------|--------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------|---------|-----------------------------------------------|
| ISSUE-1 | Should we have option to include some drives on specific nodes in pool despite the common constraints? |                                                                       | APPLIED |                                               |   
| ISSUE-2 | Do we need to implement procedure to replace Volumes on another drive if this one was excluded?        | CSI can reduce drive size to 0 and wait until Volumes will be removed | REJECT  | Just sending event is enough                  |
| ISSUE-3 | Could Serial Number be used instead of or in addition to drive path?                                   | Use SN, PID, VID instead                                              | APPLIED | Drive path might be changed after node reboot |

