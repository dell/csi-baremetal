#!/bin/bash

# Env
KIND_DIR=$1
KIND=$KIND_DIR/kind
KIND_VER=0.11.1

echo "First arg: $1"

# Get kind sources
wget -O $KIND_DIR/kind-src https://codeload.github.com/kubernetes-sigs/kind/tar.gz/refs/tags/v$KIND_VER
tar -xzvf $KIND_DIR/kind-src -C $KIND_DIR

# Add "--ipc=host" docker option and build binary
cd $KIND_DIR && patch -p0 < $KIND_DIR/kind-$KIND_VER.patch
cd $KIND_DIR/kind-$KIND_VER && make build

# Copy file
cp $KIND_DIR/kind-$KIND_VER/bin/kind $KIND

# Make executive
chmod +x $KIND

# Check
$KIND version

# Clean workdir
rm -rf $KIND_DIR/kind-$KIND_VER
rm -f $KIND_DIR/kind-src
