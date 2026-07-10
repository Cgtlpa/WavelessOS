#!/bin/sh
set -e

WAV_SYSROOT="${WAV_SYSROOT:-/usr/local/waveless}"
WAV_CACHE="${WAV_CACHE:-$HOME/.wav/cache}"
WAV_TMP="${WAV_TMP:-/tmp/wav-build}"
WAV_JOBS="${WAV_JOBS:-$(nproc)}"

export WAV_SYSROOT WAV_CACHE WAV_TMP WAV_JOBS

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== WavelessOS Bootstrap ==="
echo "sysroot: $WAV_SYSROOT"
echo "cache:   $WAV_CACHE"
echo "tmp:     $WAV_TMP"
echo ""

mkdir -p "$WAV_SYSROOT"/{bin,sbin,etc,dev,proc,sys,tmp,run,var/log,mnt}
mkdir -p "$WAV_SYSROOT"/usr/{bin,sbin,lib,share,src}
mkdir -p "$WAV_SYSROOT"/lib/modules
mkdir -p "$WAV_CACHE" "$WAV_TMP"

echo "building doit..."
cd "$REPO_DIR/toolchain/doit"
go build -ldflags="-s -w" -o "$WAV_SYSROOT/usr/bin/doit" .
echo "doit installed to $WAV_SYSROOT/usr/bin/doit"
echo ""

export PATH="$WAV_SYSROOT/usr/bin:$PATH"

echo "installing base files..."
cp -a "$REPO_DIR/rootfs/". "$WAV_SYSROOT/"
echo ""

echo "building core packages..."
cd "$REPO_DIR"

for pkg in linux-headers glibc zlib busybox linux; do
	echo ""
	echo ">>> acquiring $pkg..."
	doit wave acquire "$pkg"
done

echo ""
echo "=== bootstrap complete ==="
echo "sysroot is at $WAV_SYSROOT"
echo "run 'doit wave acquire <package>' to install more packages"
echo "run the installer to deploy to disk"
