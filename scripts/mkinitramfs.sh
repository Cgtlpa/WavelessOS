#!/bin/sh
set -e

WAV_SYSROOT="${WAV_SYSROOT:-/usr/local/waveless}"
OUTPUT="${1:-$WAV_SYSROOT/boot/initramfs.img}"

echo "=== building initramfs ==="

TMP="$(mktemp -d)"
trap "rm -rf $TMP" EXIT

mkdir -p "$TMP"/{bin,sbin,dev,etc,proc,sys,newroot}

if [ -f "$WAV_SYSROOT/bin/busybox" ]; then
	cp "$WAV_SYSROOT/bin/busybox" "$TMP/bin/"
elif [ -f "$WAV_SYSROOT/usr/bin/busybox" ]; then
	cp "$WAV_SYSROOT/usr/bin/busybox" "$TMP/bin/"
else
	echo "error: busybox not found in sysroot"
	exit 1
fi

for applet in sh mount umount cat ls echo test mkdir mknod modprobe; do
	ln -s busybox "$TMP/bin/$applet"
done

mknod -m 622 "$TMP/dev/console" c 5 1
mknod -m 666 "$TMP/dev/null" c 1 3
mknod -m 666 "$TMP/dev/zero" c 1 5
mknod -m 444 "$TMP/dev/urandom" c 1 9

cat > "$TMP/etc/inittab" << 'EOF'
::sysinit:/etc/rc.init
::askfirst:-/bin/sh
EOF

cat > "$TMP/etc/rc.init" << 'EOF'
#!/bin/busybox sh

busybox mount -t proc proc /proc
busybox mount -t sysfs sysfs /sys
busybox mount -t devtmpfs devtmpfs /dev

cmdline=$(busybox cat /proc/cmdline)

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
	# resolve LABEL= and UUID=
	case "$root" in
		LABEL=*) root="$(busybox findfs "$root" 2>/dev/null)" ;;
		UUID=*) root="$(busybox findfs "$root" 2>/dev/null)" ;;
	esac
	if [ -n "$root" ] && [ -b "$root" ]; then
		busybox mkdir -p /newroot
		busybox mount "$root" /newroot 2>/dev/null
		if [ -x /newroot/sbin/init ]; then
			busybox mount --move /proc /newroot/proc
			busybox mount --move /sys /newroot/sys
			busybox mount --move /dev /newroot/dev
			busybox exec switch_root /newroot /sbin/init
		fi
		busybox umount /newroot 2>/dev/null
	fi
fi

# try to mount ISO (for live/install environment)
for cdrom in /dev/sr0 /dev/sr1 /dev/vdb; do
	if [ -b "$cdrom" ]; then
		busybox mount "$cdrom" /newroot 2>/dev/null && break
	fi
done

if [ -f /newroot/scripts/installer.sh ]; then
	busybox echo "WavelessOS live environment"
	if [ -n "$install_mode" ]; then
		busybox echo "running installer..."
		busybox exec /bin/sh /newroot/scripts/installer.sh
	fi
	export PATH=/bin:/sbin:/newroot/scripts:/newroot/usr/bin
	busybox echo "run 'installer' to start installation or type 'exit' to reboot"
	echo "#!/bin/busybox sh" > /bin/installer
	echo "exec /bin/sh /newroot/scripts/installer.sh" >> /bin/installer
	busybox chmod +x /bin/installer
	busybox exec /bin/sh
fi

busybox echo "initramfs: no root found, dropping to shell"
busybox exec /bin/sh
EOF

chmod +x "$TMP/etc/rc.init"

cd "$TMP"
find . | cpio -oH newc 2>/dev/null | gzip > "$OUTPUT"

echo "initramfs written to $OUTPUT ($(du -h "$OUTPUT" | cut -f1))"
