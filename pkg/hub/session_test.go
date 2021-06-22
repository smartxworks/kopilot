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

package hub_test

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/icrowley/fake"
	"github.com/smartxworks/kopilot/pkg/hub"
	"github.com/smartxworks/kopilot/pkg/hub/mock"
	assert "github.com/stretchr/testify/require"
)

//go:generate mockgen -source=session.go -destination=mock/session.go -package=mock
//go:generate mockgen -destination=mock/conn.go -package=mock net Conn

func TestClusterSessionManager(t *testing.T) {
	for _, tt := range []struct {
		sessionCount       int
		failedSessionCount int
	}{
		{0, 0},
		{0, 1},
		{1, 0},
		{1, 1},
		{2, 0},
		{2, 1},
		{2, 2},
	} {
		func() {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sessionManager := hub.NewClusterSessionManager()
			id := fake.Characters()

			for i := 0; i < tt.sessionCount; i++ {
				s := mock.NewMockClusterSession(ctrl)
				s.EXPECT().Open().Return(mock.NewMockConn(ctrl), nil).AnyTimes()
				sessionManager.OnNewClusterSession(id, s)
			}

			for i := 0; i < tt.failedSessionCount; i++ {
				s := mock.NewMockClusterSession(ctrl)
				s.EXPECT().Open().Return(nil, errors.New(fake.Characters())).AnyTimes()
				s.EXPECT().Close().Return(nil).AnyTimes()
				sessionManager.OnNewClusterSession(id, s)
			}

			for i := 0; i < 10; i++ {
				conn, err := sessionManager.DialCluster(id)
				if tt.sessionCount > 0 {
					assert.NoError(t, err)
					assert.NotNil(t, conn)
				} else {
					assert.Error(t, err)
				}
			}
		}()
	}
}
