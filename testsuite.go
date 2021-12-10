package main

import (
	"fmt"
	"regexp"
)

func main() {
	drivePath := "/dev/sdb"

	device := regexp.MustCompile(`[A-Za-z0-9]+`).FindAllString(drivePath, -1)

	fmt.Printf("%+v", device)
}
