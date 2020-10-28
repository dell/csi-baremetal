#!/bin/bash

if [ $# -ne 2 ]; then
	echo "ERROR: node name and control-plane docker container should be provided"
	exit 1
fi

NODE=$1
CONTROL_PLANE_NODE=$2

echo "Drain node"
kubectl drain "${NODE}" --ignore-daemonsets
echo "Delete node"
kubectl delete node "${NODE}"
echo "kubeadm reset"
docker exec -it "${NODE}" kubeadm reset --force
echo "cleanup kubernetes folder"
docker exec -it "${NODE}" rm -rf /etc/kubernetes
echo "restart kubelet"
docker exec -it "${NODE}" systemctl restart kubelet
echo "ensure that node isn't in cluster"
nodeRemoved=false
for i in {1..10}; do
  res=$(kubectl get node "${NODE}" 2>&1)
  if [[ $res == *"not found" ]]; then
    echo "=== node ${NODE} was removed successfully"
    nodeRemoved=true
    break
  else
    echo "Node is still in the cluster: ${res}. Retry"
    sleep 1s
  fi
done

if [ "${nodeRemoved}" == false ]; then
  echo "ERROR: node ${NODE} wasn't removed"
  exit 2
fi

echo "getting join command"
join_command=$(docker exec -it "${CONTROL_PLANE_NODE}" sh -c 'kubeadm token create --print-join-command 2>/dev/null')
sleep 1s
join_command="${join_command/$'\r'} --ignore-preflight-errors=all"
echo "joining node ${NODE}"
docker exec -it "${NODE}" $join_command

echo "ensure node becomes ready"
for i in {1..20}; do
  res=$(kubectl get node "${NODE}" 2>&1)
  if [[ $res == *"NotReady"* ]]; then
    echo "Node hasn't ready yet: ${res}. Retry"
    sleep 1s
  else
    echo "Node ${NODE} successfully added to the cluster"
    exit 0
  fi
done

exit 3
