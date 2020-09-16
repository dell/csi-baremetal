# project name
PROJECT          := baremetal-csi-plugin

### file paths
CHARTS_PATH		 := charts/baremetal-csi-plugin
EXTENDER_CHARTS_PATH := charts/scheduler-extender

### version
MAJOR            := 0
MINOR            := 0
PATCH            := 9
PRODUCT_VERSION  ?= ${MAJOR}.${MINOR}.${PATCH}
BUILD_REL_A      := $(shell git rev-list HEAD |wc -l)
BUILD_REL_B      := $(shell git rev-parse --short HEAD)
BLD_CNT          := $(shell echo ${BUILD_REL_A})
BLD_SHA          := $(shell echo ${BUILD_REL_B})
RELEASE_STR      := ${BLD_CNT}.${BLD_SHA}
FULL_VERSION     := ${PRODUCT_VERSION}-${RELEASE_STR}
TAG              := ${FULL_VERSION}

### third-party components version
CSI_PROVISIONER_TAG := v1.2.2
CSI_REGISTRAR_TAG   := v1.0.1-gke.0
CSI_ATTACHER_TAG    := v1.0.1
LIVENESS_PROBE_TAG  := v2.1.0
BUSYBOX_TAG         := 1.29

### PATH
SCHEDULER_EXTENDER_PKG := scheduler
SCHEDULER_EXTENDER_PATCHER_PKG := scheduler-patcher

### components
NODE             := node
DRIVE_MANAGER    := drivemgr
CONTROLLER       := controller
EXTENDER         := extender
EXTENDER_PATCHER := scheduler-patcher

BASE_DRIVE_MGR     := basemgr
LOOPBACK_DRIVE_MGR := loopbackmgr
DRIVE_MANAGER_TYPE := ${BASE_DRIVE_MGR}

# external components
CSI_PROVISIONER := csi-provisioner
CSI_REGISTRAR   := csi-node-driver-registrar
CSI_ATTACHER    := csi-attacher
LIVENESS_PROBE  := livenessprobe
BUSYBOX         := busybox

HEALTH_PROBE    	 := health_probe
HEALTH_PROBE_BIN_URL := https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.3.1/grpc_health_probe-linux-amd64

### go env vars
GO_ENV_VARS     := GO111MODULE=on ${GOPRIVATE_PART} ${GOPROXY_PART}

### custom variables that could be ommited
GOPRIVATE_PART  :=
GOPROXY_PART    := GOPROXY=https://proxy.golang.org,direct

# override some of variables, optional file
-include variables.override.mk
