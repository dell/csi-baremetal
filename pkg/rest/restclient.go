package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/sirupsen/logrus"
)

// HTTPClient is a struct for HTTP server client
type RClient struct {
	URL        string // protocol://ip:port
	HTTPClient *http.Client
}

// TODO: unused method
// ListDisks is a function for listing disks from nodes from REST client
func (c *RClient) ListDisks(host string) ([]util.HalDisk, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/disks", c.URL), nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTPClient.Do(req)
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()

	if err != nil {
		return nil, err
	}

	var disks []util.HalDisk
	err = json.NewDecoder(resp.Body).Decode(&disks)

	return disks, err
}

func (c *RClient) CreateLogicalVolumeRequest(lv lvm.LogicalVolume) error {
	ll := logrus.WithField("method", "CreateLogicalVolumeRequest")

	lvReqBody, err := json.Marshal(lv)
	if err != nil {
		return fmt.Errorf("could not marshal lvBody")
	}

	endpoint := fmt.Sprintf("%s/lv", c.URL)
	ll.Infof("Creating lv. PUT %s %v ", endpoint, lv)
	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(lvReqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return err
	}
	ll.Infof("Response %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CreateLogicalVolumeRequest for lv %v. Retunted code %d", lv, resp.StatusCode)
	}
	return nil
}

func (c *RClient) RemoveLogicalVolumeRequest(lv lvm.LogicalVolume) error {
	ll := logrus.WithField("method", "RemoveLogicalVolumeRequest")

	lvReqBody, err := json.Marshal(lv)
	if err != nil {
		return fmt.Errorf("could not marshal lvBody")
	}

	endpoint := fmt.Sprintf("%s/lv", c.URL)
	ll.Infof("Removing lv: %v. DELETE %s", lv, endpoint)
	req, err := http.NewRequest("DELETE", endpoint, bytes.NewBuffer(lvReqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return err
	}
	ll.Infof("Response %d", resp.StatusCode)
	return nil
}

func (c *RClient) GetVolumeGroupRequest() (lvm.VolumeGroup, error) {
	ll := logrus.WithField("method", "GetVolumeGroupRequest")
	endpoint := fmt.Sprintf("%s/vg", c.URL)
	vg := lvm.VolumeGroup{}
	ll.Infof("GET %s", endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return vg, err
	}

	resp, err := c.HTTPClient.Do(req)
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		ll.Errorf("GET endpoint %s failed with error %v", endpoint, err)
	}

	data, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(data, &vg)
	if err != nil {
		ll.Errorf("Could not unmarshal response to VolumeGroup object. Error %v.", err)
		return vg, err
	}
	ll.Infof("GET %s, Status 200, VolumeGroup: %v", endpoint, vg)
	return vg, nil
}
