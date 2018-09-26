/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"flag"
	"log"

	"github.com/golang/glog"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/apis"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/cloud/google"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/cloud/google/machinesetup"
	"sigs.k8s.io/cluster-api-provider-gcp/pkg/controller"
	clusterapis "sigs.k8s.io/cluster-api/pkg/apis"
	clustercommon "sigs.k8s.io/cluster-api/pkg/apis/cluster/common"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {
	flag.Parse()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Initializing Dependencies.")
	initStaticDeps(mgr)

	log.Printf("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatal(err)
	}

	if err := clusterapis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatal(err)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting the Cmd.")

	// Start the Cmd
	log.Fatal(mgr.Start(signals.SetupSignalHandler()))
}

// Setup static dependencies.
// TODO: Do something better
func initStaticDeps(mgr manager.Manager) {
	machineConfigLocation := "/etc/machinesetup/machine_setup_configs.yaml"
	configWatch, err := machinesetup.NewConfigWatch(machineConfigLocation)
	if err != nil {
		glog.Fatalf("Could not create config watch: %v", err)
	}

	//
	google.MachineActuator, err = google.NewMachineActuator(google.MachineActuatorParams{
		MachineSetupConfigGetter: configWatch,
		EventRecorder:            mgr.GetRecorder("gce-controller"),
		Client:                   mgr.GetClient(),
		Scheme:                   mgr.GetScheme(),
	})
	if err != nil {
		glog.Fatalf("Error creating cluster provisioner for google : %v", err)
	}
	clustercommon.RegisterClusterProvisioner(google.ProviderName, google.MachineActuator)
}
