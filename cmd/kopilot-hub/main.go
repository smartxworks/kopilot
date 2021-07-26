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
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/namsral/flag"
	subresourceserver "github.com/smartxworks/kubernetes-subresource-server-runtime"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/smartxworks/kopilot/pkg/client/clientset/versioned"
	"github.com/smartxworks/kopilot/pkg/hub"
	"github.com/smartxworks/kopilot/pkg/hub/cluster"
	"github.com/smartxworks/kopilot/pkg/hub/peer"
)

func main() {
	hub.InitFlags(flag.CommandLine)
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Fatalf("failed to build kubeconfig: %s", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("failed to create Kubernetes client: %s", err)
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("failed to create client: %s", err)
	}

	sessioManager := cluster.NewSessionManager()
	peerManager := peer.NewManager(kubeClient)

	s := subresourceserver.New(kubeClient)
	s.AddSubresource(cluster.NewAgentSubresource(client))
	s.AddSubresource(cluster.NewConnectSubresource(client, sessioManager))
	s.AddSubresource(cluster.NewProxySubresource(client, sessioManager, peerManager))

	ctx, cancel := context.WithCancel(context.Background())
	shutdownHandler := make(chan os.Signal, 2)
	signal.Notify(shutdownHandler, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-shutdownHandler
		cancel()
		<-shutdownHandler
		os.Exit(1)
	}()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := s.Start(ctx); err != nil {
			log.Fatalf("error running server: %s", err)
		}
		return nil
	})
	g.Go(func() error {
		if err := peer.StartServer(ctx, client, sessioManager); err != nil {
			log.Fatalf("error running peer server: %s", err)
		}
		return nil
	})
	g.Wait()
}
