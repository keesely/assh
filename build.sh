#!/bin/bash
export GO11MODULE="on"

cd "$(dirname "$0")"
go mod tidy

PROJECT="assh"
VERSION="`go run . version 2>/dev/null || echo "v2.0.0-dev"`"
BUILD=`date +%FT%T%z`

function build () {
  os=$1
  arch=$2
  alias_name=$3
  package="${PROJECT}-${alias_name}-${arch}_${VERSION}"

  echo "build ${package}..."

  BuildPath="./build/${package}"
  mkdir -p $BuildPath
  CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build -o "${BuildPath}/assh" -ldflags "-X main.Version=${VERSION} -X main.Build=${BUILD}" ./

  if [ ${os} == "windows" ]; then
    cd ${BuildPath}
    mv assh assh.exe
    cd -
  fi

  cd ./build
  zip -r "${package}.zip" "./${package}"
  echo "Clean ${package}..."
  rm -rf "./${package}"
  cd ..
}

if [ -z "$1" ];then
  build darwin amd64 macOS
  build darwin 386 macOS
  build darwin arm macOS

  build linux amd64 linux
  build linux 386 linux
  build linux arm linux

  build windows amd64 windows
  build windows 386 windows
else
  build $@
fi
