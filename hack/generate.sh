#!/usr/bin/env bash

set -eux

bash $GOPATH/src/k8s.io/code-generator/generate-groups.sh deepcopy,client,informer,lister \
    $PROJECT/pkg/client $PROJECT/pkg/apis \
    kopilot:v1alpha1 \
    --go-header-file hack/boilerplate.go.txt

go generate ./...

controller-gen paths=./... crd webhook output:webhook:artifacts:config=config/kopilot-webhook
controller-gen paths=./pkg/hub/... rbac:roleName=kopilot-hub output:rbac:artifacts:config=config/kopilot-hub
