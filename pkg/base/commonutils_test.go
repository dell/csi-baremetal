package base

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"sync"
	"testing"
	"time"
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
