package storagemanager

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/rest"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/sirupsen/logrus"
)

// implement StorageSubsystem interface
type LVMVolumeManager struct {
	LVMTopology        StorageTopology
	NodesCommunicators map[string]*LVMRestCommunicator
	Initialized        bool
	lvNameProducer     nameGen
}

// implement Communicator interface
type LVMRestCommunicator struct {
	client *rest.RClient
}

// generates names for LV TODO: do we need make it thread safety?
type nameGen struct {
	val int32
}

func (c *nameGen) GetName() string {
	c.val++
	return fmt.Sprintf("lv0%d", c.val)
}

// fill NodesCommunicators map with keys as a node hostname and value - appropriate HTTPClient
func (l *LVMVolumeManager) initCommunicators() error {
	ll := logrus.WithField("method", "LVMVolumeManager.initCommunicators")
	ll.Infof("Initialization")
	pods, err := util.GetNodeServicePods()
	if err != nil {
		ll.Errorf("Could not get pods with node plugin. Error: %v", err)
		return err
	}

	l.NodesCommunicators = make(map[string]*LVMRestCommunicator, len(pods))
	for _, pod := range pods {
		rc := &rest.RClient{
			URL:        fmt.Sprintf("http://%s:%s", pod.PodIP, "9999"),
			HTTPClient: &http.Client{Timeout: time.Second * 10},
		}
		l.NodesCommunicators[pod.NodeName] = &LVMRestCommunicator{
			client: rc,
		}
		ll.Infof("Communicator for node %s has created", pod.NodeName)
	}

	ll.Infof("Communicators were successfully initialized.")
	l.Initialized = true

	return nil
}

// create logical volume with appropriate size in preferredNode (if provided)
func (l *LVMVolumeManager) PrepareVolume(capacityGb float64, preferredNode string) (node string, volumeID string, err error) {
	ll := logrus.WithField("method", "LVMVolumeManager.PrepareVolume")
	if !l.Initialized {
		ll.Infof("API Communicators were not initialized. Initializing ...")
		err := l.initCommunicators()
		if err != nil {
			ll.Errorf("Failed to initialize API Communicators")
			return "", "", err
		}
	}
	var nc *LVMRestCommunicator
	if preferredNode != "" {
		if _, ok := l.NodesCommunicators[preferredNode]; !ok {
			return "", "", fmt.Errorf("there is no communicator for node %s, could not create volume on it", preferredNode)
		}
		node = preferredNode
	} else {
		// search appropriate node here using LVMTopology
		ll.Infof("preferredNode was not provided, search node ...")
		appropriateNode := make([]string, 0)
		t := l.GetStorageTopology(true) // TODO: do not collect each time
		for n, vgInterface := range t {
			if vgInterface.(lvm.VolumeGroup).FreeSizeInGb > capacityGb {
				appropriateNode = append(appropriateNode, n)
			}
		}
		if len(appropriateNode) == 0 {
			ll.Errorf("There is no node with enough capacity(%fG).", capacityGb)
			return "", "", fmt.Errorf("there is no node with enough capacity")
		}
		// get random node
		r := rand.New(rand.NewSource(time.Now().Unix()))
		i := r.Intn(len(appropriateNode))
		node = appropriateNode[i]
		ll.Infof("choose node %s", node)
	}
	nc = l.NodesCommunicators[node]

	// check that appropriate node has enough capacity
	vgInfo, err := nc.GetNodeStorageInfo()
	if err != nil {
		ll.Errorf("Could not get VG info from node %s.", node)
		return "", "", err
	}

	if vgInfo != nil && vgInfo.(lvm.VolumeGroup).FreeSizeInGb < capacityGb {
		logrus.Errorf("There is not enough capacity on node %s, capacity required %f but actual - %f",
			node, capacityGb, vgInfo.(lvm.VolumeGroup).FreeSizeInGb)
	}

	// create LV
	var vID string
	vID, err = nc.PrepareVolumeOnNode(VolumeInfo{CapacityGb: capacityGb, Name: l.lvNameProducer.GetName()})
	if err != nil {
		ll.Errorf("Could not Create LV on node %s. Error: %v", node, err)
		return "", "", err
	}
	volumeID = fmt.Sprintf("%s_%s", node, vID)
	ll.Infof("Returning: node %s, volumeID %s", node, volumeID)
	return node, volumeID, nil
}

// completely remove logical volume in node
func (l *LVMVolumeManager) ReleaseVolume(node string, volumeID string) error {
	ll := logrus.WithField("method", "LVMVolumeManager.ReleaseVolume")
	s := strings.Split(volumeID, "_") // TODO: handle index out of range error or implement struct for ID
	vgName, lvName := s[1], s[2]
	vi := VolumeInfo{
		Name: fmt.Sprintf("%s/%s", vgName, lvName),
	}
	ll.Infof("Releasing volume with id %s. Volume Info %v", volumeID, vi)
	err := l.NodesCommunicators[node].ReleaseVolumeOnNode(vi)
	if err != nil {
		ll.Errorf("Could not release volume %s. Error: %v", volumeID, err)
		return err
	}
	ll.Infof("Volume with ID %s was successfully removed from node %s", volumeID, node)

	return nil
}

// return map with keys - node hostname, value - VolumeGroup object
func (l *LVMVolumeManager) GetStorageTopology(collect bool) StorageTopology {
	ll := logrus.WithField("method", "LVMVolumeManager.GetStorageTopology").WithField("collect info", collect)
	if !collect {
		ll.Info("Returning LVMTopology from cache")
		return l.LVMTopology
	}

	var t = make(StorageTopology)

	for node, com := range l.NodesCommunicators {
		vg, err := com.GetNodeStorageInfo()
		if err != nil {
			ll.Errorf("Could not get storage info from node %s.", node)
		}
		if vgObj, ok := vg.(lvm.VolumeGroup); ok {
			t[node] = vgObj
		} else {
			ll.Errorf("got %v, expect VolumeGroup interface", vg)
		}
	}
	if len(t) == 0 {
		logrus.Fatalf("Could not collect storage topology")
	}
	l.LVMTopology = t
	return t
}

func (l *LVMVolumeManager) IsInitialized() bool {
	return l.Initialized
}

// return interface of util.VolumeGroup object
func (lrc *LVMRestCommunicator) GetNodeStorageInfo() (interface{}, error) {
	vg, err := lrc.client.GetVolumeGroupRequest()
	if err != nil {
		return nil, err
	}
	return vg, nil
}

// create Logical Volume on node (uses rest call), VolumeInfo should contain only name
func (lrc *LVMRestCommunicator) PrepareVolumeOnNode(vi VolumeInfo) (volumeID string, err error) {
	ll := logrus.WithField("method", "PrepareVolumeOnNode")
	ll.Infof("Preparing volume,  size of %fG", vi.CapacityGb)

	// TODO: search VG nameGen
	lv := lvm.LogicalVolume{
		Name:   vi.Name,
		VGName: rest.VgName, // TODO: it should be provided in VolumeInfo
		LVSize: fmt.Sprintf("%fG", vi.CapacityGb),
	}
	err = lrc.client.CreateLogicalVolumeRequest(lv)
	if err != nil {
		ll.Errorf("Could not create LV %v from VolumeInfo %v. Error: %v", lv, vi, err)
		return "", err
	}
	volumeID = fmt.Sprintf("%s_%s", lv.VGName, lv.Name)
	return volumeID, err
}

// remove Logical Volume on node (uses rest call)
func (lrc *LVMRestCommunicator) ReleaseVolumeOnNode(vi VolumeInfo) error {
	ll := logrus.WithField("method", "ReleaseVolumeOnNode")
	ll.Infof("Got request: %v", vi)

	lv := lvm.LogicalVolume{
		Name:   strings.Split(vi.Name, "/")[1], // TODO: handle index out of range error or implement struct for ID
		VGName: strings.Split(vi.Name, "/")[0],
	}

	err := lrc.client.RemoveLogicalVolumeRequest(lv)
	if err != nil {
		ll.Errorf("Could not remove volume %v. Error: %v", vi, err)
		return err
	}
	ll.Infof("Volume %v was successfully removed", vi)

	return nil
}
