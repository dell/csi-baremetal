package base

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
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
	check string
}{
	{"hdd", api.StorageClassHDD},
	{"ssd", api.StorageClassSSD},
	{"nvme", api.StorageClassNVMe},
	{"hddlvg", api.StorageClassHDDLVG},
	{"ssdlvg", api.StorageClassSSDLVG},
	{"any", api.StorageClassAny},
	{"random", api.StorageClassAny},
}

func TestConvertStorageClass(t *testing.T) {
	for _, test := range strToSC {
		got := ConvertStorageClass(test.strSC)
		if got != test.check {
			t.Errorf("Unexpected conversion between stringSC and api.StorageClass, expected %s, got %s",
				test.strSC, test.check)
		}
	}
}

var driveTypeToSC = []struct {
	driveType string
	check     string
}{
	{api.DriveTypeHDD, api.StorageClassHDD},
	{api.DriveTypeSSD, api.StorageClassSSD},
	{api.DriveTypeNVMe, api.StorageClassNVMe},
	{"random", api.StorageClassAny}, // random drive type
}

// Test byte value parsing from strings containing correct values
func TestConvertDriveTypeToStorageClass(t *testing.T) {
	for _, test := range driveTypeToSC {
		got := ConvertDriveTypeToStorageClass(test.driveType)
		if got != test.check {
			t.Errorf("Unexpected conversion between api.DriveType and api.StorageClass, expected %s, got %s",
				test.driveType, test.check)
		}
	}
}

func TestContainsString(t *testing.T) {
	var containsStringScenarios = []struct {
		slice  []string
		str    string
		result bool
	}{
		{[]string{}, "", false},
		{[]string{}, "any", false},
		{[]string{"one"}, "two", false},
		{[]string{"one", "Two"}, "two", false},
		{[]string{"one", "two"}, "two", true},
	}

	var res bool
	for _, scenario := range containsStringScenarios {
		res = ContainsString(scenario.slice, scenario.str)
		assert.Equal(t, res, scenario.result)
	}
}

func TestRemoveString(t *testing.T) {
	var removeStringScenarios = []struct {
		slice  []string
		str    string
		result []string
	}{
		{[]string{}, "", []string(nil)},
		{[]string{}, "any", []string(nil)},
		{[]string{"one"}, "two", []string{"one"}},
		{[]string{"one", "Two"}, "two", []string{"one", "Two"}},
		{[]string{"one", "two"}, "two", []string{"one"}},
	}

	var res []string
	for _, scenario := range removeStringScenarios {
		res = RemoveString(scenario.slice, scenario.str)
		assert.Equal(t, res, scenario.result)
	}
}
