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

package cluster

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"text/template"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	subresourceserver "github.com/smartxworks/kubernetes-subresource-server-runtime"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	kopilotv1alpha1 "github.com/smartxworks/kopilot/pkg/apis/kopilot/v1alpha1"
	clientset "github.com/smartxworks/kopilot/pkg/client/clientset/versioned"
	"github.com/smartxworks/kopilot/pkg/hub"
)

//+kubebuilder:rbac:groups=kopilot.smartx.com,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kopilot.smartx.com,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=configmaps,resourceNames=extension-apiserver-authentication,verbs=get;watch

var GroupVersionResource = schema.GroupVersionResource{
	Group:    fmt.Sprintf("subresource.%s", kopilotv1alpha1.SchemeGroupVersion.Group),
	Version:  kopilotv1alpha1.SchemeGroupVersion.Version,
	Resource: "clusters",
}

//go:embed kopilot-agent.yaml
var AgentYAMLTemplate string

func NewAgentSubresource(client clientset.Interface) *subresourceserver.Subresource {
	return &subresourceserver.Subresource{
		NamespaceScoped:      true,
		GroupVersionResource: GroupVersionResource,
		Name:                 "agent",
		ConnectMethods:       []string{http.MethodGet},
		Connect: func(ctx context.Context, key types.NamespacedName) (http.Handler, error) {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cluster, err := client.KopilotV1alpha1().Clusters(key.Namespace).Get(r.Context(), key.Name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
						return
					}
					http.Error(w, fmt.Sprintf("failed to get cluster: %s", err), http.StatusInternalServerError)
					return
				}

				token := r.URL.Query().Get("token")
				if token != cluster.Token {
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}

				connectSubresource := NewConnectSubresource(nil, nil)
				connectPath := connectSubresource.Path(key)
				tmpl := template.Must(template.New("kopilot-agent.yaml").Parse(AgentYAMLTemplate))
				data := map[string]string{
					"imageName":  hub.C.AgentImage,
					"connectURL": fmt.Sprintf("wss://%s%s?token=%s", hub.C.PublicAddr, connectPath, token),
				}
				if err := tmpl.Execute(w, data); err != nil {
					panic(err)
				}
			}), nil
		},
	}
}

func NewConnectSubresource(client clientset.Interface, sessionManager SessionManager) *subresourceserver.Subresource {
	return &subresourceserver.Subresource{
		NamespaceScoped:      true,
		GroupVersionResource: GroupVersionResource,
		Name:                 "connect",
		ConnectMethods:       []string{http.MethodGet},
		Connect: func(ctx context.Context, key types.NamespacedName) (http.Handler, error) {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cluster, err := client.KopilotV1alpha1().Clusters(key.Namespace).Get(r.Context(), key.Name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
						return
					}
					http.Error(w, fmt.Sprintf("failed to get cluster: %s", err), http.StatusInternalServerError)
					return
				}

				token := r.URL.Query().Get("token")
				if token != cluster.Token {
					http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
					return
				}

				upgrader := websocket.Upgrader{
					ReadBufferSize:  1024,
					WriteBufferSize: 1024,
				}
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					http.Error(w, fmt.Sprintf("failed to upgrade to WebSocket: %s", err), http.StatusInternalServerError)
					return
				}

				sess, err := yamux.Server(conn.UnderlyingConn(), nil)
				if err != nil {
					http.Error(w, fmt.Sprintf("failed to multiplex channel: %s", err), http.StatusInternalServerError)
					return
				}

				sessionManager.AddClusterSession(key, sess)
			}), nil
		},
	}
}

func NewProxySubresource(client clientset.Interface, sessionManager SessionManager, peerManager PeerManager) *subresourceserver.Subresource {
	return &subresourceserver.Subresource{
		NamespaceScoped:      true,
		GroupVersionResource: GroupVersionResource,
		Name:                 "proxy",
		ConnectMethods:       []string{http.MethodGet},
		Connect: func(ctx context.Context, key types.NamespacedName) (http.Handler, error) {
			return NewProxyHandler(client, sessionManager, peerManager, key, ""), nil
		},
		Route: func(ctx context.Context, key types.NamespacedName, path string) (http.Handler, error) {
			return NewProxyHandler(client, sessionManager, peerManager, key, path), nil
		},
	}
}

func NewProxyHandler(client clientset.Interface, sessionManager SessionManager, peerManager PeerManager, key types.NamespacedName, subpath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := client.KopilotV1alpha1().Clusters(key.Namespace).Get(r.Context(), key.Name, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf("failed to get cluster: %s", err), http.StatusInternalServerError)
			return
		}

		target, err := url.Parse("http://127.0.0.1")
		if err != nil {
			panic(err)
		}

		rp := httputil.NewSingleHostReverseProxy(target)
		origDirector := rp.Director
		rp.Director = func(r *http.Request) {
			origDirector(r)
			r.URL.Path = subpath
		}
		rp.Transport = &http.Transport{
			Dial: func(network string, addr string) (net.Conn, error) {
				return sessionManager.DialCluster(key)
			},
		}
		if peerManager != nil {
			rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
				peers, err := peerManager.ListPeers(r.Context())
				if err != nil {
					http.Error(w, fmt.Sprintf("failed to list peers: %s", err), http.StatusInternalServerError)
					return
				}

				idx := -1
				nextPeer := func() string {
					idx++
					if idx >= len(peers) {
						return ""
					}
					return peers[idx]
				}
				peerManager.TryNextPeer(w, r, e, key, nextPeer)
			}
		}
		rp.ServeHTTP(w, r)
	})
}

type PeerManager interface {
	ListPeers(ctx context.Context) ([]string, error)
	TryNextPeer(w http.ResponseWriter, r *http.Request, e error, key types.NamespacedName, nextPeer func() string)
}
