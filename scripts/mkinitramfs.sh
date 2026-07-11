#!/bin/sh
set -e

WAV_SYSROOT="${WAV_SYSROOT:-/usr/local/waveless}"
OUTPUT="${1:-$WAV_SYSROOT/boot/initramfs.img}"

echo "=== building initramfs ==="

export TMPDIR="${TMPDIR:-/var/tmp}"
TMP="$(mktemp -d --tmpdir="$TMPDIR")"
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP"/{bin,sbin,usr/bin,dev,etc,proc,sys,newroot,live}

if [ -f "$WAV_SYSROOT/bin/busybox" ]; then
	cp "$WAV_SYSROOT/bin/busybox" "$TMP/bin/"
elif [ -f "$WAV_SYSROOT/usr/bin/busybox" ]; then
	cp "$WAV_SYSROOT/usr/bin/busybox" "$TMP/bin/"
else
	echo "error: busybox not found in sysroot"
	exit 1
fi

for applet in sh ash mount umount cat ls echo test mkdir mknod modprobe sleep switch_root findfs setsid cttyhack reboot poweroff halt hostname id stty; do
	ln -sf busybox "$TMP/bin/$applet"
done

for applet in switch_root reboot poweroff halt; do
	ln -sf busybox "$TMP/sbin/$applet"
done

if [ -f "$WAV_SYSROOT/usr/bin/bash" ]; then
	cp "$WAV_SYSROOT/usr/bin/bash" "$TMP/usr/bin/bash"
	ln -sf bash "$TMP/usr/bin/sh"
fi

if [ -f "$WAV_SYSROOT/usr/bin/dialog" ]; then
	cp "$WAV_SYSROOT/usr/bin/dialog" "$TMP/usr/bin/dialog"
fi

if [ -f "$WAV_SYSROOT/usr/bin/doas" ]; then
	cp "$WAV_SYSROOT/usr/bin/doas" "$TMP/usr/bin/doas"
fi

if [ -f "$WAV_SYSROOT/usr/bin/wget" ]; then
	cp "$WAV_SYSROOT/usr/bin/wget" "$TMP/usr/bin/wget" 2>/dev/null || true
fi

if [ -f "$WAV_SYSROOT/usr/bin/cp" ]; then
	cp "$WAV_SYSROOT/usr/bin/cp" "$TMP/usr/bin/cp" 2>/dev/null || true
fi

mknod -m 622 "$TMP/dev/console" c 5 1
mknod -m 666 "$TMP/dev/null" c 1 3
mknod -m 666 "$TMP/dev/zero" c 1 5
mknod -m 444 "$TMP/dev/urandom" c 1 9

cat > "$TMP/init" << 'INITEOF'
#!/bin/busybox sh

/bin/busybox mount -t proc proc /proc
/bin/busybox mount -t sysfs sysfs /sys
/bin/busybox mount -t devtmpfs devtmpfs /dev

cmdline=$(/bin/busybox cat /proc/cmdline)

install_mode=""
for x in $cmdline; do
	if [ "$x" = "install" ]; then
		install_mode=1
	fi
done

root=""
for x in $cmdline; do
	case "$x" in
		root=*) root="${x#root=}" ;;
	esac
done

if [ -n "$root" ]; then
	case "$root" in
		LABEL=*) root="$(/bin/busybox findfs "$root" 2>/dev/null)" ;;
		UUID=*) root="$(/bin/busybox findfs "$root" 2>/dev/null)" ;;
	esac
	if [ -n "$root" ] && [ -b "$root" ]; then
		/bin/busybox mkdir -p /newroot
		/bin/busybox mount "$root" /newroot 2>/dev/null
		if [ -x /newroot/sbin/init ]; then
			/bin/busybox mount --move /proc /newroot/proc
			/bin/busybox mount --move /sys /newroot/sys
			/bin/busybox mount --move /dev /newroot/dev
			exec /bin/busybox switch_root /newroot /sbin/init
		fi
		/bin/busybox umount /newroot 2>/dev/null
	fi
fi

for _ in 1 2 3 4 5; do
	for dev in /dev/sr[0-9] /dev/cdrom; do
		if [ -b "$dev" ]; then
			/bin/busybox mount "$dev" /newroot 2>/dev/null && break 2
		fi
	done
	for dev in $(/bin/busybox ls /sys/block/ 2>/dev/null); do
		case "$dev" in
			loop*|ram*) continue ;;
		esac
		if [ -b "/dev/$dev" ]; then
			/bin/busybox mount "/dev/$dev" /newroot 2>/dev/null && break 2
		fi
	done
	/bin/busybox sleep 1
done

if [ -z "$install_mode" ] && [ -f /newroot/boot/waveless.squashfs ]; then
	/bin/busybox mkdir -p /squashfs /overlay/upper /overlay/work
	/bin/busybox mount -t squashfs /newroot/boot/waveless.squashfs /squashfs 2>/dev/null
	/bin/busybox mount -t tmpfs tmpfs /overlay 2>/dev/null
	/bin/busybox mount -t overlay overlay \
		-o lowerdir=/squashfs,upperdir=/overlay/upper,workdir=/overlay/work \
		/newroot 2>/dev/null
	if [ -x /newroot/sbin/init ]; then
		/bin/busybox mkdir -p /newroot/proc /newroot/sys /newroot/dev
		/bin/busybox mount --move /proc /newroot/proc 2>/dev/null
		/bin/busybox mount --move /sys /newroot/sys 2>/dev/null
		/bin/busybox mount --move /dev /newroot/dev 2>/dev/null
		exec /bin/busybox switch_root /newroot /sbin/init
	fi
fi

if [ -z "$install_mode" ] && [ -b /dev/disk/by-label/WAVELESS ]; then
	root_dev="/dev/disk/by-label/WAVELESS"
	/bin/busybox mkdir -p /squashfs /overlay/upper /overlay/work
	/bin/busybox mount "$root_dev" /squashfs 2>/dev/null
	if [ -f /squashfs/boot/waveless.squashfs ]; then
		/bin/busybox mount -t squashfs /squashfs/boot/waveless.squashfs /live 2>/dev/null
		/bin/busybox mount -t tmpfs tmpfs /overlay 2>/dev/null
		/bin/busybox mount -t overlay overlay \
			-o lowerdir=/live,upperdir=/overlay/upper,workdir=/overlay/work \
			/squashfs 2>/dev/null
	fi
	if [ -x /squashfs/sbin/init ]; then
		/bin/busybox mkdir -p /squashfs/proc /squashfs/sys /squashfs/dev
		/bin/busybox mount --move /proc /squashfs/proc 2>/dev/null
		/bin/busybox mount --move /sys /squashfs/sys 2>/dev/null
		/bin/busybox mount --move /dev /squashfs/dev 2>/dev/null
		exec /bin/busybox switch_root /squashfs /sbin/init
	fi
fi

if [ -f /newroot/scripts/installer.sh ]; then
	if [ -f /newroot/boot/waveless.squashfs ]; then
		/bin/busybox mount /newroot/boot/waveless.squashfs /live
	fi

	export PATH="/usr/bin:/bin:/sbin:/newroot/usr/bin:/newroot/usr/sbin:/newroot/scripts:/live/usr/bin:/live/usr/sbin:/live/bin:/live/sbin"

	if [ -n "$install_mode" ]; then
		/bin/busybox echo "running installer..."
		exec /bin/busybox sh /newroot/scripts/installer.sh
	fi

	/bin/busybox echo "WavelessOS live environment"
	/bin/busybox echo "type 'installer' to install or 'reboot' to reboot"

	while true; do
		setsid cttyhack /usr/bin/bash -c 'export PATH="/usr/bin:/bin:/sbin:/newroot/usr/bin:/newroot/usr/sbin:/newroot/scripts:/live/usr/bin:/live/usr/sbin:/live/bin:/live/sbin"; exec /usr/bin/bash' 2>/dev/null || setsid cttyhack sh
		/bin/busybox echo "shell exited, use 'reboot' to reboot"
	done
fi

/bin/busybox echo "initramfs: no root found, dropping to shell"
exec setsid cttyhack sh
INITEOF

chmod +x "$TMP/init"

cd "$TMP"
(find . | cpio -oH newc) | gzip > "$OUTPUT"
if [ ! -s "$OUTPUT" ]; then
	echo "error: initramfs is empty"
	exit 1
fi

echo "initramfs written to $OUTPUT ($(du -h "$OUTPUT" | cut -f1))"
