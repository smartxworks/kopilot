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

package peer

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
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	clientset "github.com/smartxworks/kopilot/pkg/client/clientset/versioned"
	"github.com/smartxworks/kopilot/pkg/hub"
	"github.com/smartxworks/kopilot/pkg/hub/cluster"
)

func StartServer(ctx context.Context, client clientset.Interface, sessionManager cluster.SessionManager) error {
	r := mux.NewRouter()
	r.PathPrefix("/proxy/{namespace}/{name}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := types.NamespacedName{
			Namespace: vars["namespace"],
			Name:      vars["name"],
		}
		subpath := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/proxy/%s/%s", vars["namespace"], vars["name"]))
		cluster.NewProxyHandler(client, sessionManager, nil, key, subpath).ServeHTTP(w, r)
	})

	cert, err := tls.LoadX509KeyPair(filepath.Join(hub.C.PeerCertDir, "tls.crt"), filepath.Join(hub.C.PeerCertDir, "tls.key"))
	if err != nil {
		log.Fatalf("failed to load peer cert: %s", err)
	}

	caCert, err := ioutil.ReadFile(filepath.Join(hub.C.PeerCertDir, "ca.crt"))
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

	server := &http.Server{
		Addr:      hub.C.PeerBindAddr,
		Handler:   r,
		TLSConfig: tlsConfig,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		<-ctx.Done()
		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("error shutting down the HTTP server: %s", err)
		}
		close(idleConnsClosed)
	}()

	if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return err
	}

	<-idleConnsClosed
	return nil
}

type Manager struct {
	kubeClient kubernetes.Interface
}

var _ cluster.PeerManager = &Manager{}

func NewManager(c kubernetes.Interface) *Manager {
	return &Manager{
		kubeClient: c,
	}
}

func (m *Manager) ListPeers(ctx context.Context) ([]string, error) {
	endpoints, err := m.kubeClient.CoreV1().Endpoints(hub.C.ServiceNamespace).Get(ctx, hub.C.ServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get endpoints: %s", err)
	}

	if len(endpoints.Subsets) == 0 {
		return nil, fmt.Errorf("no subset found for service %q", fmt.Sprintf("%s/%s", hub.C.ServiceNamespace, hub.C.ServiceName))
	}

	subset := endpoints.Subsets[0]
	if len(subset.Ports) == 0 {
		return nil, fmt.Errorf("no port found for service %q", fmt.Sprintf("%s/%s", hub.C.ServiceNamespace, hub.C.ServiceName))
	}

	port := subset.Ports[0].Port
	for _, p := range subset.Ports {
		if p.Name == "peer" {
			port = p.Port
			break
		}
	}

	var peers []string
	for _, addr := range subset.Addresses {
		if addr.IP == hub.C.IP {
			continue
		}
		peers = append(peers, fmt.Sprintf("%s:%d", addr.IP, port))
	}
	return peers, nil
}

func (m *Manager) TryNextPeer(w http.ResponseWriter, r *http.Request, e error, key types.NamespacedName, nextPeer func() string) {
	peer := nextPeer()
	if peer == "" {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	target, err := url.Parse(fmt.Sprintf("https://%s/proxy/%s/%s", peer, key.Namespace, key.Name))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse URL: %s", err), http.StatusInternalServerError)
		return
	}

	cert, err := tls.LoadX509KeyPair(filepath.Join(hub.C.PeerCertDir, "tls.crt"), filepath.Join(hub.C.PeerCertDir, "tls.key"))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load cert: %s", err), http.StatusInternalServerError)
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
		r.URL.Path = strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/proxy/%s/%s", key.Namespace, key.Name))
		m.TryNextPeer(w, r, e, key, nextPeer)
	}
	rp.ServeHTTP(w, r)
}
