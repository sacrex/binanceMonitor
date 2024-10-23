#!/usr/bin/env bash

HTTPS_PROXY=http://192.168.112.1:7890
HTTP_PROXY=http://192.168.112.1:7890

go build -o monitor .

./monitor