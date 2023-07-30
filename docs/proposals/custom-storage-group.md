# Proposal: Custom Storage Group 

Last updated: 15-05-2023


## Abstract

At present, for each PVC, CSI just chooses one of those physical drives whose available capacity meets the requested
storage size of PVC to provision corresponding volume and doesn't support the functionality to provision volume on the physical drive specified by user.
Here, we propose a custom-storage-group solution that user can select specific physical drives to provision their volumes.

## Background

To use CSI to create persistent volumes, you need to specify the storage classes managed by CSI in your k8s volume specifications. All the current CSI-managed storage classes are pre-defined storage classes created during CSI installation, based on local disk types including HDD, SSD, NVME and whether to use 1 entire local drive or create 1 logical volume for the k8s persistent volume.
In our current Bare-metal CSI, users can only have PVC satisfying their basic request based on storage type and size and cannot select the specific drive for their PVC. Instead, for each PVC, CSI will select 1 among those local drives satisfying the request of PVC. Furthermore, for LVG PVCs, the current CSI’s drive selection strategy is that it tries to accumulate those PVCs on the drive with existing LVG PVCs and the least free capacity if applicable.
This strategy aims to leave more entire free disk, which is useful if there would be requirements of non-LVG PVCs later, but it cannot satisfy the requirement of selecting specific drives in volume provisioning.

## Proposal

StorageGroup is a new Custom Resource of Bare-metal CSI.

User can create custom storage group on specific drives by specifying some criteria to select drives on properties via driveSelector field of StorageGroup.

Then, the corresponding custom-storage-group k8s label **drive.csi-baremetal.dell.com/storage-group: ${storage-group-name}** would be immediately synced to all the CSI Drives selected in this storage group by the storage group controller after each storage group created in kubernetes. Actually, all the corresponding custom-storage-group k8s label would then be also further immediately synced to the corresponding AC objects.

Here, we actually use the storage-group k8s label to symbolize the drives list in the corresponding storage group, which can be conveniently listed by k8s list api with labelSelector.

Then, to make PVCs land on those specific drives selected, just add the corresponding custom storage group label **drive.csi-baremetal.dell.com/storage-group: ${storage-group-name}** in volume claim spec, CSI will ensure the custom-storage-group label matching in selecting drives for PVCs, PVC with k8s label **drive.csi-baremetal.dell.com/storage-group: ${storage-group-name}** would only reside on one of those specific drives with the same k8s label **drive.csi-baremetal.dell.com/storage-group: ${storage-group-name}**.
