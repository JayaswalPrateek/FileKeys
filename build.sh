#!/bin/bash

echo "Checking updates for dependencies" &&
    go get -u && go mod tidy &&
    echo -e "DONE\n"

echo "Linting" &&
    go fmt ./main.go && go vet ./main.go && golangci-lint run ./main.go &&
    echo -e "DONE\n"

echo "Generating executable" &&
    go build -ldflags="-s -w" -o FileKeys_linux64 main.go &&
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -ldflags="-s -w" -o FileKeys64.exe main.go
    echo -e "DONE"
