#!/bin/sh
set -e

WAV_SYSROOT="${WAV_SYSROOT:-/usr/local/waveless}"
WAV_CACHE="${WAV_CACHE:-$HOME/.wav/cache}"
WAV_TMP="${WAV_TMP:-/tmp/wav-build}"
WAV_JOBS="${WAV_JOBS:-$(nproc)}"

export WAV_SYSROOT WAV_CACHE WAV_TMP WAV_JOBS

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

missing=""
for cmd in go wget make gcc cpio gzip file unzip tar zstd; do
	command -v "$cmd" >/dev/null 2>&1 || missing="$missing $cmd"
done
if [ -n "$missing" ]; then
	echo "error: missing required build tools:$missing"
	exit 1
fi

echo "=== WavelessOS Precompile ==="
echo "sysroot: $WAV_SYSROOT"
echo "cache:   $WAV_CACHE"
echo "tmp:     $WAV_TMP"
echo "jobs:    $WAV_JOBS"
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

for pkg in linux-headers glibc zlib busybox bash doas linux; do
	echo ""
	echo ">>> acquiring $pkg..."
	doit wave acquire "$pkg"
done

echo ""
echo "building xfce packages..."

for pkg in \
	xfconf libxfce4util garcon libxfce4ui \
	xfce4-panel xfce4-session xfce4-settings xfwm4 xfdesktop \
	xfce4-terminal xfce4-appfinder xfce4-power-manager \
	xfce4-notifyd xfce4-screensaver xfce4-taskmanager \
	thunar; do
	echo ""
	echo ">>> acquiring $pkg..."
	doit wave acquire "$pkg"
done

echo ""
echo "building desktop utilities..."

for pkg in pavucontrol pfetch fastfetch vim nano; do
	echo ""
	echo ">>> acquiring $pkg..."
	doit wave acquire "$pkg"
done

echo ""
echo "building desktop applications..."

for pkg in firefox; do
	echo ""
	echo ">>> acquiring $pkg..."
	doit wave acquire "$pkg" || echo "warning: failed to acquire $pkg, skipping"
done

echo ""
echo "=== precompile complete ==="
echo "sysroot is at $WAV_SYSROOT"
echo "run '$SCRIPT_DIR/mkiso.sh' to build the ISO"
