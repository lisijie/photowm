#!/bin/bash

go get github.com/jteeuwen/go-bindata/...

echo "generating fonts: ./fonts/fonts-data.go"

go-bindata -pkg=fonts -o ./fonts/fonts-data.go ./resource