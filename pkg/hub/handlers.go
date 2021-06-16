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

package hub

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

type ClusterConnectHandler struct {
	ClusterTokenMapper ClusterTokenMapper

	callbacks []ClusterConnectHandlerCallback
	mutex     sync.Mutex
}

var _ http.Handler = &ClusterConnectHandler{}

func NewClusterConnectHandler(m ClusterTokenMapper) *ClusterConnectHandler {
	return &ClusterConnectHandler{
		ClusterTokenMapper: m,
	}
}

func (h *ClusterConnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	id, err := h.ClusterTokenMapper.MapClusterToken(ctx, token)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to map cluster token: %s", err), http.StatusInternalServerError)
		return
	}
	if id == "" {
		code := http.StatusUnauthorized
		http.Error(w, http.StatusText(code), code)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to upgrade WebSocket: %s", err), http.StatusInternalServerError)
		return
	}

	sess, err := yamux.Server(conn.UnderlyingConn(), nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create multiplex channel: %s", err), http.StatusInternalServerError)
		return
	}

	h.onNewClusterSession(id, sess)
}

func (h *ClusterConnectHandler) AddCallbacks(cbs ...ClusterConnectHandlerCallback) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.callbacks = append(h.callbacks, cbs...)
}

func (h *ClusterConnectHandler) onNewClusterSession(id string, sess *yamux.Session) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for _, cb := range h.callbacks {
		cb.OnNewClusterSession(id, sess)
	}
}

type ClusterTokenMapper interface {
	MapClusterToken(ctx context.Context, token string) (string, error)
}

type ClusterConnectHandlerCallback interface {
	OnNewClusterSession(id string, sess *yamux.Session)
}

type ClusterProxy struct {
	ClusterDialer ClusterDialer
}

func NewClusterProxy(d ClusterDialer) *ClusterProxy {
	return &ClusterProxy{
		ClusterDialer: d,
	}
}

func (h *ClusterProxy) Proxy(w http.ResponseWriter, r *http.Request, vars map[string]string) {
	id := vars["id"]

	target, err := url.Parse("http://127.0.0.1")
	if err != nil {
		panic(err)
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Transport = &http.Transport{
		Dial: func(network string, addr string) (net.Conn, error) {
			return h.ClusterDialer.DialCluster(id)
		},
	}
	rp.ServeHTTP(w, r)
}

type ClusterDialer interface {
	DialCluster(id string) (net.Conn, error)
}

type ClusterSessionManager struct {
	sessions map[string]*yamux.Session
	mutex    sync.Mutex
}

var _ ClusterConnectHandlerCallback = &ClusterSessionManager{}
var _ ClusterDialer = &ClusterSessionManager{}

func NewClusterSessionManager() *ClusterSessionManager {
	return &ClusterSessionManager{
		sessions: map[string]*yamux.Session{},
	}
}

func (m *ClusterSessionManager) OnNewClusterSession(id string, s *yamux.Session) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	sess := m.sessions[id]
	if sess != nil {
		sess.Close()
	}

	m.sessions[id] = s
}

func (m *ClusterSessionManager) DialCluster(id string) (net.Conn, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	sess := m.sessions[id]
	if sess == nil {
		return nil, fmt.Errorf("no session found for cluster %q", id)
	}
	return sess.Open()
}

//go:embed kopilot-agent.yaml
var AgentYAMLTemplate string

type AgentYAMLHandler struct {
	PublicAddr     string
	AgentImageName string
}

var _ http.Handler = &AgentYAMLHandler{}

func NewAgentYAMLHandler(publicAddr string, agentImageName string) *AgentYAMLHandler {
	return &AgentYAMLHandler{
		PublicAddr:     publicAddr,
		AgentImageName: agentImageName,
	}
}

func (h *AgentYAMLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	provider := r.URL.Query().Get("provider")

	k8sPKIDir := "/etc/kubernetes/pki"
	if strings.ToLower(strings.TrimSpace(provider)) == "minikube" {
		k8sPKIDir = "/var/lib/minikube/certs"
	}

	tmpl := template.Must(template.New("kopilot-agent.yaml").Parse(AgentYAMLTemplate))
	data := map[string]string{
		"imageName":  h.AgentImageName,
		"connectURL": fmt.Sprintf("wss://%s/connect?token=%s", h.PublicAddr, token),
		"k8sPKIDir":  k8sPKIDir,
	}
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, fmt.Sprintf("failed to execute template: %s", err), http.StatusInternalServerError)
		return
	}
}
