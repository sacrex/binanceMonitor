#!/usr/bin/env bash

HTTPS_PROXY=http://192.168.112.1:7890
HTTP_PROXY=http://192.168.112.1:7890

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags '-extldflags "-static"' -o monitor.exe ./

mv monitor.exe release/
cp .env release/
./release/monitor.exe

# 服务查看
# nssm list