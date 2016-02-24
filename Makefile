
all: latest

latest:
	CGO_ENABLED=0 go build -a -installsuffix cgo
