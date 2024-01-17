BINARY_NAME=arch_deployer
.DEFAULT_GOAL := build

build:
	GOOS=linux GOARCH=amd64 go build -o ../deployer/src/main/bin/installer src/main/main.go
	
update:
	go get -u ./...

dep:
	go mod download

lint:
	go env
	golangci-lint run --enable-all

clean:
	go clean