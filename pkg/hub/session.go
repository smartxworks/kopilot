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
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
)

type ClusterSessionManager struct {
	sessionLists map[string][]ClusterSession
	mutex        sync.Mutex
}

type ClusterSession interface {
	Open() (net.Conn, error)
	Close() error
}

var _ ClusterConnectHandlerCallback = &ClusterSessionManager{}
var _ ClusterDialer = &ClusterSessionManager{}

func NewClusterSessionManager() *ClusterSessionManager {
	return &ClusterSessionManager{
		sessionLists: map[string][]ClusterSession{},
	}
}

func (m *ClusterSessionManager) OnNewClusterSession(id string, s ClusterSession) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	ss := m.sessionLists[id]
	if ss == nil {
		ss = []ClusterSession{}
	}
	ss = append(ss, s)
	m.sessionLists[id] = ss
}

func (m *ClusterSessionManager) DialCluster(id string) (net.Conn, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	ss := m.sessionLists[id]
	for {
		if len(ss) == 0 {
			return nil, fmt.Errorf("no session found for cluster %q", id)
		}

		idx := rand.Intn(len(ss))
		log.Printf("dialing cluster %q with session #%d", id, idx)
		conn, err := ss[idx].Open()
		if err != nil {
			log.Printf("removing session #%d of cluster %q due to dial error: %s", idx, id, err)
			ss[idx].Close()
			ss = append(ss[:idx], ss[idx+1:]...)
			m.sessionLists[id] = ss
			continue
		}

		return conn, nil
	}
}
