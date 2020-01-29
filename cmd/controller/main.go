package main

import (
	"flag"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/controller"
	"github.com/container-storage-interface/spec/lib/go/csi"

	volumev1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = volumev1.AddToScheme(scheme)

	var port int
	var host string

	flag.IntVar(&port, "port", 50053, "Port for controller service")
	flag.StringVar(&host, "host", "", "Host for controller service")
	flag.Parse()

	ctrl.SetLogger(zap.Logger(true))
	cl, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}
	s := base.NewServerRunner(nil, host, int32(port))
	// register grpc services here

	server := controller.NewControllerService(cl)

	csi.RegisterIdentityServer(s.GRPCServer, controller.NewIdentityServer())
	csi.RegisterControllerServer(s.GRPCServer, server)
	if err := s.RunServer(); err != nil {
		setupLog.Error(err, "fail to serve")
		os.Exit(1)
	}
}
