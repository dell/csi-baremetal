package util

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const port = ":9999"

// ********** REST CLIENT part **********

// Client is a struct for HTTP server client
type Client struct {
	BaseURL   *url.URL
	UserAgent string

	httpClient *http.Client
}

// ListDisks is a function for listing disks from nodes from REST client
func (c *Client) ListDisks(host string) ([]HalDisk, error) {
	rel := &url.URL{
		Path: "/disks",
		Host: host,
	}
	u := c.BaseURL.ResolveReference(rel)
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var disks []HalDisk
	err = json.NewDecoder(resp.Body).Decode(&disks)
	return disks, err
}

// ********** REST SERVER part **********

// GetDisks is a function for getting disks from node
func GetDisks(w http.ResponseWriter, r *http.Request) {
	ll := logrus.WithField("method", "getDisks")
	w.Header().Set("Content-Type", "application/json")
	disks := AllDisks()
	ll.Info(disks)
	json.NewEncoder(w).Encode(disks)
}

// StartRest is a function for starting REST server
func StartRest() {
	r := mux.NewRouter()
	r.HandleFunc("/disks", GetDisks).Methods("GET")
	logrus.Info("Starting REST server on ", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalln(err)
	}
}

// Node is a struct for a node
type Node struct {
	Name string
	// Pod.NodeName = Node.Hostname
	Hostname string
	SpecIP   string
}

func getNodes() ([]Node, error) {
	clientset, err := getClient("")

	if err != nil {
		panic(err.Error())
	}

	nodes, err := clientset.CoreV1().Nodes().List(v1.ListOptions{})

	if err != nil {
		logrus.Errorf("Failed to get nodes: %v", err)
	}

	temp := make([]Node, 0)

	for i := range nodes.Items {
		temp = append(temp, Node{
			Name:     nodes.Items[i].ObjectMeta.Name,
			Hostname: nodes.Items[i].ObjectMeta.Labels["kubernetes.io/hostname"],
			SpecIP:   nodes.Items[i].ObjectMeta.Labels["spec.ip"],
		})
	}

	logrus.Info(temp)

	return temp, nil
}

// Pod is a struct for a pod
type Pod struct {
	Name     string
	NodeName string
	HostIP   string
	PodIP    string
}

// GetPods is a function for getting pods
func GetPods() ([]Pod, error) {
	clientset, err := getClient("")

	if err != nil {
		panic(err.Error())
	}

	pods, err := clientset.CoreV1().Pods("").List(v1.ListOptions{})

	if err != nil {
		logrus.Errorf("Failed to get nodes: %v", err)
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
	logrus.Info("Pods with plugin: ", temp)

	return temp, nil
}

func getClient(pathToConfig string) (*kubernetes.Clientset, error) {
	// specify path to kube config if you debug
	//pathToConfig = "/home/chemaf/.kube/config"
	var config *rest.Config
	var err error
	if pathToConfig == "" {
		logrus.Info("Using in cluster config")
		config, err = rest.InClusterConfig()
	} else {
		logrus.Info("Using out of cluster config")
		config, err = clientcmd.BuildConfigFromFlags("", pathToConfig)
	}

	if err != nil {
		panic(err.Error())
	}

	return kubernetes.NewForConfig(config)
}
