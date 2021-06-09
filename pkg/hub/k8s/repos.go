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

	"github.com/smartxworks/kopilot/pkg/hub"
	clientset "github.com/smartxworks/kopilot/pkg/hub/k8s/client/clientset/versioned"
)

//+kubebuilder:rbac:groups=kopilot.smartx.com,resources=clusters,verbs=get;list;watch;create;update;patch;delete

type ClusterRepository struct {
	client clientset.Interface
}

var _ hub.ClusterTokenMapper = &ClusterRepository{}

func NewClusterRepository(client clientset.Interface) *ClusterRepository {
	return &ClusterRepository{
		client: client,
	}
}

func (r *ClusterRepository) MapClusterToken(ctx context.Context, token string) (string, error) {
	clusterList, err := r.client.KopilotV1alpha1().Clusters("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list clusters: %s", err)
	}

	for _, cluster := range clusterList.Items {
		if cluster.Token == token {
			return fmt.Sprintf("%s_%s", cluster.Namespace, cluster.Name), nil
		}
	}
	return "", nil
}
