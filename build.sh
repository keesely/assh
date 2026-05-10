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

  # 构建 assh 主二进制（SSH 客户端）
  echo "  -> assh (SSH client)"
  CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build \
    -o "${BuildPath}/assh" \
    -ldflags "-X main.Version=${VERSION} -X main.Build=${BUILD}" ./

  # 构建 assh-fs 文件系统工具二进制（SFTP + 将来挂载）
  echo "  -> assh-fs (file system tool)"
  CGO_ENABLED=0 GOOS=${os} GOARCH=${arch} go build \
    -o "${BuildPath}/assh-fs" \
    -ldflags "-X main.Version=${VERSION} -X main.Build=${BUILD}" ./cmd/fs/

  if [ ${os} == "windows" ]; then
    cd ${BuildPath}
    mv assh assh.exe
    mv assh-fs assh-fs.exe
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
