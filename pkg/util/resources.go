package util

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8srest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Pod is a struct for a pod
type Pod struct {
	Name     string
	NodeName string
	HostIP   string
	PodIP    string
}

// GetNodeServicePods is a function for getting pods
// which server csi node service
func GetNodeServicePods() ([]Pod, error) {
	clientset, err := getK8sClient("")

	if err != nil {
		panic(err.Error())
	}

	pods, err := clientset.CoreV1().Pods("").List(v1.ListOptions{})

	if err != nil {
		logrus.Errorf("Failed to get podes: %v", err)
	}

	temp := make([]Pod, 0)

	for i := range pods.Items {
		podName := pods.Items[i].ObjectMeta.Name
		fmt.Printf("%+v", pods.Items[i].Spec.Hostname)

		if strings.Contains(podName, "baremetal-csi") {
			temp = append(temp, Pod{
				Name:     podName,
				NodeName: pods.Items[i].Spec.NodeName,
				HostIP:   pods.Items[i].Status.HostIP,
				PodIP:    pods.Items[i].Status.PodIP,
			})
		}
	}

	logrus.Infof("Pods with plugin: %+v", temp)

	return temp, nil
}

func getK8sClient(pathToConfig string) (*kubernetes.Clientset, error) {
	// specify path to kube config if you debug
	//pathToConfig = "/home/chemaf/.kube/config"
	var (
		config *k8srest.Config
		err    error
	)

	if pathToConfig == "" {
		logrus.Info("Using in cluster config")

		config, err = k8srest.InClusterConfig()
	} else {
		logrus.Info("Using out of cluster config")

		config, err = clientcmd.BuildConfigFromFlags("", pathToConfig)
	}

	if err != nil {
		panic(err.Error())
	}

	return kubernetes.NewForConfig(config)
}
