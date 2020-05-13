package util

import (
	"strings"
	"testing"
)

// Test byte value parsing from unparseable string. Error expected.
func TestStrToBytesIncorrect(t *testing.T) {
	got, err := StrToBytes("foo")
	if err == nil {
		t.Errorf("No error got when trying to parse unparseable value. Returned value: %d", got)
	}
	if !strings.Contains(err.Error(), "unparseable") {
		t.Errorf("Unexpected error text. Received error: %s", err.Error())
	}
}

// Test byte value parsing from string with unknown size unit. Error expected.
func TestStrToBytesWrongUnit(t *testing.T) {
	var unit = "Cm"
	got, err := StrToBytes("15" + unit)
	if err == nil {
		t.Errorf("No error got when trying to parse value with incorrect unit \"%s\". Returned value: %d", unit, got)
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Unexpected error text. Received error: %s", err.Error())
	}
}

var sizeTests = []struct {
	str   string
	bytes int64
}{
	{"15 b", 15},
	{"601B", 601},
	{"48e3", 48 * 1024},
	{"102 KB", 102 * 1024},
	{"9851 Mi", 9851 * 1024 * 1024},
	{"3gb", 3 * 1024 * 1024 * 1024},
	{"7t", 7 * 1024 * 1024 * 1024 * 1024},
	{"This disk has 5 gb of free space", 5 * 1024 * 1024 * 1024},
}

// Test byte value parsing from strings containing correct values
func TestStrToBytesCorrectBytes(t *testing.T) {
	for _, test := range sizeTests {
		got, err := StrToBytes(test.str)
		if err != nil {
			t.Errorf("Unexpected error got when trying to parse value \"%s\". Received error: %s", test.str, err.Error())
		}
		if got != test.bytes {
			t.Errorf("Unexpected conversion result: %d (bytes) when parsing value \"%s\". Expected: %d", got, test.str, test.bytes)
		}
	}
}

// Test value unit conversion for precision loss case. Error expected.
func TestToSizeUnitPrecisionLoss(t *testing.T) {
	got, err := ToSizeUnit(4095, KBYTE, MBYTE) // 4095KB doesn't represent integer value of megabytes
	if err == nil {
		t.Errorf("No error got when trying to convert value with precision loss. Returned value: %d", got)
	}
	if !strings.Contains(err.Error(), "precision loss") {
		t.Errorf("Unexpected error text. Received error: %s", err.Error())
	}
	if got != 3 {
		t.Errorf("Unexpected result for precision loss conversion: %d", got)
	}
}

var unitConversionTests = []struct {
	value  int64
	from   SizeUnit
	to     SizeUnit
	result int64
}{
	{61, BYTE, BYTE, 61},
	{48, KBYTE, BYTE, 48 * 1024},
	{45436, MBYTE, BYTE, 45436 * 1024 * 1024},
	{4, GBYTE, BYTE, 4 * 1024 * 1024 * 1024},
	{12, GBYTE, KBYTE, 12 * 1024 * 1024},
	{7, GBYTE, MBYTE, 7 * 1024},
	{98, MBYTE, KBYTE, 98 * 1024},
	{8 * 1024, MBYTE, GBYTE, 8},
	{3 * 1024 * 1024, BYTE, MBYTE, 3},
	{9 * 1024 * 1024, MBYTE, TBYTE, 9},
}

// Test size value conversion for correct params
func TestToSizeUnitCorrect(t *testing.T) {
	for _, test := range unitConversionTests {
		got, err := ToSizeUnit(test.value, test.from, test.to)
		if err != nil {
			t.Errorf("Unexpected error got when trying to convert value %d * (%d bytes) to "+
				"unit of size %d. Received error: %s", test.value, test.from, test.to, err.Error())
		}
		if got != test.result {
			t.Errorf("Unexpected conversion result: %d bytes, when parsing value %d * (%d bytes) "+
				"to unit of size %d. Expected: %d", got, test.value, test.from, test.to, test.result)
		}
	}
}

var byteConversionTests = []struct {
	value  int64
	from   SizeUnit
	result int64
}{
	{72, BYTE, 72},
	{48, KBYTE, 48 * 1024},
	{45436, MBYTE, 45436 * 1024 * 1024},
	{4, GBYTE, 4 * 1024 * 1024 * 1024},
	{6, TBYTE, 6 * 1024 * 1024 * 1024 * 1024},
}

// Test size value to bytes conversion for correct params
func TestToBytesCorrect(t *testing.T) {
	for _, test := range byteConversionTests {
		got := ToBytes(test.value, test.from)
		if got != test.result {
			t.Errorf("Unexpected conversion result: %d bytes, when parsing value %d * (%d bytes) "+
				"to bytes. Expected: %d", got, test.value, test.from, test.result)
		}
	}
}
