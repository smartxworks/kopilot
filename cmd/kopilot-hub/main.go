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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/namsral/flag"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/smartxworks/kopilot/pkg/hub"
	"github.com/smartxworks/kopilot/pkg/hub/k8s"
	clientset "github.com/smartxworks/kopilot/pkg/hub/k8s/client/clientset/versioned"
)

var bindAddr = "127.0.0.1:8080"
var publicAddr = "kopilot-hub.kopilot-system"
var agentImageName = "kopilot-agent"
var ip = ""
var peerBindAddr = "0.0.0.0:6443"
var peerCertDir = "/tmp/kopilot-hub/peer-certs"

func main() {
	flag.StringVar(&bindAddr, "bind", bindAddr, "bind address")
	flag.StringVar(&publicAddr, "public-addr", publicAddr, "public address of server")
	flag.StringVar(&agentImageName, "agent-image", agentImageName, "kopilot-agent image")
	flag.StringVar(&ip, "ip", ip, "IP")
	flag.StringVar(&peerBindAddr, "peer-bind", peerBindAddr, "bind address of peer server")
	flag.StringVar(&peerCertDir, "peer-cert-dir", peerCertDir, "certificate directory of peer server")
	flag.Parse()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	signal.Notify(sigint, syscall.SIGTERM)

	clusterSessionManager := hub.NewClusterSessionManager()
	server := newServer(clusterSessionManager)
	peerServer := newPeerServer(clusterSessionManager)

	g := errgroup.Group{}
	g.Go(func() error {
		log.Println("starting server")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			return fmt.Errorf("error running server: %s", err)
		}
		return nil
	})
	g.Go(func() error {
		log.Println("starting peer server")
		if err := peerServer.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			return fmt.Errorf("error running peer server: %s", err)
		}
		return nil
	})

	<-sigint

	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("failed to shutdown server: %s", err)
	}
	if err := peerServer.Shutdown(context.Background()); err != nil {
		log.Printf("failed to shutdown peer server: %s", err)
	}

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

func newServer(clusterSessionManager *hub.ClusterSessionManager) *http.Server {
	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Fatalf("failed to build kubeconfig: %s", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("failed to build Kubernetes client: %s", err)
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

	agentYAMLHandler := hub.NewAgentYAMLHandler(publicAddr, agentImageName)
	clusterConnectHandler := hub.NewClusterConnectHandler(clusterRepository)
	clusterConnectHandler.AddCallbacks(clusterSessionManager)
	clusterProxyLB := hub.NewClusterProxy(clusterSessionManager)
	peersLister := k8s.NewPeersLister(kubeClient, "kopilot-hub")
	peersLister.ServiceNamespace = "kopilot-system"
	peersLister.SelfIP = ip
	clusterProxyLB.PeersLister = peersLister
	clusterProxyLB.TryNextPeer = tryNextPeer

	r := mux.NewRouter()
	r.Handle("/kopilot-agent.yaml", agentYAMLHandler)
	r.Handle("/connect", clusterConnectHandler)
	r.PathPrefix("/proxy/{id}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		http.StripPrefix(fmt.Sprintf("/proxy/%s", vars["id"]),
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				clusterProxyLB.Proxy(w, r, vars)
			})).ServeHTTP(w, r)
	})

	return &http.Server{
		Addr:    bindAddr,
		Handler: r,
	}
}

func newPeerServer(clusterSessionManager *hub.ClusterSessionManager) *http.Server {
	clusterProxy := hub.NewClusterProxy(clusterSessionManager)

	r := mux.NewRouter()
	r.PathPrefix("/proxy/{id}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		http.StripPrefix(fmt.Sprintf("/proxy/%s", vars["id"]),
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				clusterProxy.Proxy(w, r, vars)
			})).ServeHTTP(w, r)
	})

	cert, err := tls.LoadX509KeyPair(filepath.Join(peerCertDir, "tls.crt"), filepath.Join(peerCertDir, "tls.key"))
	if err != nil {
		log.Fatalf("failed to load peer cert: %s", err)
	}

	caCert, err := ioutil.ReadFile(filepath.Join(peerCertDir, "ca.crt"))
	if err != nil {
		log.Fatalf("failed to load peer CA: %s", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
	}

	return &http.Server{
		Addr:      peerBindAddr,
		Handler:   r,
		TLSConfig: tlsConfig,
	}
}

func tryNextPeer(w http.ResponseWriter, r *http.Request, e error, id string, nextPeer func() string) {
	peer := nextPeer()
	if peer == "" {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	target, err := url.Parse(fmt.Sprintf("https://%s/proxy/%s", peer, id))
	if err != nil {
		panic(err)
	}

	cert, err := tls.LoadX509KeyPair(filepath.Join(peerCertDir, "tls.crt"), filepath.Join(peerCertDir, "tls.key"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{cert},
		},
	}
	rp.ModifyResponse = func(r *http.Response) error {
		if r.StatusCode == http.StatusBadGateway {
			return errors.New("")
		}
		return nil
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		if e.Error() != "" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		tryNextPeer(w, r, e, id, nextPeer)
	}
	rp.ServeHTTP(w, r)
}
