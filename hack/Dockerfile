FROM golang:1.16-alpine3.13

RUN apk add bash gcc musl-dev git

RUN tmp=$(mktemp -d) && \
    cd $tmp && \
    go mod init tmp && \
    go get github.com/golang/mock/mockgen@v1.6.0 && \
    go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.0 && \
    rm -rf $tmp

RUN git clone --branch=v0.21.1 --depth=1 https://github.com/kubernetes/code-generator.git $GOPATH/src/k8s.io/code-generator
