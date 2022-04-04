package logger

import (
	"bytes"
	"encoding/json"
	"path"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
)

func foo() {
	logrus.Debug("Hello world from foo function!")
}

func bar() {
	log := logrus.WithFields(logrus.Fields{"test": "field"})
	log.Infoln("Hello world from bar function!")
	log.Infof("Hello world from bar function!")
}

type A struct{}

func (A) valueFunc() {
	logrus.Print("Hello world from valueFunc function!")
}

func (*A) pointerFunc() {
	logrus.Printf("Hello world from pointerFunc function!")
}

func (*A) ReflectedFunc(msg string) {
	logrus.Printf("Hello world from ReflectedFunc function: %s", msg)
}

func TestRuntimeFormatter(t *testing.T) {
	buffer := bytes.NewBuffer(nil)

	childFormatter := logrus.JSONFormatter{}
	formatter := &RuntimeFormatter{
		ChildFormatter: &childFormatter,
		MaxLevel:       logrus.DebugLevel,
	}
	logrus.SetFormatter(formatter)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(buffer)

	decoder := json.NewDecoder(buffer)
	currentDirectory := path.Join("pkg", "base", "logger")
	currentFile := "runtime_formatter_test.go"

	foo()

	expectFunction(t, decoder, currentDirectory+"/"+currentFile+":14", "foo")

	bar()

	expectFunction(t, decoder, currentDirectory+"/"+currentFile+":19", "bar")
	expectFunction(t, decoder, currentDirectory+"/"+currentFile+":20", "bar")

	a := A{}

	a.valueFunc()

	expectFunction(t, decoder, currentDirectory+"/"+currentFile+":26", "valueFunc")

	(&a).pointerFunc()

	expectFunction(t, decoder, currentDirectory+"/"+currentFile+":30", "pointerFunc")

	switch method := reflect.ValueOf(&a).MethodByName("ReflectedFunc").Interface().(type) {
	case func(string):
		method("hello world")
	}

	expectFunction(t, decoder, currentDirectory+"/"+currentFile+":34", "ReflectedFunc")
}

func TestFunctionInFunctionFormatter(t *testing.T) {
	buffer := bytes.NewBuffer(nil)

	childFormatter := logrus.JSONFormatter{}
	formatter := &RuntimeFormatter{
		ChildFormatter: &childFormatter,
		MaxLevel:       logrus.DebugLevel,
	}
	logrus.SetFormatter(formatter)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(buffer)

	decoder := json.NewDecoder(buffer)
	currentDirectory := path.Join("pkg", "base", "logger")
	currentFile := "runtime_formatter_test.go"

	funcInFunc()

	expectFunction(t, decoder, currentDirectory+"/"+currentFile+":121", "baz")

}

func expectFunction(t *testing.T, decoder *json.Decoder, expectedFile, expectedFunction string) {
	data := map[string]string{}
	err := decoder.Decode(&data)
	if err != nil {
		t.Fatal(err)
	}

	function := data[FunctionKey]
	file, _ := data[FileKey]

	if function != expectedFunction {
		t.Fatalf("Expected function: %s, got: %s", expectedFunction, function)
	}
	if len(file) > 0 && file != expectedFile {
		t.Fatalf("Expected file: %s, got: %s", expectedFile, file)
	}
}

func baz() {
	logrus.Debug("Hello world from baz function!")
}

func funcInFunc() {
	baz()
}
