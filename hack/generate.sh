#!/usr/bin/env bash

set -eux

bash $GOPATH/src/k8s.io/code-generator/generate-groups.sh deepcopy,client,informer,lister \
    $PROJECT/pkg/hub/k8s/client $PROJECT/pkg/hub/k8s/apis \
    kopilot:v1alpha1 \
    --go-header-file hack/boilerplate.go.txt

go generate ./...

controller-gen paths=./... crd rbac:roleName=kopilot-hub webhook
