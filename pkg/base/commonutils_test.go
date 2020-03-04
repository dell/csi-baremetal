package base

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"github.com/stretchr/testify/assert"
)

const tmpMounts = "/tmp/mounts"

func TestConsistentReadSuccess(t *testing.T) {
	// happy path
	fileContent := "some content"
	file, _ := os.Create(tmpMounts)
	_, _ = file.WriteString(fileContent)
	content, err := ConsistentRead(tmpMounts, 5, 5*time.Millisecond)
	assert.Nil(t, err)
	assert.Equal(t, fileContent, string(content))

	// read at second time
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		content, err = ConsistentRead(tmpMounts, 2, 100*time.Millisecond)
		wg.Done()
	}()
	time.Sleep(20 * time.Millisecond)
	newContent := "new content"
	_, _ = file.WriteString(newContent)
	wg.Wait()
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s%s", fileContent, newContent), string(content))
}

func TestConsistentReadFail(t *testing.T) {
	// file does not exist
	content, err := ConsistentRead("/tmp/bla-bla-bla", 1, time.Millisecond)
	assert.Nil(t, content)
	assert.NotNil(t, err)

	fileContent := "some content"
	file, _ := os.Create(tmpMounts)
	_, _ = file.WriteString(fileContent)

	// second time read fail
	newContent := "new content"
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		content, err = ConsistentRead(tmpMounts, 2, 100*time.Millisecond)
		wg.Done()
	}()

	_, _ = file.WriteString(newContent)
	time.Sleep(20 * time.Millisecond)
	_ = os.Remove(tmpMounts)
	wg.Wait()
	assert.Nil(t, content)
	assert.NotNil(t, err)

	// unable to get consistent content
	newFile, _ := os.Create(tmpMounts)
	_, _ = newFile.WriteString(fileContent)

	wg.Add(1)
	go func() {
		content, err = ConsistentRead(tmpMounts, 1, 100*time.Millisecond)
		wg.Done()
	}()

	time.Sleep(20 * time.Millisecond)
	_, _ = newFile.WriteString(newContent)
	wg.Wait()
	assert.Nil(t, content)
	assert.NotNil(t, err)
	fmt.Println(string(content))
}

var strToSC = []struct {
	strSC string
	check api.StorageClass
}{
	{"hdd", api.StorageClass_HDD},
	{"ssd", api.StorageClass_SSD},
	{"nvme", api.StorageClass_NVME},
	{"hddlvg", api.StorageClass_HDDLVG},
	{"ssdlvg", api.StorageClass_SSDLVG},
	{"any", api.StorageClass_ANY},
	{"random", api.StorageClass_ANY},
}

func TestConvertStorageClass(t *testing.T) {
	for _, test := range strToSC {
		got := ConvertStorageClass(test.strSC)
		if got != test.check {
			t.Errorf("Unexpected conversion between stringSC and api.StorageClass, expected %s, got %s",
				test.strSC, test.check.String())
		}
	}
}

var driveTypeToSC = []struct {
	driveType api.DriveType
	check     api.StorageClass
}{
	{api.DriveType_HDD, api.StorageClass_HDD},
	{api.DriveType_SSD, api.StorageClass_SSD},
	{api.DriveType_NVMe, api.StorageClass_NVME},
	{api.DriveType(5), api.StorageClass_ANY}, // random drive type
}

// Test byte value parsing from strings containing correct values
func TestConvertDriveTypeToStorageClass(t *testing.T) {
	for _, test := range driveTypeToSC {
		got := ConvertDriveTypeToStorageClass(test.driveType)
		if got != test.check {
			t.Errorf("Unexpected conversion between api.DriveType and api.StorageClass, expected %s, got %s",
				test.driveType.String(), test.check.String())
		}
	}
}
