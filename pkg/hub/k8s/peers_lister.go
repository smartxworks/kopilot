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

package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/smartxworks/kopilot/pkg/hub"
)

//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

type PeersLister struct {
	KubeClient       kubernetes.Interface
	ServiceName      string
	ServiceNamespace string
	SelfIP           string
}

var _ hub.PeersLister = &PeersLister{}

func NewPeersLister(c kubernetes.Interface, serviceName string) *PeersLister {
	return &PeersLister{
		KubeClient:       c,
		ServiceNamespace: "default",
		ServiceName:      serviceName,
	}
}

func (l *PeersLister) ListPeers(ctx context.Context) ([]string, error) {
	endpoints, err := l.KubeClient.CoreV1().Endpoints(l.ServiceNamespace).Get(ctx, l.ServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get endpoints: %s", err)
	}

	if len(endpoints.Subsets) == 0 {
		return nil, fmt.Errorf("no subset found for service %q", fmt.Sprintf("%s/%s", l.ServiceNamespace, l.ServiceName))
	}

	subset := endpoints.Subsets[0]
	if len(subset.Ports) == 0 {
		return nil, fmt.Errorf("no port found for service %q", fmt.Sprintf("%s/%s", l.ServiceNamespace, l.ServiceName))
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
		if addr.IP == l.SelfIP {
			continue
		}
		peers = append(peers, fmt.Sprintf("%s:%d", addr.IP, port))
	}
	return peers, nil
}
