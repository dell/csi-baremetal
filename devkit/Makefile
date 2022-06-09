#
# ancillary variables
#
major        := 0
minor        := 0
patch        := 0
git_cnt       = $(shell git rev-list HEAD |wc -l)
git_id        = $(shell git rev-parse --short HEAD)
git_rev       = $(git_cnt).$(git_id)


#
# file and directory variables
#
PWD := $(shell pwd)
WRAPPER := devkit
INSTALL_DIR := /usr/local/bin


#
# version variables
#
# tag and repo values may be overriden from the outside
REPO ?= csi-baremetal-devkit
TAG  ?= $(major).$(minor).$(patch)-$(git_rev)
IMAGE_REPO_TAG := $(REPO):$(TAG)

DOCKER_VERSION ?= 20.10.14_ce
GO_VERSION ?= 1.17.7
GOLANGCI_VERSION ?= 1.44.0
PROTOBUF_VERSION ?= 3.11.0
PROTOC_GEN_GO_VER ?= v1.3.2
KUBECTL_VERSION ?= 1.23.4
HELM_VERSION ?= 3.8.1
KIND_BUILDER ?= golang:1.16


#
# docker arguments
#
DOCKER_BUILD_ARGS = --tag ${IMAGE_REPO_TAG} \
              --file ${PWD}/Dockerfile \
              --network host \
              --build-arg arg_docker_ver=${DOCKER_VERSION} \
              --build-arg arg_go_ver=${GO_VERSION} \
              --build-arg arg_golandci_ver=${GOLANGCI_VERSION} \
              --build-arg arg_protobuf_ver=${PROTOBUF_VERSION} \
              --build-arg arg_protoc_gen_go_ver=${PROTOC_GEN_GO_VER} \
              --build-arg arg_kubectl_ver=${KUBECTL_VERSION} \
	      --build-arg arg_helm_ver=${HELM_VERSION} \
	      --build-arg KIND_BUILDER=${KIND_BUILDER} \
              ${PWD}

ifdef CACHE_IMAGE
DOCKER_BUILD_ARGS += --cache-from ${CACHE_IMAGE}
endif


#
# targets
#
all: build

build:
	docker build ${DOCKER_BUILD_ARGS}


install:
	ln --symbolic  ${PWD}/${WRAPPER} ${INSTALL_DIR}/${WRAPPER}


uninstall:
	rm --force ${INSTALL_DIR}/${WRAPPER}


help:
	@echo -e "To make ${REPO} image run:\n    make"
	@echo -e "To get image repo run:\n    make repo"
	@echo -e "To get image version run:\n    make version"
	@echo -e "To make and push ${REPO} image run:\n    make push REGISTRY=<your_registry>"
	@echo -e "To remove all images with repo ${REPO} run:\n    make clean"
	@echo -e "To install wrapper to ${INSTALL_DIR} run:\n    make install"
	@echo -e "To remove wrapper from ${INSTALL_DIR} run:\n    make uninstall"
	@echo -e "To get help run:\n    make help"
	@echo "To tune ${WRAPPER}'s behavior set the following environment variables:"
	@cat ${WRAPPER} | grep ":-" | sed -e 's|=.\+$$||' -e 's|readonly[[:space:]]\+|    |'
	@echo -e "NOTE: variables with _PATH suffexes should contain path within a file system\n"\
	         "     variables with _BOOL suffix should contain true/false values"


clean:
	docker rmi --force ${IMAGE_REPO_TAG} 2>/dev/null || true

push:
	docker tag ${IMAGE_REPO_TAG} ${REGISTRY}/${IMAGE_REPO_TAG}
	docker push ${REGISTRY}/${IMAGE_REPO_TAG}

version:
	@echo ${TAG}

repo:
	@echo ${REPO}
