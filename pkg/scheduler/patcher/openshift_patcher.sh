#!/bin/sh

#  Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

IMAGE="${IMAGE:-0.0.7}"
REGISTRY="${REGISTRY:-10.244.120.194:9042}"
PORT="${PORT:-8889}"
POLICY_CONFIGMAP_NAME="${POLICY_CONFIGMAP_NAME:-scheduler-policy}"
RELEASE_NAME="${RELEASE_NAME:-scheduler-extender}"
ARTIFACTORY=10.244.120.194:8081
CSI_ARTIFACTORY_PATH=artifactory/atlantic-build/com/emc/atlantic/charts/csi
PATH_TO_CHART="${PATH_TO_CHART:-http://${ARTIFACTORY}/${CSI_ARTIFACTORY_PATH}/${IMAGE}/scheduler-extender-${IMAGE}.tgz}"

checkErr() {
	if [ $? -ne 0 ]; then
		echo Script failed: $1
		exit 1
	fi
}

function usage()
{
   cat << HEREDOC

   Usage: openshift_patcher [--help] [--install] [--remove]
   Available environment variable: IMAGE, PATH_TO_CHART, RELEASE_NAME, POLICY_CONFIGMAP_NAME, PORT.
   IMAGE - scheduler extender docker image (example: 0.0.8-245.3f9fabc)
   PATH_TO_CHART - path to scheduler extender charts (example: csi-baremetal/chart/scheduler-extender)
   RELEASE_NAME - name of scherduler extender helm release (example: scheduler-extender)
   POLICY_CONFIGMAP_NAME - name of scheduler policy ConfigMap (example: scheduler-policy)
   PORT - port, used by scheduler extender (example: 8889)
   arguments:
     -h, --help           show this help message and exit
     -i, --install        install and configure scheduler extender
     -r, --remove         remove scheduler extender

HEREDOC
}

function install() {
    helm install ${RELEASE_NAME} ${PATH_TO_CHART} --set image.tag=${IMAGE} --set registry=${REGISTRY} --set port=${PORT}
    checkErr "error during scheduler-extender installation."

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

    oc get configmap ${POLICY_CONFIGMAP_NAME} -n openshift-config 2> /dev/null
    exit_status=$?

    if [ $exit_status -eq 0 ]; then
       # If ConfigMap contains predicates and priorities we will lost them in case of removing and replacing configmap.
       # But it hard to edit ConfigMap in bash script. Should we allow user to edit ConfigMap by its own.
       oc delete configmap ${POLICY_CONFIGMAP_NAME} -n openshift-config
       checkErr "error during execution \"os delete configmap\" command."
    fi
    oc create configmap -n openshift-config --from-file=policy.cfg ${POLICY_CONFIGMAP_NAME}
    checkErr "error during execution \"oc create configmap\" command."

    oc patch Scheduler cluster --type='merge' -p '{"spec":{"policy":{"name":"'${POLICY_CONFIGMAP_NAME}'"}}}' --type=merge
    checkErr "error during execution \"oc patch Scheduler cluster\" command."

    rm policy.cfg
}

function remove() {
    helm delete ${RELEASE_NAME}
    checkErr "error during scheduler-extender removing."
    oc delete configmap ${POLICY_CONFIGMAP_NAME} -n openshift-config
    checkErr "error during execution \"os delete configmap\" command."
    oc patch Scheduler cluster --type='merge' -p '{"spec":{"policy":{"name":""}}}' --type=merge
    checkErr "error during execution \"oc patch Scheduler cluster\" command."
}
# Support only one argument per execution
case "$1" in
    -h | --help ) usage; exit; ;;
    -i | --install ) install; ;;
    -r | --remove )  remove; ;;
    * ) echo "Invalid argument"; exit 1; ;;
esac



