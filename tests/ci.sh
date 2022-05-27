#!/bin/bash
if [ $# -gt 1 ]; then
    echo "deploy.sh <number of devices>"
fi

# TODO check for REGISTRY env var
export REGISTRY=asdrepo.isus.emc.com:9042

NUMBER_OF_DEVICES=3
if [ $# -eq 1 ]; then
    NUMBER_OF_DEVICES=$1
fi
echo "Number of loopback devices per node:" $NUMBER_OF_DEVICES


# build binaries
make DRIVE_MANAGER_TYPE=loopbackmgr build
if [ $? -ne 0 ]; then
    echo "Failed to build binaries"
    exit 1
fi

# build final images
make DRIVE_MANAGER_TYPE=loopbackmgr images
if [ $? -ne 0 ]; then
    echo "Failed to build final images"
    exit 1
fi

# pull sidecar images
make kind-pull-sidecar-images
if [ $? -ne 0 ]; then
    echo "Failed to build final images"
    exit 1
fi

# load to kind cluster
make kind-tag-images
if [ $? -ne 0 ]; then
    echo "Failed to tag images"
    exit 1
fi

make kind-load-images KIND=`which kind`
if [ $? -ne 0 ]; then
    echo "Failed to load images"
    exit 1
fi

export OPERATOR_VERSION=`cd ../csi-baremetal-operator && make version`
make load-operator-image KIND=`which kind`
if [ $? -ne 0 ]; then
    echo "Failed to load operator image"
    exit 1
fi

echo export CSI_VERSION=`make version`
echo export CSI_OPERATOR_VERSION=$OPERATOR_VERSION
#export SHORT_CI_TIMEOUT=20m
echo export CSI_CHARTS_PATH=../csi-baremetal-operator/charts

#CI=true CSI_VERSION=${CSI_VERSION} OPERATOR_VERSION=${OPERATOR_VERSION} go test -v test/e2e/baremetal_e2e_test.go -ginkgo.v -ginkgo.progress -ginkgo.failFast -all-tests -kubeconfig=${HOME}/.kube/config -chartsDir ${CHARTS_DIR}
