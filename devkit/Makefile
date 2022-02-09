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
DOCKER_VERSION ?= 20.10.9_ce
GO_VERSION ?= 1.16.13
GOLANGCI_VERSION ?= 1.44.0

IMAGE_REPO_TAG := $(REPO):$(TAG)

#
# targets
#
all: build

build:
	docker build --tag ${IMAGE_REPO_TAG} \
	             --file ${PWD}/Dockerfile \
	             --network host \
	             --build-arg arg_docker_ver=${DOCKER_VERSION} \
	             --build-arg arg_go_ver=${GO_VERSION} \
	             --build-arg arg_golandci_ver=${GOLANGCI_VERSION} \
	             ${PWD}


install:
	ln --symbolic  ${PWD}/${WRAPPER} ${INSTALL_DIR}/${WRAPPER}


uninstall:
	rm --force ${INSTALL_DIR}/${WRAPPER}


help:
	@echo -e "To make ${REPO} image run:\n    make"
	@echo -e "To make and push ${REPO} image run:\n    make push REPO=<your_repo>"
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

push: build
	docker push ${IMAGE_REPO_TAG}
