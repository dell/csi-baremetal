#!/bin/bash

echo "Remove plugin (node-driver-registrar, identity and node servers) ..."
kubectl delete -f baremetal-csi-plugin.yaml

echo "Remove plugin (provisioner, controller server) ..."
kubectl delete -f baremetal-csi-controller.yaml

echo "Remove RBACs ..."
kubectl delete -f resources/rbac --recursive

echo "Remove CRDs ..."
kubectl delete -f resources/crds --recursive

echo "Remove sockets from nodes ..."
for i in $(kubectl get nodes -o wide --no-headers | grep -w Ready |awk '{print $6}'); do
    ssh root@$i "rm -rf /var/lib/kubelet/plugins/* && rm -rf /var/lib/kubelet/plugins_registry/*"
done

echo "Remove PVC, PV, SC from Examples"
kubectl delete -f resources/examples/csi-app.yaml
kubectl delete -f resources/examples/baremetal-csi-pvc.yaml
kubectl delete -f resources/examples/baremetal-csi-sc.yaml
