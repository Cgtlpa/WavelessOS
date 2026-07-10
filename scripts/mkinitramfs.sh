#!/bin/sh
set -e

WAV_SYSROOT="${WAV_SYSROOT:-/usr/local/waveless}"
OUTPUT="${1:-$WAV_SYSROOT/boot/initramfs.img}"

echo "=== building initramfs ==="

export TMPDIR="${TMPDIR:-/var/tmp}"
TMP="$(mktemp -d --tmpdir="$TMPDIR")"
trap "rm -rf $TMP" EXIT

mkdir -p "$TMP"/{bin,sbin,dev,etc,proc,sys,newroot,live}

if [ -f "$WAV_SYSROOT/bin/busybox" ]; then
	cp "$WAV_SYSROOT/bin/busybox" "$TMP/bin/"
elif [ -f "$WAV_SYSROOT/usr/bin/busybox" ]; then
	cp "$WAV_SYSROOT/usr/bin/busybox" "$TMP/bin/"
else
	echo "error: busybox not found in sysroot"
	exit 1
fi

for applet in sh mount umount cat ls echo test mkdir mknod modprobe sleep switch_root findfs; do
	ln -s busybox "$TMP/bin/$applet"
done

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

# check for install parameter
install_mode=""
for x in $cmdline; do
	if [ "$x" = "install" ]; then
		install_mode=1
	fi
done

# try root= from cmdline
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

# try to mount ISO (for live/install environment)
for _ in 1 2 3 4 5; do
	for cdrom in /dev/sr0 /dev/sr1 /dev/vdb /dev/sda /dev/cdrom; do
		if [ -b "$cdrom" ]; then
			/bin/busybox mount "$cdrom" /newroot 2>/dev/null && break 2
		fi
	done
	/bin/busybox sleep 1
done

if [ -z "$install_mode" ] && [ -f /newroot/boot/waveless.squashfs ]; then
	/bin/busybox mount /newroot/boot/waveless.squashfs /live
	/bin/busybox mount --move /proc /live/proc
	/bin/busybox mount --move /sys /live/sys
	/bin/busybox mount --move /dev /live/dev
	exec /bin/busybox switch_root /live /sbin/init
fi

if [ -z "$install_mode" ] && { [ -b /dev/disk/by-label/WAVELESS ] || [ -b /dev/disk/by-uuid/WAVELESS ]; }; then
	root_dev=""
	for dev in /dev/disk/by-label/WAVELESS /dev/disk/by-uuid/WAVELESS; do
		[ -b "$dev" ] && root_dev="$dev" && break
	done
	if [ -n "$root_dev" ]; then
		/bin/busybox mkdir -p /live
		/bin/busybox mount "$root_dev" /live 2>/dev/null
		if [ -f /live/boot/waveless.squashfs ]; then
			/bin/busybox mount /live/boot/waveless.squashfs /live 2>/dev/null || true
		fi
		if [ -x /live/sbin/init ]; then
			/bin/busybox mount --move /proc /live/proc
			/bin/busybox mount --move /sys /live/sys
			/bin/busybox mount --move /dev /live/dev
			exec /bin/busybox switch_root /live /sbin/init
		fi
	fi
fi

if [ -f /newroot/scripts/installer.sh ]; then
	if [ -f /newroot/boot/waveless.squashfs ]; then
		/bin/busybox mount /newroot/boot/waveless.squashfs /live
	fi
	/bin/busybox echo "WavelessOS live environment"
	if [ -n "$install_mode" ]; then
		export PATH=/bin:/sbin:/live/bin:/live/sbin:/live/usr/bin:/live/usr/sbin
		/bin/busybox echo "running installer..."
		/bin/busybox sh /newroot/scripts/installer.sh
		/bin/busybox echo "installer exited, dropping to shell"
	fi
	export PATH=/bin:/sbin:/newroot/scripts:/newroot/usr/bin:/live/bin:/live/sbin:/live/usr/bin:/live/usr/sbin
	/bin/busybox echo "run 'installer' to start installation or 'reboot' to reboot"
	echo "#!/bin/busybox sh" > /bin/installer
	echo "exec /bin/busybox sh /newroot/scripts/installer.sh" >> /bin/installer
	/bin/busybox chmod +x /bin/installer
	while true; do
		/bin/busybox sh
		/bin/busybox echo "shell exited, use 'reboot' to reboot"
		/bin/busybox sleep 1
	done
fi

/bin/busybox echo "initramfs: no root found, dropping to shell"
exec /bin/busybox sh
INITEOF

chmod +x "$TMP/init"

cd "$TMP"
find . | cpio -oH newc 2>/dev/null | gzip > "$OUTPUT"

echo "initramfs written to $OUTPUT ($(du -h "$OUTPUT" | cut -f1))"
