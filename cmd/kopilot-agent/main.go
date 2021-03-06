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

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/namsral/flag"
)

func main() {
	connectURL := ""
	flag.StringVar(&connectURL, "connect", connectURL, "connect URL of kopilot-hub")
	apiserverAddr := "kubernetes.default"
	flag.StringVar(&apiserverAddr, "apiserver", apiserverAddr, "kube-apiserver address")
	flag.Parse()

	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	wsConn, _, err := dialer.Dial(connectURL, nil)
	if err != nil {
		log.Fatalf("failed to dial hub %q: %s", connectURL, err)
	}

	sess, err := yamux.Client(wsConn.UnderlyingConn(), nil)
	if err != nil {
		log.Fatalf("failed to create multiplex channel: %s", err)
	}

	log.Println("connected to hub")

	apiserverURL, err := url.Parse(fmt.Sprintf("https://%s", apiserverAddr))
	if err != nil {
		log.Fatalf("failed to parse apiserver URL: %s", err)
	}

	apiserverProxy := httputil.NewSingleHostReverseProxy(apiserverURL)

	saDir := "/run/secrets/kubernetes.io/serviceaccount"
	token, err := os.ReadFile(filepath.Join(saDir, "token"))
	if err != nil {
		log.Fatalf("failed to read token: %s", err)
	}

	origDirector := apiserverProxy.Director
	apiserverProxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(token)))
	}

	caCert, err := ioutil.ReadFile(filepath.Join(saDir, "ca.crt"))
	if err != nil {
		log.Fatalf("failed to load CA cert: %s", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	apiserverProxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: caCertPool,
		},
	}

	server := &http.Server{
		Handler: apiserverProxy,
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

	log.Println("starting apiserver proxy")
	if err := server.Serve(sess); err != nil {
		log.Fatalf("error running apiserver proxy: %s", err)
	}

	<-idleConnsClosed
}
