package main

import (
	"math/rand"
	"os"
	"time"

	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	"github.com/dell/csi-baremetal/pkg/scheduler/plugin"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	// Register plugin to the scheduler framework.
	command := app.NewSchedulerCommand(
		app.WithPlugin(plugin.Name, plugin.New),
	)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
