# file paths

# version
MAJOR            := 0
MINOR            := 0
PATCH            := 4
PRODUCT_VERSION  ?= ${MAJOR}.${MINOR}.${PATCH}
BUILD_REL_A      := $(shell git rev-list HEAD |wc -l)
BUILD_REL_B      := $(shell git rev-parse --short HEAD)
BLD_CNT          := $(shell echo ${BUILD_REL_A})
BLD_SHA          := $(shell echo ${BUILD_REL_B})
RELEASE_STR      := ${BLD_CNT}.${BLD_SHA}
FULL_VERSION     := ${PRODUCT_VERSION}-${RELEASE_STR}
TAG              := ${FULL_VERSION}


HAL_VERSION      := 3.4.0.0-1835.b1a54fa

# registry
REPO             := baremetal-csi-plugin
REGISTRY         := 10.244.120.194:8085/atlantic
HARBOR           := harbor.lss.emc.com/atlantic

# paths
CHARTS_PATH		 := charts/baremetal-csi-plugin
