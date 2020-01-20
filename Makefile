# build command:
# REGISTRY=<registry_url> TAG="<tag_name>" make all

# file paths

# version
MAJOR            := 1
MINOR            := 0
PATCH            := 0
PRODUCT_VERSION  ?= ${MAJOR}.${MINOR}.${PATCH}
BUILD_REL_A      := $(shell git rev-list HEAD |wc -l)
BUILD_REL_B      := $(shell git rev-parse --short HEAD)
BLD_CNT          := $(shell echo ${BUILD_REL_A})
BLD_SHA          := $(shell echo ${BUILD_REL_B})
RELEASE_STR      := ${BLD_CNT}.${BLD_SHA}
FULL_VERSION     := ${PRODUCT_VERSION}-${RELEASE_STR}
TAG				 := ${FULL_VERSION}

REPO 			 := baremetal-csi-plugin
REGISTRY          = 10.244.120.194:8085
HARBOR           := harbor.lss.emc.com/ecs

LINTER_VERSION   := 1.21.0

.PHONY: test build

all: build image push

dependency:
	GO111MODULE=on go mod download

build:

image:
	# docker build --network host --force-rm --tag ${REGISTRY}/${REPO}:${TAG} .
	# docker tag ${REGISTRY}/${REPO}:${TAG} ${HARBOR}/${REPO}:${TAG}

push:
	# docker push ${REGISTRY}/${REPO}:${TAG}
	# docker push ${HARBOR}/${REPO}:${TAG}

clean:
	rm -rf ./build/_output

clean-image:
	docker rmi ${REGISTRY}/${REPO}:${TAG}

lint: install-compile-proto
	golangci-lint run ./...

test:
	# install hal for correct compilation during go test
	make -f pkg/hwmgr/Makefile install-hal
	go test -cover ./... -coverprofile=coverage.out

coverage:
	go tool cover -html=coverage.out -o coverage.html

prepare-protoc:  # TODO: temporary solution while we build all in common devkit (DELIVERY-1488)
	mkdir -p proto_3.11.0
	curl -L -O https://github.com/protocolbuffers/protobuf/releases/download/v3.11.0/protoc-3.11.0-linux-x86_64.zip && \
	unzip protoc-3.11.0-linux-x86_64.zip -d proto_3.11.0/ && \
	sudo mv proto_3.11.0/bin/protoc /usr/bin/protoc && \
	protoc --version; rm -rf proto_3.11.0; rm protoc-*
	go get -u github.com/golang/protobuf/protoc-gen-go

compile-proto:
	mkdir -p api/generated/v1/
	protoc -I=api/v1 --go_out=plugins=grpc:api/generated/v1 api/v1/*.proto

install-compile-proto: prepare-protoc compile-proto
