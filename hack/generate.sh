#!/usr/bin/env bash

set -eux

controller-gen paths=./... object:headerFile=hack/boilerplate.go.txt crd rbac:roleName=kopilot-hub webhook

go generate ./...

bash $GOPATH/src/k8s.io/code-generator/generate-groups.sh client,informer,lister \
    $PROJECT/pkg/hub/k8s/client $PROJECT/pkg/hub/k8s/apis \
    kopilot:v1alpha1 \
    --go-header-file hack/boilerplate.go.txt
