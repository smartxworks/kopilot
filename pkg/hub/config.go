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
	"github.com/namsral/flag"
)

type Config struct {
	AgentImage       string
	PublicAddr       string
	PeerBindAddr     string
	PeerCertDir      string
	ServiceNamespace string
	ServiceName      string
	IP               string
}

var C = Config{
	PublicAddr:       "kubernetes.default",
	PeerBindAddr:     ":6443",
	PeerCertDir:      "/tmp/k8s-subresource-server/cert",
	ServiceNamespace: "kopilot-system",
	ServiceName:      "kopilot-hub",
}

func InitFlags(flag *flag.FlagSet) {
	flag.StringVar(&C.AgentImage, "agent-image", C.AgentImage, "")
	flag.StringVar(&C.PublicAddr, "public-addr", C.PublicAddr, "public address of server")
	flag.StringVar(&C.PeerBindAddr, "peer-bind", C.PeerBindAddr, "peer server bind address")
	flag.StringVar(&C.PeerCertDir, "peer-cert-dir", C.PeerCertDir, "certificate directory of peer server")
	flag.StringVar(&C.ServiceNamespace, "service-namespace", C.ServiceNamespace, "namespace of kopilot-hub service")
	flag.StringVar(&C.ServiceName, "service-name", C.ServiceName, "name of kopilot-hub service")
	flag.StringVar(&C.IP, "ip", C.IP, "IP")
}
