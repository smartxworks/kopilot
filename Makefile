.DEFAULT_GOAL := test
PROJECT := github.com/smartxworks/kopilot

generate:
	docker build -f hack/Dockerfile . | tee /dev/tty | tail -n1 | cut -d' ' -f3 | xargs -I{} \
		docker run --rm -v $$PWD:/go/src/$(PROJECT) -w /go/src/$(PROJECT) -e PROJECT=$(PROJECT) {} ./hack/generate.sh

fmt:
	go fmt ./...

test:
	go test -coverprofile=cover.out ./...

dev:
	skaffold dev

run:
	skaffold run

manifests:
	skaffold render --default-repo=smartxrocks --offline=true --digest-source=tag > manifests.yaml
