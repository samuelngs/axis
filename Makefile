
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

.PHONY: docker
docker:
	docker-compose up -d
