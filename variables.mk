# project name
PROJECT          := csi-baremetal

### common path
CSI_OPERATOR_PATH=../csi-baremetal-operator
CSI_CHART_CRDS_PATH=$(CSI_OPERATOR_PATH)/charts/csi-baremetal-operator/crds
CRD_OPTIONS ?= "crd:trivialVersions=true"

### version
MAJOR            := 1
MINOR            := 1
PATCH            := 0
PRODUCT_VERSION  ?= ${MAJOR}.${MINOR}.${PATCH}
BUILD_REL_A      := $(shell git rev-list HEAD |wc -l)
BUILD_REL_B      := $(shell git rev-parse --short HEAD)
BLD_CNT          := $(shell echo ${BUILD_REL_A})
BLD_SHA          := $(shell echo ${BUILD_REL_B})
RELEASE_STR      := ${BLD_CNT}.${BLD_SHA}
FULL_VERSION     := ${PRODUCT_VERSION}-${RELEASE_STR}
TAG              := ${FULL_VERSION}
BRANCH           := $(shell git rev-parse --abbrev-ref HEAD)

### third-party components version
CSI_PROVISIONER_TAG := v3.1.0
CSI_RESIZER_TAG     := v1.4.0
CSI_REGISTRAR_TAG   := v2.5.0
LIVENESS_PROBE_TAG  := v2.6.0
BUSYBOX_TAG         := 1.29

### PATH
SCHEDULING_PKG := scheduling
SCHEDULER_EXTENDER_PKG := extender
SCHEDULER_EXTENDER_PATCHER_PKG := scheduler/patcher
CR_CONTROLLERS := crcontrollers
NODE_CONTROLLER_PKG := node

### components
NODE             := node
DRIVE_MANAGER    := drivemgr
CONTROLLER       := controller
SCHEDULER        := scheduler
EXTENDER         := extender
EXTENDER_PATCHER := scheduler-patcher
NODE_CONTROLLER  := ${NODE_CONTROLLER_PKG}-${CONTROLLER}
PLUGIN           := plugin
OPERATOR         := operator

BASE_DRIVE_MGR     := basemgr
LOOPBACK_DRIVE_MGR := loopbackmgr
DRIVE_MANAGER_TYPE := ${BASE_DRIVE_MGR}

# external components
CSI_PROVISIONER := csi-provisioner
CSI_REGISTRAR   := csi-node-driver-registrar
CSI_RESIZER     := csi-resizer
LIVENESS_PROBE  := livenessprobe
BUSYBOX         := busybox

HEALTH_PROBE    	 := health_probe
HEALTH_PROBE_BIN_URL := https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.3.1/grpc_health_probe-linux-amd64

### go env vars
GO_ENV_VARS     := GO111MODULE=on ${GOPRIVATE_PART} ${GOPROXY_PART}

### custom variables that could be ommited
GOPRIVATE_PART  :=
GOPROXY_PART    := GOPROXY=https://proxy.golang.org,direct

### go dependencies
CONTROLLER_GEN_VER := v0.5.0
MOCKERY_VER        := v2.9.4
PROTOC_GEN_GO_VER  := v1.3.2
CLIENT_GO_VER      := v0.22.5

### Ingest information to the binary at the compile time
PACKAGE_NAME := github.com/dell/csi-baremetal
METRICS_PACKAGE := ${PACKAGE_NAME}/pkg/metrics
BASE_PACKAGE := ${PACKAGE_NAME}/pkg/base
LDFLAGS := -ldflags "-X ${METRICS_PACKAGE}.Revision=${RELEASE_STR} -X ${METRICS_PACKAGE}.Branch=${BRANCH} -X ${BASE_PACKAGE}.PluginVersion=${PRODUCT_VERSION}"

### Kind
KIND_BUILD_DIR		:= ${PWD}/devkit/kind
KIND_CONFIG_DIR		:= tests/kind
KIND				:= ${KIND_BUILD_DIR}/kind
KIND_CONFIG			:= kind.yaml
KIND_IMAGE_VERSION	:= v1.19.11
KIND_WAIT			:= 30s

### ci vars
# timeout for short test suite, must be parsable as Go time.Duration (60m, 2h)
SHORT_CI_TIMEOUT := 60m
SANITY_TEST := 'volumes should store data'  # focus expects to get a regexp as input not a simple string

# override some of variables, optional file
-include variables.override.mk
