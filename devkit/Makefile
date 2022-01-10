# Additional variables
#include vars.mk

IMG ?= csi-baremetal-devkit
SIMG ?= csi-baremetal-devkit
TIMG ?= csi-baremetal-devkit
DEVKIT_VERSION = latest

build:
	docker build -f Dockerfile -t ${IMG}:${DEVKIT_VERSION} .

push:
	docker push ${IMG}:${DEVKIT_VERSION}

retag:
	docker tag ${SIMG}:${DEVKIT_VERSION} ${TIMG}:latest
	docker push ${TIMG}:latest
