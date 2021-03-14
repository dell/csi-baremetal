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

	corev1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
)

// GetNodeIDByName return special id for k8sNode with nodeName
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func GetNodeIDByName(client k8sClient.Client, nodeName string, annotationKey string, featureChecker featureconfig.FeatureChecker) (string, error) {
	k8sNode := corev1.Node{}
	if err := client.Get(context.Background(), k8sClient.ObjectKey{Name: nodeName}, &k8sNode); err != nil {
		return "", err
	}

	return GetNodeID(k8sNode, annotationKey, featureChecker)
}

// GetNodeID return special id for k8sNode
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func GetNodeID(k8sNode corev1.Node, annotationKey string, featureChecker featureconfig.FeatureChecker) (string, error) {
	if featureChecker.IsEnabled(featureconfig.FeatureNodeIDFromAnnotation) {
		akey, err := chooseAnnotaionKey(annotationKey, featureChecker)
		if err != nil {
			return "", err
		}

		if val, ok := k8sNode.GetAnnotations()[akey]; ok {
			return val, nil
		}
		return "", fmt.Errorf("annotation %s hadn't been set for node %s", akey, k8sNode.Name)
	}

	// use standard UID if uniq nodeID usage isn't enabled
	return string(k8sNode.UID), nil
}

func chooseAnnotaionKey(annotationKey string, featureChecker featureconfig.FeatureChecker) (string, error) {
	if featureChecker.IsEnabled(featureconfig.FeatureExternalAnnotationForNode) {
		if annotationKey == "" {
			return "", fmt.Errorf("%s is set as True but ", featureconfig.FeatureExternalAnnotationForNode)
		}

		return annotationKey, nil
	}

	return DeafultNodeIDAnnotationKey, nil
}
