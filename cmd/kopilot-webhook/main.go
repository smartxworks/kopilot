/*
Copyright 2021.

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
	"log"

	"github.com/namsral/flag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	kopilotv1alpha1 "github.com/smartxworks/kopilot/pkg/apis/kopilot/v1alpha1"
	"github.com/smartxworks/kopilot/pkg/webhook/cluster"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kopilotv1alpha1.AddToScheme(scheme))
}

func main() {
	var bindPort = 9443
	flag.IntVar(&bindPort, "bind-port", bindPort, "")
	flag.Parse()

	ctrl.SetLogger(klogr.New())
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Port:   bindPort,
	})
	if err != nil {
		log.Fatalf("failed to create controller manager: %s", err)
	}

	mgr.GetWebhookServer().Register("/mutate-v1alpha1-cluster", &webhook.Admission{Handler: cluster.NewMutator()})
	mgr.GetWebhookServer().Register("/validate-v1alpha1-cluster", &webhook.Admission{Handler: cluster.NewValidator()})

	log.Println("starting webhook")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Fatalf("err running webhook: %s", err)
	}
}
