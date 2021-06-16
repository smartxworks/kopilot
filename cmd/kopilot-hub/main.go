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
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/namsral/flag"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/smartxworks/kopilot/pkg/hub"
	"github.com/smartxworks/kopilot/pkg/hub/k8s"
	clientset "github.com/smartxworks/kopilot/pkg/hub/k8s/client/clientset/versioned"
)

func main() {
	bindAddr := "127.0.0.1:8080"
	flag.StringVar(&bindAddr, "bind", bindAddr, "bind address")
	publicAddr := "kopilot-hub.kopilot-system"
	flag.StringVar(&publicAddr, "public-addr", publicAddr, "public address of server")
	agentImageName := "kopilot-agent"
	flag.StringVar(&agentImageName, "agent-image", agentImageName, "kopilot-agent image")
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Fatalf("failed to build kubeconfig: %s", err)
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("failed to build client: %s", err)
	}

	clusterRepository := k8s.NewClusterRepository(client)
	go func() {
		if err := k8s.ServeWebhook(); err != nil {
			log.Fatalf("error serving webhook: %s", err)
		}
	}()

	clusterSessionManager := hub.NewClusterSessionManager()

	agentYAMLHandler := hub.NewAgentYAMLHandler(publicAddr, agentImageName)
	clusterConnectHandler := hub.NewClusterConnectHandler(clusterRepository)
	clusterConnectHandler.AddCallbacks(clusterSessionManager)
	clusterProxy := hub.NewClusterProxy(clusterSessionManager)

	r := mux.NewRouter()
	r.Handle("/kopilot-agent.yaml", agentYAMLHandler)
	r.Handle("/connect", clusterConnectHandler)
	r.PathPrefix("/proxy/{id}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		http.StripPrefix(fmt.Sprintf("/proxy/%s", vars["id"]),
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				clusterProxy.Proxy(w, r, vars)
			})).ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    bindAddr,
		Handler: r,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		signal.Notify(sigint, syscall.SIGTERM)
		<-sigint

		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("failed to shutdown server: %s", err)
		}
		close(idleConnsClosed)
	}()

	log.Println("starting server")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error running server: %s", err)
	}

	<-idleConnsClosed
}
