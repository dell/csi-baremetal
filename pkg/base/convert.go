package base

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sizeStrFmt = regexp.MustCompile(`(\d+)\s*(\S+)`)

type SizeUnit uint64

const (
	TBYTE SizeUnit = 1024 * 1024 * 1024 * 1024
	GBYTE SizeUnit = 1024 * 1024 * 1024
	MBYTE SizeUnit = 1024 * 1024
	KBYTE SizeUnit = 1024
	BYTE  SizeUnit = 1
)

// Parses provided string and returns its value in bytes. Example: "15 Kb" -> 15360, "1GB" -> 1073741824
func StrToBytes(str string) (uint64, error) {
	var matches = sizeStrFmt.FindAllStringSubmatch(str, -1)
	if matches == nil {
		return 0, fmt.Errorf("unparseable size definition: %v", str)
	}
	value, _ := strconv.Atoi(matches[0][1]) //We don't expect error here, because number is validated by regex
	var mod uint64
	switch strings.ToLower(matches[0][2]) {
	case "t", "tb", "ti", "tib", "e12":
		mod = uint64(TBYTE)
	case "g", "gb", "gi", "gib", "e9":
		mod = uint64(GBYTE)
	case "m", "mb", "mi", "mib", "e6":
		mod = uint64(MBYTE)
	case "k", "kb", "ki", "kib", "e3":
		mod = uint64(KBYTE)
	case "b":
		mod = uint64(BYTE)
	default:
		return 0, fmt.Errorf("unknown size unit %v in supplied value %v", matches[0][2], str)
	}
	return mod * uint64(value), nil
}

// Convert value from specified size unit to another unit. Returns error if conversion leads to precision loss.
func ToSizeUnit(value uint64, from SizeUnit, to SizeUnit) (uint64, error) {
	var fromMod = uint64(from)
	var toMod = uint64(to)
	var byteValue = fromMod * value
	var res = byteValue / toMod
	if byteValue%toMod != 0 {
		//The error can be ignored, if precision loss is OK for you
		return res, fmt.Errorf("precision loss prohibited in conversion from value %d with unit size %d to unit with size %d", value, fromMod, toMod)
	}
	return res, nil
}

func ToBytes(value uint64, from SizeUnit) uint64 {
	res, _ := ToSizeUnit(value, from, BYTE)
	return res
}
