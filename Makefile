# build command:
# REGISTRY=<registry_url> TAG="<tag_name>" make all

# version
MAJOR            := 0
MINOR            := 0
PATCH            := 1
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

.PHONY: test

all: build image push

build:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/_output/baremetal_csi ./cmd/main.go

image:
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}:${TAG} .
	docker tag ${REGISTRY}/${REPO}:${TAG} ${HARBOR}/${REPO}:${TAG}

push:
	docker push ${REGISTRY}/${REPO}:${TAG}
	docker push ${HARBOR}/${REPO}:${TAG}

clean:
	rm -rf ./build/_output
	docker rmi ${REGISTRY}/${REPO}:${TAG}

prepare-lint:
	curl -L -O https://github.com/golangci/golangci-lint/releases/download/v${LINTER_VERSION}/golangci-lint-${LINTER_VERSION}-linux-amd64.tar.gz && \
    tar -xf golangci-lint-${LINTER_VERSION}-linux-amd64.tar.gz && \
    cp golangci-lint-${LINTER_VERSION}-linux-amd64/golangci-lint ${GOPATH}/bin/ && \
    rm -r golangci-lint-${LINTER_VERSION}-*

lint:
	golangci-lint run ./...

test:
	go test -cover ./... -coverprofile=coverage.out

coverage:
	go tool cover -html=coverage.out -o coverage.html
