#!/bin/sh

TARGZ=$2
TARGET=$1
TMPDIR=./tmp
TARGET_NAME=${TARGET//\//_}

curl -C - -o "$TARGET_NAME" $TARGZ
rm -rf $TMPDIR
mkdir $TMPDIR
tar -xvzf "$TARGET_NAME" -C $TMPDIR
cd $TMPDIR/rootfs
sudo tar -c . | sudo docker import - $TARGET
