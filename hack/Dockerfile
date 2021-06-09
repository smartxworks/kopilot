FROM golang:1.16-alpine3.13

RUN apk add bash gcc musl-dev git

RUN tmp=$(mktemp -d) && \
    cd $tmp && \
    go mod init tmp && \
    go get github.com/golang/mock/mockgen@v1.5.0 && \
    go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.5.0 && \
    rm -rf $tmp

RUN git clone --branch=v0.20.7 --depth=1 https://github.com/kubernetes/code-generator.git $GOPATH/src/k8s.io/code-generator
