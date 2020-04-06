package idracmgr

import "C"
import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"github.com/sirupsen/logrus"
)

const (
	storageURL = "/redfish/v1/Systems/System.Embedded.1/Storage/"
	keyURL     = "@odata.id"
)

type IDRACManager struct {
	log      *logrus.Entry
	client   *http.Client
	ip       string
	user     string
	password string
}

func NewIDRACManager(log *logrus.Logger, timeout time.Duration, user string, password string, ip string) *IDRACManager {
	//TODO AK8S-210 - Integrate CSI with Vault
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &IDRACManager{
		client:   &http.Client{Timeout: timeout, Transport: tr},
		ip:       ip,
		user:     user,
		password: password,
		log:      log.WithField("component", "IDRACManager"),
	}
}

/*contains urls of controller, enclosure etc, example @odata.id:/redfish/v1/Systems/System.Embedded.1/Storage/NonRAID.Integrated.1-1
...
Member: [{
	@odata.id:/redfish/v1/Systems/System.Embedded.1/Storage/NonRAID.Integrated.1-1}
]
...
*/
type Storage struct {
	Member []map[string]string `json:"Members"`
}

/* contains urls of drives, example @odata.id:/redfish/v1/Systems/System.Embedded.1/Storage/Drives/Disk.Bay.0:Enclosure.Internal.0-1:NonRAID.Integrated.1-1
...
Drives: [{
 	@odata.id:/redfish/v1/Systems/System.Embedded.1/Storage/Drives/Disk.Bay.0:Enclosure.Internal.0-1:NonRAID.Integrated.1-1
}]
...
*/
type Controller struct {
	Drive []map[string]string `json:"Drives"`
}

//container info about drive
type IDRACDrive struct {
	Status        map[string]string `json:"Status"`
	ID            string            `json:"Id"`
	SerialNumber  string            `json:"SerialNumber"`
	CapacityBytes int64             `json:"CapacityBytes"`
	MediaType     string            `json:"MediaType"`
	Manufacturer  string            `json:"Manufacturer"`
	Protocol      string            `json:"Protocol"`
	Model         string            `json:"Model"`
}

func (mgr *IDRACManager) GetDrivesList() ([]*api.Drive, error) {
	controllerURL := mgr.getControllerURLs()
	if len(controllerURL) == 0 {
		return nil, errors.New("unable to inspect iDRAC controller")
	}
	drivesURL := make([]string, 0)
	for _, c := range controllerURL {
		drivesURL = append(drivesURL, mgr.getDrivesURLs(c)...)
	}
	if len(drivesURL) == 0 {
		return nil, errors.New("unable to inspect iDRAC drives")
	}
	drives := make([]*api.Drive, 0)
	for _, driveURL := range drivesURL {
		drive := mgr.getDrive(driveURL)
		if drive != nil {
			drives = append(drives, drive)
		}
	}
	return drives, nil
}

//getControllerURLs returns slice of all controllers url in Storage
func (mgr *IDRACManager) getControllerURLs() []string {
	endpoint := fmt.Sprintf("https://%s%s", mgr.ip, storageURL)
	controllerURLs := make([]string, 0)
	response, err := mgr.doRequest(endpoint)
	if err != nil {
		mgr.log.Errorf("Couldn't create storage request %s to IDRAC: %v", endpoint, err)
		return controllerURLs
	}
	if response != nil {
		defer func() {
			if err != response.Body.Close() {
				mgr.log.Errorf("Fail to close connection url: %s, err: %v", endpoint, err)
			}
		}()
	}
	var storage Storage
	if err := json.NewDecoder(response.Body).Decode(&storage); err != nil {
		mgr.log.Errorf("Fail to convert to storage struct, %v", err)
		return controllerURLs
	}
	for _, member := range storage.Member {
		//getting url of all controllers or enclosure in storage
		controllerURLs = append(controllerURLs, fmt.Sprintf("https://%s%s", mgr.ip, member[keyURL]))
	}
	return controllerURLs
}

//getDrivesURLs returns slice of all drive url in Controller with controllerURL
func (mgr *IDRACManager) getDrivesURLs(controllerURL string) []string {
	driveURLs := make([]string, 0)
	// get information about controllers using url from Storage
	response, err := mgr.doRequest(controllerURL)
	if response != nil {
		defer func() {
			if err != response.Body.Close() {
				mgr.log.Errorf("Fail to close connection url: %s, err: %v", controllerURL, err)
			}
		}()
	}
	if err != nil {
		mgr.log.Errorf("Couldn't create controller request %s to IDRAC: %v", controllerURL, err)
		return driveURLs
	}
	var controller Controller
	if err := json.NewDecoder(response.Body).Decode(&controller); err != nil {
		mgr.log.Errorf("Fail to convert to controller struct, %v", err)
		return driveURLs
	}
	for _, odata := range controller.Drive {
		//getting url of all drive in controller or enclosure
		driveURLs = append(driveURLs, fmt.Sprintf("https://%s%s", mgr.ip, odata[keyURL]))
	}
	return driveURLs
}

//getDrive returns api.Drive with information from IDRAC drive with url driveURL
func (mgr *IDRACManager) getDrive(driveURL string) *api.Drive {
	response, err := mgr.doRequest(driveURL)
	if err != nil {
		mgr.log.Errorf("Couldn't create drive %s request to IDRAC: %v", driveURL, err)
		return nil
	}
	if response != nil {
		defer func() {
			if err != response.Body.Close() {
				mgr.log.Errorf("Fail to close connection, url: %s, err: %v", driveURL, err)
			}
		}()
	}
	var drive IDRACDrive
	if err := json.NewDecoder(response.Body).Decode(&drive); err != nil {
		mgr.log.Errorf("Fail to convert to IDRACDrive struct, err: %v", err)
		return nil
	}
	var diskType api.DriveType
	if drive.Protocol == "NVMe" {
		diskType = api.DriveType_NVMe
	} else {
		diskType = convertMediaType(drive.MediaType)
	}
	apiDrive := &api.Drive{
		VID:          drive.Manufacturer,
		PID:          drive.Model,
		SerialNumber: drive.SerialNumber,
		Health:       convertDriveHealth(drive.Status["Health"]),
		Type:         diskType,
		Size:         drive.CapacityBytes,
		Status:       api.Status_ONLINE,
	}
	return apiDrive
}

func (mgr *IDRACManager) doRequest(url string) (*http.Response, error) {
	mgr.log.Info("Connecting to IDRAC with url ", url)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(mgr.user, mgr.password)
	request.Header.Add("Accept", "application/json")
	response, err := mgr.client.Do(request)
	if err != nil {
		return nil, err
	}
	return response, err
}

func convertDriveHealth(health string) api.Health {
	switch health {
	case "OK":
		return api.Health_GOOD
	// shouldn't it be SUSPECT?
	case "Warning":
		return api.Health_BAD
	case "Critical":
		return api.Health_BAD
	default:
		return api.Health_UNKNOWN
	}
}

func convertMediaType(mediaType string) api.DriveType {
	switch mediaType {
	case "HDD":
		return api.DriveType_HDD
	case "SSD":
		return api.DriveType_SSD
	default:
		return api.DriveType_HDD
	}
}
