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
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"

	"github.com/hashicorp/yamux"
	"k8s.io/apimachinery/pkg/types"
)

type SessionManager interface {
	AddClusterSession(key types.NamespacedName, sess *yamux.Session)
	DialCluster(key types.NamespacedName) (net.Conn, error)
}

func NewSessionManager() SessionManager {
	return &sessionManager{
		sessionLists: map[string][]*yamux.Session{},
	}
}

type sessionManager struct {
	sessionLists map[string][]*yamux.Session
	mutex        sync.Mutex
}

func (m *sessionManager) AddClusterSession(key types.NamespacedName, s *yamux.Session) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	id := key.String()
	ss := m.sessionLists[id]
	if ss == nil {
		ss = []*yamux.Session{}
	}
	ss = append(ss, s)
	m.sessionLists[id] = ss
}

func (m *sessionManager) DialCluster(key types.NamespacedName) (net.Conn, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	id := key.String()
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
