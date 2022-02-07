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

#### Pass settings

1. Exclude drives on CSI installation.

Helm command for `csi-baremetal-driver` should be modified with:
```yaml
--set 'driver.node.excludedDrives={sdb,sdc}'
```

This option will set excluded drives setting for all nodes.

2. After installing CSI user could edit a ConfigMap with CSI nodes setting to change excluded drives list
```yaml
excluded_drives:
  default: # will work for all nodes
    - sdb (ISSUE-3)
    - sdc
  nodes:
    - name: node1
      drives:
        - sdh
    - name: node2
      drives:
        - sdc
```

Result set of excluded drives will be the union of `default` and `nodes` options (ISSUE-1). An example from the ConfigMap above:
- node1: [sdb, sdc, sdh]
- node2: [sdb, sdc]

#### Excluded drives behavior

If drive was marked as excluded, it changes status to EXCLUDED. Drivemgr continues to receive and show its health, but DR procedure won't be initialized.
An example:
```bash
NAME                                   SIZE        TYPE   HEALTH   STATUS   USAGE      SYSTEM   PATH          SERIAL NUMBER        NODE                                
017d2529-a72b-422f-a435-23939f6ef4e5   105906176   HDD    GOOD     ONLINE   IN_USE              /dev/loop16   LOOPBACK3303120819   1145a3e6-2f80-4de8-ac90-d35391fdc026
03bfa055-0ddf-4c90-817a-5ab6c88aa02f   105906176   HDD    BAD      ONLINE   IN_USE              /dev/loop20   LOOPBACK3866073594   5ff13d3f-84d0-4ce0-ad80-74c02c471aaa
18e63992-0fad-49d3-930b-101d33ec9603   0           HDD    BAD      ONLINE   EXCLUDED            /dev/loop17   LOOPBACK4262524596   1145a3e6-2f80-4de8-ac90-d35391fdc026
22b6e3c0-6991-4ae7-a1ac-74e41eab77b1   0           HDD    GOOD     ONLINE   EXCLUDED            /dev/loop15   LOOPBACK198229819    1145a3e6-2f80-4de8-ac90-d35391fdc026
```
Cases:
- Drive is IN_USE and has no Volumes and Reservations -> Drive's usage will be changed to EXCLUDED
- Drive isn't IN_USE (DR was started) has Volumes or Reservations -> Drive controller will generate event about failed exclusion (ISSUE-2)

## Rationale

Drive CR can be deleted instead of just changing usage to EXCLUDED. 
But it makes worse drives pool visibility.
User can't see health of the EXCLUDED drive.

## Compatibility

n/a

## Implementation

On Discover stage (every one minute):
1. Check excluded-drives section in the node-config
2. If drive is in the exclusion list and has EXCLUDED status, we drop processing of this one
3. Check usage, Volumes and Reservations for this drive
4. Change usage to EXCLUDED or generate failing event

In drive reconciliation controller:
1. Delete Drive CR, if it is EXCLUDED and OFFLINE

## Open issues (if applicable)

| ID      | Name                                                                                                   | Descriptions                                                          | Status | Comments |
|---------|--------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------|--------|----------|
| ISSUE-1 | Should we have option to include some drives on specific nodes in pool despite the common constraints? |                                                                       |        |          |   
| ISSUE-2 | Do we need to implement procedure to replace Volumes on another drive if this one was excluded?        | CSI can reduce drive size to 0 and wait until Volumes will be removed |        |          |
| ISSUE-3 | Could Serial Number be used instead of or in addition to drive path?                                   |                                                                       |        |          |

