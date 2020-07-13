package idracmgr

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	api "github.com/dell/csi-baremetal/api/v1"
)

var logger = logrus.New()

func Test_convertMediaType(t *testing.T) {
	mediaType := convertMediaType("SSD")
	assert.Equal(t, api.DriveTypeSSD, mediaType)
	mediaType = convertMediaType("HDD")
	assert.Equal(t, api.DriveTypeHDD, mediaType)
	mediaType = convertMediaType("default")
	assert.Equal(t, api.DriveTypeHDD, mediaType)
}

func Test_convertDriveHealth(t *testing.T) {
	health := convertDriveHealth("OK")
	assert.Equal(t, api.HealthGood, health)
	health = convertDriveHealth("Warning")
	assert.Equal(t, api.HealthBad, health)
	health = convertDriveHealth("default")
	assert.Equal(t, api.HealthUnknown, health)
	health = convertDriveHealth("Critical")
	assert.Equal(t, api.HealthBad, health)
}

func TestNewIDRACManager(t *testing.T) {
	idracManager := NewIDRACManager(logger, time.Second, "user", "password", "10.10.10.10")
	assert.Equal(t, "user", idracManager.user)
	assert.Equal(t, "password", idracManager.password)
	assert.Equal(t, "10.10.10.10", idracManager.ip)
	assert.Equal(t, time.Second, idracManager.client.Timeout)

	idracManager = NewIDRACManager(logger, time.Second*10, "", "", "")
	assert.Equal(t, "", idracManager.user)
	assert.Equal(t, "", idracManager.password)
	assert.Equal(t, "", idracManager.ip)
	assert.Equal(t, time.Second*10, idracManager.client.Timeout)
}

func Test_doRequest(t *testing.T) {
	idracManager := NewIDRACManager(logger, time.Second, "user", "password", "10.10.10.10")
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(200)
		rw.Write([]byte(`OK`))
	}))
	defer server.Close()
	idracManager.client = server.Client()
	response, err := idracManager.doRequest(server.URL)
	assert.NotNil(t, response)
	assert.Nil(t, err)
	body, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.Equal(t, "OK", string(body))
	assert.Equal(t, response.StatusCode, 200)

	response, err = idracManager.doRequest("/url")
	assert.Nil(t, response)
	assert.NotNil(t, err)
}

func Test_getDrivesURL(t *testing.T) {
	idracManager := NewIDRACManager(logger, time.Second, "user", "password", "127.0.0.1")
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(200)
		rw.Write([]byte(`{"Drives": [{"@odata.id":"/redfish/v1/Systems/System.Embedded.1/Storage/Drives/Disk.Bay.0:Enclosure.Internal.0-1:NonRAID.Integrated.1-1"}]}`))
	}))
	defer server.Close()
	idracManager.client = server.Client()
	drivesURL := idracManager.getDrivesURLs(server.URL)
	assert.Equal(t, 1, len(drivesURL))
	assert.Equal(t, "https://127.0.0.1/redfish/v1/Systems/System.Embedded.1/Storage/Drives/Disk.Bay.0:Enclosure.Internal.0-1:NonRAID.Integrated.1-1", drivesURL[0])

	server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		//Error in json
		rw.WriteHeader(200)
		rw.Write([]byte(`{"Drive": [{"@odata.id":"/redfish/v1/Systems/System.Embedded.1/Storage/Drives/Disk.Bay.0:Enclosure.Internal.0-1:NonRAID.Integrated.1-1}]"`))
	}))
	defer server.Close()
	idracManager.client = server.Client()
	drivesURL = idracManager.getDrivesURLs(server.URL)
	assert.Empty(t, drivesURL)

	server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		//No Drive field
		rw.WriteHeader(200)
		rw.Write([]byte(`{"Id": "string"}`))
	}))
	idracManager.client = server.Client()
	drivesURL = idracManager.getDrivesURLs(server.URL)
	assert.Empty(t, drivesURL)
}

func Test_getDrive(t *testing.T) {
	idracManager := NewIDRACManager(logger, time.Second, "user", "password", "127.0.0.1")
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(200)
		rw.Write([]byte(`{"SerialNumber": "serialNumber"}`))
	}))
	defer server.Close()
	idracManager.client = server.Client()
	drive := idracManager.getDrive(server.URL)
	assert.Equal(t, "serialNumber", drive.SerialNumber)

	server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		//Error in Json
		rw.WriteHeader(200)
		rw.Write([]byte(`{"SerialNumber": "serialNumber"`))
	}))
	defer server.Close()
	idracManager.client = server.Client()
	drive = idracManager.getDrive(server.URL)
	assert.Nil(t, drive)
}
