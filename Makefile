
ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

GOPACKAGES := $(shell go list ./... | grep -v /vendor/)

.PHONY: all
all: latest

.PHONY: latest
latest:
	CGO_ENABLED=0 go build -a -installsuffix cgo -o bin/axis

.PHONY: test
test:
	go vet ${GOPACKAGES}
	go test -race -test.v ${GOPACKAGES}

.PHONY: test-container
test-container:
	docker run -it --rm -v ${ROOT_DIR}/bin/axis:/axis -v ${ROOT_DIR}/axis.yaml:/axis.yaml alpine:latest /axis

.PHONY: docker
docker:
	docker-compose up -d
