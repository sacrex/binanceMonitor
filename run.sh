#!/usr/bin/env bash

HTTPS_PROXY=http://192.168.112.1:7890
HTTP_PROXY=http://192.168.112.1:7890

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags '-extldflags "-static"' -o monitor.exe ./

mkdir release/
mv monitor.exe release/
cp .env release/

nssm stop monitorCexFutures
nssm start monitorCexFutures

./release/monitor.exe

# 服务查看
# nssm list