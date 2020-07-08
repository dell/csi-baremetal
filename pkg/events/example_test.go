package events_test

import (
	"io/ioutil"
	"log"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/dell/csi-baremetal.git/api/v1/drivecrd"
	"github.com/dell/csi-baremetal.git/pkg/base/k8s"
	"github.com/dell/csi-baremetal.git/pkg/events"
)

func Example() {
	// We need event interface
	// this would work only inside of a k8s cluster
	k8SClientset, err := k8s.GetK8SClientset()
	if err != nil {
		log.Fatalf("fail to create kubernetes client, error: %s", err)
		return
	}
	eventInter := k8SClientset.CoreV1().Events("current_ns")

	// get the Scheme
	// in our case we should use Scheme that aware of CR
	// if your events are based on default objects you can use runtime.NewScheme()
	scheme, err := k8s.PrepareScheme()
	if err != nil {
		log.Fatalf("fail to prepare kubernetes scheme, error: %s", err)
		return
	}
	// Setup Option
	// It's used for label overriding and logging events

	var opt events.Options

	// Optional
	alertFile, err := ioutil.ReadFile("/etc/config/alerts.yaml")
	if err != nil {
		log.Fatalf("fail to open config file, error: %s", err)
	}

	err = yaml.Unmarshal(alertFile, &opt)
	if err != nil {
		log.Fatalf("fail to unmarshal config file, error: %s", err)
	}

	logr := logrus.New()
	opt.Logger = logr.WithField("component", "Events")
	//

	eventRecorder, err := events.New("baremetal-csi-node", "434aa7b1-8b8a-4ae8-92f9-1cc7e09a9030", eventInter, scheme, opt)
	if err != nil {
		log.Fatalf("fail to create events recorder, error: %s", err)
		return
	}
	// Wait till all events are sent/handled
	defer eventRecorder.Wait()

	// Send event
	drive := new(drivecrd.Drive)
	eventRecorder.Eventf(drive, "Critical", "DriveIsDead", "drive &s is dead", drive.GetName())
}
