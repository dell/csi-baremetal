## Requirements
- go 1.16
- lvm2 packet installed on host machine
- kubectl/helm

## Build

### Set Variables

```
export REGISTRY=<your_docker_hub>
export CSI_BAREMETAL_DIR=<full_path_to_csi_baremetal_src>
export CSI_BAREMETAL_OPERATOR_DIR=<full_path_to_csi_baremetal_operator_src>
```

### Build csi-baremetal

```
cd ${CSI_BAREMETAL_DIR}
export CSI_VERSION=`make version`

# Get dependencies
make dependency

# Compile proto files
make install-compile-proto

# Generate CRD
make install-controller-gen
make generate-deepcopy

# Run unit tests
make test

# Run sanity tests
make test-sanity

# Run linting
make lint

# Build binary
make build
make DRIVE_MANAGER_TYPE=loopbackmgr build-drivemgr

# Build docker images
make images REGISTRY=${REGISTRY}
make DRIVE_MANAGER_TYPE=loopbackmgr image-drivemgr REGISTRY=${REGISTRY}
```

### Build csi-baremetal-operator

```
cd ${CSI_BAREMETAL_OPERATOR_DIR}
export CSI_OPERATOR_VERSION=`make version`

# Run unit tests
make test

# Run linting
make lint

# Build docker image
make docker-build REGISTRY=${REGISTRY}
```

## Pull images
If you have pushed images to your registry before, you can load them with the following command.
Skip it, if you have made build step.
```
make kind-pull-images TAG=${CSI_VERSION} REGISTRY=${REGISTRY}
```

## Prepare kind cluster

```
cd ${CSI_BAREMETAL_DIR}

# Build custom kind binary
make kind-build

# Create kind cluster
make kind-create-cluster

# Check cluster
kubectl cluster-info --context kind-kind

# If you use another path for kubeconfig or don't set KUBECONFIG env
kind get kubeconfig > <path_to_kubeconfig>

# Prepare sidecars 
# If you have sidecar images pushed into your registary
make kind-pull-sidecar-images
# If you have no ones
make deps-docker-pull
make deps-docker-tag

# Retag CSI images and load them to kind
make kind-tag-images TAG=${CSI_VERSION} REGISTRY=${REGISTRY}
make kind-load-images TAG=${CSI_VERSION} REGISTRY=${REGISTRY}
make load-operator-image OPERATOR_VERSION=${CSI_OPERATOR_VERSION} REGISTRY=${REGISTRY}
```

## Perform E2E

You can use specific versions of CSI or Operator or helm charts via editing environment variables.

Logs from `go test` executing will be saved into log.txt.

```
make test-ci CSI_VERSION=${CSI_VERSION} OPERATOR_VERSION=${CSI_OPERATOR_VERSION} CHARTS_DIR=${CSI_BAREMETAL_OPERATOR_DIR/charts}
```

## View results

```
# Install the tool
pip install junit2html

# Get a human-readable report
junit2html test/e2e/report.xml e2e_results.html
```
