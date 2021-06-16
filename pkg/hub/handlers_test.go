package hub

import (
	"bytes"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/icrowley/fake"
	assert "github.com/stretchr/testify/require"

	"github.com/smartxworks/kopilot/pkg/hub/mock"
)

//go:generate mockgen -source=handlers.go -destination=mock/mock.go -package=mock

func TestClusterConnectHandler(t *testing.T) {
	id := fake.Characters()
	token := fake.Characters()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tokenMapper := mock.NewMockClusterTokenMapper(ctrl)
	tokenMapper.EXPECT().MapClusterToken(gomock.Any(), token).Return(id, nil)
	handler := NewClusterConnectHandler(tokenMapper)

	callback := mock.NewMockClusterConnectHandlerCallback(ctrl)
	callback.EXPECT().OnNewClusterSession(id, gomock.Any())
	handler.AddCallbacks(callback)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	assert.NoError(t, err)

	connectURL := fmt.Sprintf("ws://%s?token=%s", u.Host, token)
	wsConn, _, err := websocket.DefaultDialer.Dial(connectURL, nil)
	assert.NoError(t, err)

	_, err = yamux.Client(wsConn.UnderlyingConn(), nil)
	assert.NoError(t, err)
}

func TestClusterConnectHandler_TokenInvalid(t *testing.T) {
	token := fake.Characters()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tokenMapper := mock.NewMockClusterTokenMapper(ctrl)
	tokenMapper.EXPECT().MapClusterToken(gomock.Any(), token).Return("", nil)
	handler := NewClusterConnectHandler(tokenMapper)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	assert.NoError(t, err)

	connectURL := fmt.Sprintf("ws://%s?token=%s", u.Host, token)
	_, resp, err := websocket.DefaultDialer.Dial(connectURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestClusterProxy(t *testing.T) {
	id := fake.Characters()
	expected := fake.Sentence()

	proxiedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, expected)
	}))
	defer proxiedServer.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dialer := mock.NewMockClusterDialer(ctrl)
	dialer.EXPECT().DialCluster(id).DoAndReturn(func(_ string) (net.Conn, error) {
		u, err := url.Parse(proxiedServer.URL)
		if err != nil {
			return nil, err
		}
		return net.Dial("tcp", u.Host)
	})
	proxy := NewClusterProxy(dialer)

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	proxy.Proxy(rr, req, map[string]string{"id": id})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, expected, rr.Body.String())
}

func TestAgentYAMLHandler(t *testing.T) {
	publicAddr := fake.DomainName()
	agentImageName := fake.Characters()
	token := fake.Characters()

	var expected bytes.Buffer
	tmpl := template.Must(template.New("kopilot-agent.yaml").Parse(AgentYAMLTemplate))
	data := map[string]string{
		"imageName":  agentImageName,
		"connectURL": fmt.Sprintf("wss://%s/connect?token=%s", publicAddr, token),
		"k8sPKIDir":  "/etc/kubernetes/pki",
	}
	err := tmpl.Execute(&expected, data)
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	assert.NoError(t, err)
	req.URL.RawQuery = fmt.Sprintf("token=%s", token)

	rr := httptest.NewRecorder()
	handler := NewAgentYAMLHandler(publicAddr, agentImageName)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, expected.String(), rr.Body.String())
}
