package base

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"time"
)

// ConsistentRead returns content of the file and ensure that
// this content is actual (no one modify file during timeout)
// in case if there were not twice same read content - return error
func ConsistentRead(filename string, retry int, timeout time.Duration) ([]byte, error) {
	oldContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	for i := 0; i < retry; i++ {
		time.Sleep(timeout)
		newContent, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(oldContent, newContent) {
			return newContent, nil
		}
		// Files are different, continue reading
		oldContent = newContent
	}
	return nil, fmt.Errorf("could not get consistent content of %s after %d attempts", filename, retry)
}
