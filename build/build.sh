#!/bin/bash
mkdir -p gnp
mkdir -p pkg

os=linux
arch_list=(amd64 arm)
for arch in ${arch_list[*]}
do
  cp -f ../conf/config-example.yaml gnp/
  GOOS=$os GOARCH=$arch go build -o gnp/gnps  ../cmd/server/server.go
  GOOS=$os GOARCH=$arch go build -o gnp/gnpc  ../cmd/client/client.go
  tar -zcvf pkg/gnp-$os-$arch.tar.gz gnp
  rm -f gnp/gnp*
done

os=darwin
arch_list=(amd64 arm64)
for arch in ${arch_list[*]}
do
  cp -f ../conf/config-example.yaml gnp/
  GOOS=$os GOARCH=$arch go build -o gnp/gnps  ../cmd/server/server.go
  GOOS=$os GOARCH=$arch go build -o gnp/gnpc  ../cmd/client/client.go
  tar -zcvf pkg/gnp-$os-$arch.tar.gz gnp
  rm -f gnp/gnp*
done

os=windows
arch_list=(amd64)
for arch in ${arch_list[*]}
do
  cp -f ../conf/config-example.yaml gnp/
  GOOS=$os GOARCH=$arch go build -o gnp/gnps  ../cmd/server/server.go
  GOOS=$os GOARCH=$arch go build -o gnp/gnpc  ../cmd/client/client.go
  tar -zcvf pkg/gnp-$os-$arch.tar.gz gnp
  rm -f gnp/gnp*
done
