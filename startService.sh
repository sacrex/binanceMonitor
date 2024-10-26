#!/usr/bin/env bash

HTTPS_PROXY=http://192.168.112.1:7890
HTTP_PROXY=http://192.168.112.1:7890

nssm stop monitorCexFutures

cp monitor.exe release/
cp .env release/

nssm start monitorCexFutures

# 服务查看
# nssm list