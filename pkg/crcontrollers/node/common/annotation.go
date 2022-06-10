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
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
)

const (
	// NumberOfRetries to obtain Node ID
	NumberOfRetries = 20
	// DelayBetweenRetries to obtain Node ID
	DelayBetweenRetries = 3
)

// A node interface with common methods
type abstractNode interface {
	GetLabels() map[string]string
	GetAnnotations() map[string]string
}

// ObtainNodeIDWithRetries obtains Node ID with retries
func ObtainNodeIDWithRetries(client *k8s.KubeClient, featureConf featureconfig.FeatureChecker, nodeName string,
	nodeIDAnnotation string, logger *logrus.Logger, retries int, delay time.Duration) (nodeID string, err error) {
	// try to obtain node ID
	for i := 0; i < retries; i++ {
		logger.Info("Obtaining node ID...")
		if nodeID, err = GetNodeIDFromCRD(client, nodeName, nodeIDAnnotation, "", featureConf); err == nil {
			logger.Infof("Node ID is %s", nodeID)
			return nodeID, nil
		}
		logger.Warningf("Unable to get node ID due to %v, sleep and retry...", err)
		time.Sleep(delay * time.Second)
	}
	// return empty node ID and error
	return "", fmt.Errorf("number of retries %d exceeded", retries)
}

// GetNodeIDFromCRD return special id for node from nodecrd.Node
func GetNodeIDFromCRD(client *k8s.KubeClient, nodeName, annotationKey, nodeSelector string, featureChecker featureconfig.FeatureChecker) (string, error) {
	bmNode := &nodecrd.Node{}
	err := client.ReadCR(context.Background(), nodeName, "", bmNode)
	if err != nil && !k8sError.IsNotFound(err) {
		return "", err
	}
	return GetNodeID(bmNode, annotationKey, nodeSelector, featureChecker)
}

// GetNodeIDFromK8s return special id for k8sNode with nodeName
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func GetNodeIDFromK8s(client k8sClient.Client, nodeName, annotationKey, nodeSelector string, featureChecker featureconfig.FeatureChecker) (string, error) {
	k8sNode := &corev1.Node{}
	if err := client.Get(context.Background(), k8sClient.ObjectKey{Name: nodeName}, k8sNode); err != nil {
		return "", err
	}
	return GetNodeID(k8sNode, annotationKey, nodeSelector, featureChecker)
}

// GetNodeID return special id for node either from nodecrd.Node and corev1.Node
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func GetNodeID(node interface{}, annotationKey, nodeSelector string, featureChecker featureconfig.FeatureChecker) (string, error) {
	nodeName, id := "", ""
	switch v := node.(type) {
	case *corev1.Node:
		nodeName, id = v.Name, string(v.UID)
	case *nodecrd.Node:
		nodeName, id = v.Name, string(v.UID)
	default:
		return "", fmt.Errorf("unknown type of node:%T", node)
	}
	if featureChecker.IsEnabled(featureconfig.FeatureNodeIDFromAnnotation) {
		if nodeSelector != "" {
			key, value := labelStringToKV(nodeSelector)
			if val, ok := node.(abstractNode).GetLabels()[key]; !ok || val != value {
				return "", nil
			}
		}
		akey, err := chooseAnnotationKey(annotationKey, featureChecker)
		if err != nil {
			return "", err
		}

		if val, ok := node.(abstractNode).GetAnnotations()[akey]; ok {
			return val, nil
		}
		return "", fmt.Errorf("annotation %s hadn't been set for node %s", akey, nodeName)
	}
	if id == "" {
		return "", fmt.Errorf("uid for node %s is not set", nodeName)
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
