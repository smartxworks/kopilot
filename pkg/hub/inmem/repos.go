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

package inmem

import (
	"context"

	"github.com/smartxworks/kopilot/pkg/hub"
)

type Cluster struct {
	ID    string
	Token string
}

type ClusterRepository struct {
	clusters []Cluster
}

var _ hub.ClusterTokenMapper = &ClusterRepository{}

func NewClusterRepository(clusters []Cluster) *ClusterRepository {
	return &ClusterRepository{
		clusters: clusters,
	}
}

func (r *ClusterRepository) MapClusterToken(ctx context.Context, token string) (string, error) {
	for _, cluster := range r.clusters {
		if cluster.Token == token {
			return cluster.ID, nil
		}
	}
	return "", nil
}
