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
	corev1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
)

type service struct {
	client            k8sClient.Client
	log               *logrus.Logger
	featureConfig     featureconfig.FeatureChecker
	delayBetweenRetry time.Duration
	numberOfRetry     int
}

// NodeAnnotation represent annotation service
type NodeAnnotation interface {
	ObtainNodeID(nodeName, nodeIDAnnotation string) (nodeID string, err error)
	GetNodeID(node interface{}, annotationKey, nodeSelector string) (string, error)
	GetNodeIDFromK8s(ctx context.Context, nodeName, annotationKey, nodeSelector string) (string, error)
	GetNodeIDFromCRD(ctx context.Context, nodeName, annotationKey, nodeSelector string) (string, error)
}

// New return NodeAnnotation service
func New(client k8sClient.Client, featureConf featureconfig.FeatureChecker, log *logrus.Logger, options ...func(*service)) NodeAnnotation {
	srv := &service{
		client:        client,
		featureConfig: featureConf,
		log:           log,
	}
	for _, o := range options {
		o(srv)
	}
	return srv
}

// WithRetryNumber declare number of retries for ObtainNodeID
func WithRetryNumber(count int) func(*service) {
	return func(s *service) {
		s.numberOfRetry = count
	}
}

// WithRetryDelay declare delay between retries for ObtainNodeID
func WithRetryDelay(delay time.Duration) func(*service) {
	return func(s *service) {
		s.delayBetweenRetry = delay
	}
}

// A node interface with common methods
type abstractNode interface {
	GetLabels() map[string]string
	GetAnnotations() map[string]string
}

// ObtainNodeID obtains Node ID with retries
func (srv *service) ObtainNodeID(nodeName, nodeIDAnnotation string) (nodeID string, err error) {
	ctx := context.Background()
	// try to obtain node ID
	for i := 0; i < srv.numberOfRetry; i++ {
		srv.log.Info("Obtaining node ID...")
		if nodeID, err = srv.GetNodeIDFromCRD(ctx, nodeName, nodeIDAnnotation, ""); err == nil {
			srv.log.Infof("Node ID is %s", nodeID)
			return nodeID, nil
		}
		srv.log.Warningf("Unable to get node ID name:%s annotation:%s due to %v, sleep and retry...", nodeName, nodeIDAnnotation, err)
		time.Sleep(srv.delayBetweenRetry)
	}
	// return empty node ID and error
	return "", fmt.Errorf("number of retries %d exceeded", srv.numberOfRetry)
}

// GetNodeIDFromCRD return special id for node from nodecrd.Node
func (srv *service) GetNodeIDFromCRD(ctx context.Context, nodeName, annotationKey, nodeSelector string) (string, error) {
	bmNode := &nodecrd.Node{}
	if err := srv.client.Get(ctx, k8sClient.ObjectKeyFromObject(bmNode), bmNode); err != nil {
		return "", err
	}
	return srv.GetNodeID(bmNode, annotationKey, nodeSelector)
}

// GetNodeIDFromK8s return special id for k8sNode with nodeName
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func (srv *service) GetNodeIDFromK8s(ctx context.Context, nodeName, annotationKey, nodeSelector string) (string, error) {
	k8sNode := &corev1.Node{}
	if err := srv.client.Get(ctx, k8sClient.ObjectKey{Name: nodeName}, k8sNode); err != nil {
		return "", err
	}
	return srv.GetNodeID(k8sNode, annotationKey, nodeSelector)
}

// GetNodeID return special id for node either from nodecrd.Node and corev1.Node
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func (srv *service) GetNodeID(node interface{}, annotationKey, nodeSelector string) (string, error) {
	nodeName, id := "", ""
	switch v := node.(type) {
	case *corev1.Node:
		nodeName, id = v.Name, string(v.UID)
	case *nodecrd.Node:
		nodeName, id = v.Name, string(v.UID)
	default:
		return "", fmt.Errorf("unknown type of node:%T", node)
	}
	if srv.featureConfig.IsEnabled(featureconfig.FeatureNodeIDFromAnnotation) {
		if nodeSelector != "" {
			key, value := labelStringToKV(nodeSelector)
			if val, ok := node.(abstractNode).GetLabels()[key]; !ok || val != value {
				return "", nil
			}
		}
		akey, err := srv.chooseAnnotationKey(annotationKey)
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

func (srv *service) chooseAnnotationKey(annotationKey string) (string, error) {
	if srv.featureConfig.IsEnabled(featureconfig.FeatureExternalAnnotationForNode) {
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
