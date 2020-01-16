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

lint:
	# golangci-lint run ./...

test:
	# go test -cover ./... -coverprofile=coverage.out

coverage:
	# go tool cover -html=coverage.out -o coverage.html

compile-proto:
	mkdir -p api/generated/v1/
	protoc -I=api/v1 --go_out=plugins=grpc:api/generated/v1 api/v1/*.proto
