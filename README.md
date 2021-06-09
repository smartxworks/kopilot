# Kopilot

[![build](https://github.com/smartxworks/kopilot/actions/workflows/build.yml/badge.svg)](https://github.com/smartxworks/kopilot/actions/workflows/build.yml)
[![codecov](https://codecov.io/gh/smartxworks/kopilot/branch/master/graph/badge.svg?token=YXRZFBEU83)](https://codecov.io/gh/smartxworks/kopilot)

_Kopilot_ is a network tunnel used to proxy Kubernetes API requests to member clusters. _Kopilot_ leverages WebSocket as the underlying connection, secured via HTTPS.

## How Does It Work

![architecture](docs/architecture.png)

The _kopilot-agent_ running in a member cluster will first initiate a WebSocket connection to the _kopilot-hub_ running in the host cluster. Then, the WebSocket connection would be multiplexed with [Yamux](https://github.com/hashicorp/yamux) and used as the proxy channel to the Kubernetes API of the member cluster.

## Getting Started

### Prerequisites

For the **host** cluster:

- _Kubernetes_ 1.16+ / _minikube_ / _kind_
- _cert-manager_ 1.0+

For **member** clusters:

- _Kubernetes_ 1.16+

### Installation

First, ensure that _cert-manager_ is installed on the **host** cluster. If it is not installed yet, you can install it as described in the _cert-manager_ [installation](https://cert-manager.io/docs/installation/kubernetes/) documentation. Alternatively, you can simply just run the single command below:

```shell
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

Once _cert-manager_ is running, you can now deploy the _kopilot-hub_ on the **host** cluster:

```shell
kubectl apply -f https://github.com/smartxworks/kopilot/releases/download/v0.1.0/kopilot.yaml
export EXTERNAL_PORT=$(kubectl get service/kopilot-hub -n kopilot-system -o jsonpath='{.spec.ports[0].nodePort}')
export EXTERNAL_IP=192.168.49.2  # change to your Kubernetes node IP
kubectl create configmap kopilot-hub -n kopilot-system --from-literal=external_addr=$EXTERNAL_IP:$EXTERNAL_PORT
```

## Usage

First, create a `Cluster` object in the **host** cluster to represent one **member** cluster that needs to be proxied:

```shell
kubectl apply -f https://raw.githubusercontent.com/smartxworks/kopilot/master/samples/cluster.yaml
```

Then, deploy the _kopilot-agent_ on the **member** cluster:

```shell
export TOKEN=$(kubectl get cluster/sample -o jsonpath='{.token}')
export MEMBER_KUBECONFIG=~/.kube/member_config  # change to your member kubeconfig path
curl -k https://$EXTERNAL_IP:$EXTERNAL_PORT/kopilot-agent.yaml?token=$TOKEN | kubectl apply --kubeconfig=$MEMBER_KUBECONFIG -f -
```

Once the _kopilot-agent_ is running, you can now send Kubernetes API requests to the **member** cluster from the **host** cluster with proper RBAC setups:

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kopilot-client
  namespace: kopilot-system
---
apiVersion: v1
kind: Pod
metadata:
  name: kopilot-client
  namespace: kopilot-system
spec:
  serviceAccountName: kopilot-client
  containers:
    - name: curl
      image: curlimages/curl
      command:
        - sleep
        - infinity
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kopilot-client
rules:
  - nonResourceURLs:
      - /proxy/*
    verbs:
      - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kopilot-client
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kopilot-client
subjects:
  - kind: ServiceAccount
    name: kopilot-client
    namespace: kopilot-system
EOF

kubectl exec -it pod/kopilot-client -n kopilot-system -- /bin/sh
curl -k -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" https://kopilot-hub.kopilot-system/proxy/default_sample/version
```

## License

This project is licensed under the Apache-2.0 License. See the [LICENSE](/LICENSE) file for more information.
