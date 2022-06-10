/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/sirupsen/logrus"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
)

const (
	// NumberOfRetries to obtain Node ID
	NumberOfRetries = 20
	// DelayBetweenRetries to obtain Node ID
	DelayBetweenRetries = 3
)

// ObtainNodeIDWithRetries obtains Node ID with retries
func ObtainNodeIDWithRetries(client k8sClient.Client, featureConf featureconfig.FeatureChecker, nodeName string,
	nodeIDAnnotation string, logger *logrus.Logger, retries int, delay time.Duration) (nodeID string, err error) {
	// try to obtain node ID
	for i := 0; i < retries; i++ {
		logger.Info("Obtaining node ID...")
		if nodeID, err = GetNodeIDByName(client, nodeName, nodeIDAnnotation, "", featureConf); err == nil {
			logger.Infof("Node ID is %s", nodeID)
			return nodeID, nil
		}
		logger.Warningf("Unable to get node ID due to %v, sleep and retry...", err)
		time.Sleep(delay * time.Second)
	}
	// return empty node ID and error
	return "", fmt.Errorf("number of retries %d exceeded", retries)
}

// GetNodeIDByName return special id for k8sNode with nodeName
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func GetNodeIDByName(client k8sClient.Client, nodeName, annotationKey, nodeSelector string, featureChecker featureconfig.FeatureChecker) (string, error) {
	bmNode := nodecrd.Node{}
	if err := client.Get(context.Background(), k8sClient.ObjectKey{Name: nodeName}, &bmNode); err != nil {
		return "", err
	}

	return GetNodeID(&bmNode, annotationKey, nodeSelector, featureChecker)
}

// GetNodeID return special id for bmNode
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func GetNodeID(bmNode *nodecrd.Node, annotationKey, nodeSelector string, featureChecker featureconfig.FeatureChecker) (string, error) {
	name := bmNode.Name
	if featureChecker.IsEnabled(featureconfig.FeatureNodeIDFromAnnotation) {
		if nodeSelector != "" {
			key, value := labelStringToKV(nodeSelector)
			if val, ok := bmNode.GetLabels()[key]; !ok || val != value {
				return "", nil
			}
		}
		akey, err := chooseAnnotationKey(annotationKey, featureChecker)
		if err != nil {
			return "", err
		}

		if val, ok := bmNode.GetAnnotations()[akey]; ok {
			return val, nil
		}
		return "", fmt.Errorf("annotation %s hadn't been set for node %s", akey, name)
	}

	// use standard UID if uniq nodeID usage isn't enabled
	id := string(bmNode.UID)
	if id == "" {
		return "", fmt.Errorf("uid for node %s is not set", name)
	}

	return id, nil
}

func chooseAnnotationKey(annotationKey string, featureChecker featureconfig.FeatureChecker) (string, error) {
	if featureChecker.IsEnabled(featureconfig.FeatureExternalAnnotationForNode) {
		if annotationKey == "" {
			return "", fmt.Errorf("%s is set as True but annotation keys is empty", featureconfig.FeatureExternalAnnotationForNode)
		}

		return annotationKey, nil
	}

	return DeafultNodeIDAnnotationKey, nil
}

func labelStringToKV(payload string) (string, string) {
	data := strings.Split(payload, "=")
	if len(data) != 2 {
		return "", ""
	}
	return data[0], data[1]
}
