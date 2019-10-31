#!/bin/bash

echo "Deploy CRDs ..."
kubectl apply -f resources/crds --recursive --validate=false

echo "Deploy RBACs ..."
kubectl apply -f resources/rbac --recursive

echo "Deploy plugin (node-driver-registrar, identity and node servers) ..."
kubectl apply -f baremetal-csi-plugin.yaml

echo "Deploy plugin (provisioner, controller server) ..."
kubectl apply -f baremetal-csi-controller.yaml

echo "Create storage class ..."
kubectl apply -f resources/examples/baremetal-csi-sc.yaml
