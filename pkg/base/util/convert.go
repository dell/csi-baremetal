/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sizeStrFmt = regexp.MustCompile(`(\d+(\.\d+)?)\s*(\S+)`)

// SizeUnit is the type for unit of information
type SizeUnit int64

const (
	// TBYTE represents 1 terabyte
	TBYTE SizeUnit = 1024 * 1024 * 1024 * 1024
	// GBYTE represents 1 gigabyte
	GBYTE SizeUnit = 1024 * 1024 * 1024
	// MBYTE represents 1 megabyte
	MBYTE SizeUnit = 1024 * 1024
	// KBYTE represents 1 kilobyte
	KBYTE SizeUnit = 1024
	// BYTE represents 1 byte
	BYTE SizeUnit = 1
)

// StrToBytes parses provided string and returns its value in bytes. Example: "15 Kb" -> 15360, "1GB" -> 1073741824
// Receives string value of information size with literal
// Returns provided size in bytes or error if something went wrong
func StrToBytes(str string) (int64, error) {
	var matches = sizeStrFmt.FindAllStringSubmatch(str, -1)
	if matches == nil {
		return 0, fmt.Errorf("unparseable size definition: %v", str)
	}
	value, _ := strconv.ParseFloat(matches[0][1], 64) // We don't expect error here, because number is validated by regex
	var mod int64
	switch strings.ToLower(matches[0][3]) {
	case "t", "tb", "ti", "tib", "e12":
		mod = int64(TBYTE)
	case "g", "gb", "gi", "gib", "e9":
		mod = int64(GBYTE)
	case "m", "mb", "mi", "mib", "e6":
		mod = int64(MBYTE)
	case "k", "kb", "ki", "kib", "e3":
		mod = int64(KBYTE)
	case "b":
		mod = int64(BYTE)
	default:
		return 0, fmt.Errorf("unknown size unit %v in supplied value %v", matches[0][2], str)
	}
	return int64(float64(mod) * value), nil
}

// ToSizeUnit converts value from specified size unit to another unit
// Receives size as value, 'from' as provided size unit and 'to' as size unit to convert
// Returns error if conversion leads to precision loss.
func ToSizeUnit(value int64, from SizeUnit, to SizeUnit) (int64, error) {
	var fromMod = int64(from)
	var toMod = int64(to)
	var byteValue = fromMod * value
	var res = byteValue / toMod
	if byteValue%toMod != 0 {
		// The error can be ignored, if precision loss is OK for you
		return res, fmt.Errorf("precision loss prohibited in conversion from value %d with unit size %d to unit with size %d", value, fromMod, toMod)
	}
	return res, nil
}

// ToBytes converts size of provided unit to its value in bytes
// Receives value as size and 'from' as size unit
// Returns provided size in bytes
func ToBytes(value int64, from SizeUnit) int64 {
	res, _ := ToSizeUnit(value, from, BYTE)
	return res
}
