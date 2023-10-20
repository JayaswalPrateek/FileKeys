#!/bin/bash

echo "Checking updates for dependencies" &&
    go get -u && go mod tidy &&
    echo -e "DONE\n\n"

echo "Linting" &&
    go fmt ./main.go && go vet ./main.go && golangci-lint run ./main.go &&
    echo -e "DONE\n\n"

echo "Generating executable" &&
    go build -ldflags="-s -w" -o backendBinary_LINUX main.go &&
    GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o backendBinary_WINDOWS main.go &&
    echo -e "DONE"
