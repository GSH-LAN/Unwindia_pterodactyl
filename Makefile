PROJECT := $(shell git config --local remote.origin.url|sed -n 's#.*/\([^.]*\)\.git#\1#p'|sed 's/[A-Z]/\L&/g')

.PHONY: \
	build \
	test
run:
	go run ./cmd/$(PROJECT)/main.go

build:
	go build ./...

test:
	go test -v -count=1 -race ./...

docker:                                                                                                       .
	docker buildx build -t ghcr.io/gsh-lan/$(PROJECT):latest . --platform=linux/amd64

docker-arm:
	docker buildx build -t ghcr.io/gsh-lan/$(PROJECT):latest . --platform=linux/arm64

dockerx:
	docker buildx create --name $(PROJECT)-builder --use --bootstrap
	docker buildx build -t ghcr.io/gsh-lan/$(PROJECT):latest --platform=linux/arm64,linux/amd64 .
	docker buildx rm $(PROJECT)-builder

dockerx-builder:
	docker buildx create --name $(PROJECT)-builder --use --bootstrap

run-pulsar:
	docker run --rm --name=pulsar -p 8082:8080 -p 6650:6650 apachepulsar/pulsar:2.9.1 bin/pulsar standalone