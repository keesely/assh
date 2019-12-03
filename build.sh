#########################################################################
# File Name: build.sh
# Author: Kee
# mail: chinboy2012@gmail.com
# Created Time: 2019.11.08
#########################################################################
#!/bin/bash
export GO11MODULE="on"
go mod tidy

PROJECT="assh"
VERSION="v0.0.1"
BUILD=`date +%FT%T%z`

function build () {
  os=$1
  arch=$2
  alias_name=$3
  package="${PROJECT}-${alias_name}-${arch}_${VERSION}"

  echo "build ${package}..."

  BuildPath="./build/${package}"
  mkdir -p $BuildPath
  CGO_ENABLED=0 GOOS=${os} GOARH=${arch} go build -o "${BuildPath}/assh" -ldflags "-X main.Version=${VERSION} -X main.Build=${BUILD}" ./

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

# OS X Mac
build darwin amd64 macOS

# Linux
build linux amd64 linux
build linux 386 linux
build linux arm linux

# Windows
 build windows amd64 windows
 build windows 386 windows
else
  build $@
fi
