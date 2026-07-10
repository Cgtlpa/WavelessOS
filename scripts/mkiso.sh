#!/bin/sh
set -e

WAV_SYSROOT="${WAV_SYSROOT:-/usr/local/waveless}"
REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT="${1:-$REPO_DIR/iso/waveless.iso}"

echo "=== building WavelessOS ISO ==="

export TMPDIR=/var/tmp
TMP="$(mktemp -d --tmpdir="$TMPDIR")"
trap "rm -rf $TMP" EXIT

ISODIR="$TMP/iso"
mkdir -p "$ISODIR/boot/grub"

KERNEL=""
for k in "$WAV_SYSROOT/boot/vmlinuz-"*; do
	if [ -f "$k" ]; then
		KERNEL="$k"
		break
	fi
done

if [ -z "$KERNEL" ]; then
	echo "error: no kernel found in $WAV_SYSROOT/boot/"
	echo "run: doit wave acquire linux"
	exit 1
fi

echo "kernel: $KERNEL"

"$REPO_DIR/scripts/mkinitramfs.sh" "$ISODIR/boot/initramfs.img"

cp "$KERNEL" "$ISODIR/boot/vmlinuz"

mkdir -p "$ISODIR/scripts" "$ISODIR/toolchain"
cp "$REPO_DIR/scripts/installer.sh" "$ISODIR/scripts/"
cp -a "$REPO_DIR/toolchain/doit" "$ISODIR/toolchain/doit"
if [ -f "$REPO_DIR/wave.conf" ]; then
	cp "$REPO_DIR/wave.conf" "$ISODIR/"
fi

if command -v mksquashfs >/dev/null 2>&1; then
	echo "creating live filesystem..."
	# generate pseudo file for rootfs overlay
	PSEUDO="$TMP/pseudo"
	> "$PSEUDO"
	cd "$REPO_DIR/rootfs"
	find . -type d ! -name . | while read d; do
		echo "\"${d#.}\" d 755 0 0" >> "$PSEUDO"
	done
	find . -type f | while read f; do
		mode="$(stat -c %a "$f")"
		echo "\"${f#.}\" f $mode 0 0 cat \"$REPO_DIR/rootfs${f#.}\"" >> "$PSEUDO"
	done
	find . -type l | while read l; do
		target="$(readlink "$l")"
		echo "\"${l#.}\" s 777 0 0 \"$target\"" >> "$PSEUDO"
	done
	cd /
	mksquashfs "$WAV_SYSROOT" "$ISODIR/boot/waveless.squashfs" \
		-pf "$PSEUDO" -comp xz -quiet
	LIVE="squashfs"
else
	echo "squashfs-tools not found, omitting live filesystem"
	echo "installer will build from source or use the ISO-mounted repo"
	LIVE="none"
fi

cat > "$ISODIR/boot/grub/grub.cfg" << 'GRUB'
set timeout=10
set default=0

menuentry "WavelessOS Live" {
	linux /boot/vmlinuz rw console=ttyS0 console=tty0 nomodeset
	initrd /boot/initramfs.img
}

menuentry "WavelessOS Install" {
	linux /boot/vmlinuz rw console=ttyS0 console=tty0 nomodeset install
	initrd /boot/initramfs.img
}

menuentry "Boot from first disk" {
	set root=(hd0)
	chainloader +1
}
GRUB

if command -v grub-mkrescue >/dev/null 2>&1; then
	echo "creating ISO..."
	grub-mkrescue -o "$OUTPUT" "$ISODIR" -- -volid WAVELESS 2>/dev/null
	echo "ISO written to $OUTPUT ($(du -h "$OUTPUT" | cut -f1))"
else
	echo "grub-mkrescue not found"
	echo "install xorriso and grub and try again"
	echo "ISO directory prepared at: $ISODIR"
	exit 1
fi
