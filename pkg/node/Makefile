.PHONY: build

dependency:
	GO111MODULE=on go mod download

build:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/_output/nodemgr ../../cmd/node/main.go