
all: latest

latest:
	CGO_ENABLED=1 go build -race -a -installsuffix cgo

docker:
	docker-compose up -d
