# common
CR_GROUP = "csi-baremetal.dell.com"
CR_VERSION = "v1"

# storage classes
HDD_SC = "csi-baremetal-sc-hdd"
SSD_SC = "csi-baremetal-sc-ssd"
HDDLVG_SC = "csi-baremetal-sc-hddlvg"
SSDLVG_SC = "csi-baremetal-sc-ssdlvg"
SYSLVG_SC = "csi-baremetal-sc-syslvg"
NVME_SC = "csi-baremetal-sc-nvme-raw-part"
NVMELVG_SC = "csi-baremetal-sc-nvmelvg"

# storage types
STORAGE_TYPE_SSD = "SSD"
STORAGE_TYPE_HDD = "HDD"
STORAGE_TYPE_NVME = "NVME"
STORAGE_TYPE_HDDLVG = "HDDLVG"
STORAGE_TYPE_SSDLVG = "SSDLVG"
STORAGE_TYPE_SYSLVG = "SYSLVG"
STORAGE_TYPE_NVMELVG = "NVMELVG"

# usages
USAGE_IN_USE = "IN_USE"
USAGE_RELEASING = "RELEASING"
USAGE_RELEASED = "RELEASED"
USAGE_REMOVING = "REMOVING"
USAGE_REMOVED = "REMOVED"
USAGE_RAILED = "FAILED"

# statuses
STATUS_ONLINE = "ONLINE"
STATUS_OFFLINE = "OFFLINE"

# health
HEALTH_GOOD = "GOOD"
HEALTH_BAD = "BAD"

# fake attach
FAKE_ATTACH_INVOLVED = "FakeAttachInvolved"
FAKE_ATTACH_CLEARED = "FakeAttachCleared"

# plurals
DRIVES_PLURAL = "drives"
AC_PLURAL = "availablecapacities"
ACR_PLURAL = "availablecapacityreservations"
LVG_PLURAL = "logicalvolumegroups"
VOLUMES_PLURAL = "volumes"
