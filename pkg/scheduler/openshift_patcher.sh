#!/bin/sh

IMAGE=0.0.7
REGISTRY=10.244.120.194:9042
PORT=8889
POLICY_CONFIGMAP_NAME=scheduler-policy
RELEASE_NAME=scheduler-extender
ARTIFACTORY=10.244.120.194:8081
CSI_ARTIFACTORY_PATH=artifactory/atlantic-build/com/emc/atlantic/charts/csi

helm install ${RELEASE_NAME} http://${ARTIFACTORY}/${CSI_ARTIFACTORY_PATH}/${IMAGE}/scheduler-extender-${IMAGE}.tgz \
--set image.tag=${IMAGE} --set registry=${REGISTRY} --set port=${PORT}

cat <<EOF >./policy.cfg
{
   "kind" : "Policy",
   "apiVersion" : "v1",
   "extenders": [
        {
            "urlPrefix": "http://127.0.0.1:$PORT",
            "filterVerb": "filter",
            "enableHttps": false,
            "nodeCacheCapable": false,
            "ignorable": true
        }
    ]
}
EOF

oc get configmap ${POLICY_CONFIGMAP_NAME} -n openshift-config
exit_status=$?

if [ $exit_status -eq 0 ]; then
   # If ConfigMap contains predicates and priorities we will lost them in case of removing and replacing configmap.
   # But it hard to edit ConfigMap in bash script. Should we allow user to edit ConfigMap by its own.
   oc delete configmap ${POLICY_CONFIGMAP_NAME} -n openshift-config
fi
oc create configmap -n openshift-config --from-file=policy.cfg ${POLICY_CONFIGMAP_NAME}
oc patch Scheduler cluster --type='merge' -p '{"spec":{"policy":{"name":"'${POLICY_CONFIGMAP_NAME}'"}}}' --type=merge

rm policy.cfg


