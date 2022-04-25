package util

import (
	"fmt"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	annotations "github.com/dell/csi-baremetal/pkg/crcontrollers/node/common"
	"github.com/sirupsen/logrus"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	numberOfRetries  = 20
	delayBeforeRetry = 5
)

func ObtainNodeIDWithRetries(client k8sClient.Client, featureConf featureconfig.FeatureChecker,
	nodeName string, nodeIDAnnotation string, logger *logrus.Logger) (nodeID string, err error) {
	// try to obtain node ID
	for i := 0; i < numberOfRetries; i++ {
		logger.Info("Obtaining node ID...")
		if nodeID, err = annotations.GetNodeIDByName(client, nodeName, nodeIDAnnotation, "", featureConf); err == nil {
			logger.Infof("Node ID is %s", nodeID)
			return nodeID, nil
		}
		logger.Warningf("Unable to get node ID due to %v, sleep and retry...", err)
		time.Sleep(delayBeforeRetry * time.Second)
	}
	// return empty node ID and error
	return "", fmt.Errorf("number of retries %d exceeded", numberOfRetries)
}
