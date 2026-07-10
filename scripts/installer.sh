#!/bin/sh

export DIALOGRC=/dev/null
export NCURSES_NO_UTF8_ACS=1

if [ "$(id -u)" != 0 ]; then
	echo "run as root"
	exit 1
fi

if ! command -v dialog >/dev/null 2>&1; then
	echo "install dialog first"
	exit 1
fi

if ! command -v mkfs.xfs >/dev/null 2>&1; then
	echo "install xfsprogs first"
	exit 1
fi

if ! command -v parted >/dev/null 2>&1; then
	echo "install parted first"
	exit 1
fi

if ! command -v mkfs.fat >/dev/null 2>&1; then
	echo "install dosfstools first"
	exit 1
fi

die() {
	dialog --backtitle "WavelessOS Installer" --msgbox "$1" 6 50
	exit 1
}

HEIGHT=20
WIDTH=70

check_net() {
	ping -c 1 -W 3 1.1.1.1 >/dev/null 2>&1 || ping -c 1 -W 3 8.8.8.8 >/dev/null 2>&1
}

do_install() {
	DISK=$1

	dialog --backtitle "WavelessOS Installer" --infobox "partitioning $DISK..." 3 40
	parted -s "$DISK" mklabel gpt
	parted -s "$DISK" mkpart primary 1MiB 513MiB
	parted -s "$DISK" set 1 esp on
	parted -s "$DISK" mkpart primary 513MiB 100%
	PART1="${DISK}1"
	PART2="${DISK}2"
	if [ ! -b "$PART2" ]; then
		PART2="${DISK}p2"
		PART1="${DISK}p1"
	fi

	dialog --backtitle "WavelessOS Installer" --infobox "formatting partitions..." 3 40
	mkfs.fat -F32 "$PART1" >/dev/null 2>&1 || die "failed to format $PART1"
	mkfs.xfs -f "$PART2" >/dev/null 2>&1 || die "failed to format $PART2"

	dialog --backtitle "WavelessOS Installer" --infobox "mounting partitions..." 3 40
	mount "$PART2" /mnt || die "failed to mount $PART2 on /mnt"
	mkdir -p /mnt/boot
	mount "$PART1" /mnt/boot || die "failed to mount $PART1 on /mnt/boot"

	ROOTFS="/usr/local/waveless"
	if [ ! -d "$ROOTFS" ]; then
		ROOTFS="/mnt"
		dialog --backtitle "WavelessOS Installer" --infobox "no sysroot found, installing minimal..." 3 50
		mkdir -p /mnt/{bin,sbin,etc,dev,proc,sys,tmp,var/log,run,mnt,usr/{bin,sbin,lib,share},lib/modules,boot}
		echo "waveless" > /mnt/etc/hostname 2>/dev/null

		DOIT_SRC="/toolchain/doit"
		if [ ! -d "$DOIT_SRC" ]; then
			DOIT_SRC="$(dirname "$0")/../toolchain/doit"
		fi
		if [ -f "$DOIT_SRC/main.go" ] && command -v go >/dev/null 2>&1; then
			dialog --backtitle "WavelessOS Installer" --infobox "building doit..." 3 40
			CGO_ENABLED=0 go build -ldflags="-s -w" -o /mnt/usr/bin/doit "$DOIT_SRC" 2>/dev/null
		fi
	else
		dialog --backtitle "WavelessOS Installer" --infobox "copying system files..." 3 40
		cp -a "$ROOTFS"/. /mnt/
	fi

	if [ -f /mnt/sbin/init ]; then
		:
	elif [ -f /mnt/bin/busybox ]; then
		ln -s /bin/busybox /mnt/sbin/init 2>/dev/null
	fi

	mkdir -p /mnt/etc/init.d
	if [ ! -f /mnt/etc/inittab ]; then
		cat > /mnt/etc/inittab << EOF
::sysinit:/etc/rc.init
::askfirst:-/bin/sh
::ctrlaltdel:/sbin/reboot
::shutdown:/etc/rc.shutdown
EOF
	fi

	if [ ! -f /mnt/etc/rc.init ]; then
		cat > /mnt/etc/rc.init << 'EOF'
#!/bin/sh
mount -a
mount -t tmpfs tmpfs /tmp
mkdir -p /tmp/.X11-unix /tmp/.ICE-unix
chmod 1777 /tmp
hostname -F /etc/hostname 2>/dev/null || hostname waveless

if [ -x /sbin/udevd ]; then
	/sbin/udevd --daemon
	/sbin/udevadm trigger
	/sbin/udevadm settle
fi

if [ -x /etc/rc.local ]; then
	/etc/rc.local
fi
EOF
		chmod +x /mnt/etc/rc.init
	fi

	if [ ! -f /mnt/etc/rc.shutdown ]; then
		cat > /mnt/etc/rc.shutdown << 'EOF'
#!/bin/sh
killall5 -15
sleep 1
killall5 -9 2>/dev/null
umount -a -r
EOF
		chmod +x /mnt/etc/rc.shutdown
	fi

	if [ ! -f /mnt/etc/os-release ]; then
		cat > /mnt/etc/os-release << EOF
NAME=WavelessOS
ID=waveless
PRETTY_NAME="WavelessOS"
HOME_URL=""
SUPPORT_URL=""
BUG_REPORT_URL=""
EOF
	fi

	GENFSTAB="/mnt/etc/fstab"
	{
		echo "proc      /proc   proc   defaults    0 0"
		echo "sysfs     /sys    sysfs  defaults    0 0"
		echo "tmpfs     /run    tmpfs  mode=0755   0 0"
		echo "devtmpfs  /dev    devtmpfs defaults  0 0"
		PART2_UUID=$(blkid -s UUID -o value "$PART2" 2>/dev/null)
		if [ -n "$PART2_UUID" ]; then
			echo "UUID=$PART2_UUID / xfs defaults,relatime 0 1"
		fi
		PART1_UUID=$(blkid -s UUID -o value "$PART1" 2>/dev/null)
		if [ -n "$PART1_UUID" ]; then
			echo "UUID=$PART1_UUID /boot vfat defaults 0 2"
		fi
	} > "$GENFSTAB"

	if command -v grub-install >/dev/null 2>&1; then
		dialog --backtitle "WavelessOS Installer" --infobox "installing grub..." 3 40
		if [ -d /sys/firmware/efi ]; then
			grub-install --target=x86_64-efi --efi-directory=/mnt/boot --bootloader-id=WavelessOS >/dev/null 2>&1
		else
			grub-install --target=i386-pc "$DISK" >/dev/null 2>&1
		fi

		KERNEL=""
		for k in /mnt/boot/vmlinuz-*; do
			if [ -f "$k" ]; then
				KERNEL=$(basename "$k")
				break
			fi
		done

		INITRD=""
		for i in /mnt/boot/initramfs-* /mnt/boot/initrd-* /mnt/boot/initramfs.img; do
			if [ -f "$i" ]; then
				INITRD=$(basename "$i")
				break
			fi
		done

		if [ -n "$KERNEL" ]; then
			mkdir -p /mnt/boot/grub
			cat > /mnt/boot/grub/grub.cfg << EOF
set timeout=3
set default=0

menuentry "WavelessOS" {
	linux /$KERNEL root=UUID=$PART2_UUID rw
EOF
			if [ -n "$INITRD" ]; then
				echo "	initrd /$INITRD" >> /mnt/boot/grub/grub.cfg
			fi
			echo "}" >> /mnt/boot/grub/grub.cfg
		fi
	fi

	mkdir -p /mnt/etc
	echo "waveless" > /mnt/etc/hostname

	dialog --backtitle "WavelessOS Installer" --infobox "set root password..." 3 40
	if [ -x /mnt/bin/busybox ]; then
		chroot /mnt /bin/busybox passwd 2>/dev/null
	elif [ -x /mnt/bin/sh ]; then
		chroot /mnt /bin/sh -c "passwd" 2>/dev/null
	fi

	umount -R /mnt 2>/dev/null

	dialog --backtitle "WavelessOS Installer" --msgbox "installation complete\n\nreboot and remove the install media" 7 50
}

while true; do
	if check_net; then
		break
	fi

	dialog --backtitle "WavelessOS Installer" --menu "no internet connection detected" 12 60 3 \
		1 "launch nmtui to configure wifi" \
		2 "skip and continue anyway" \
		3 "abort installer" 2>/tmp/wav_net

	case $(cat /tmp/wav_net) in
		1)
			if command -v nmtui >/dev/null 2>&1; then
				nmtui
			else
				dialog --backtitle "WavelessOS Installer" --msgbox "nmtui not found, configure wifi manually" 6 50
			fi
			;;
		2) break ;;
		3) exit 0 ;;
	esac
done

DISKS=""
for d in $(lsblk -ndo NAME,TYPE -e7 2>/dev/null | awk '$2 == "disk" {print $1}'); do
	dev="/dev/$d"
	size=$(lsblk -ndo SIZE "$dev" 2>/dev/null)
	DISKS="$DISKS $dev $d ($size)"
done

if [ -z "$DISKS" ]; then
	dialog --backtitle "WavelessOS Installer" --msgbox "no disks found" 5 40
	exit 1
fi

dialog --backtitle "WavelessOS Installer" --menu "select target disk" $HEIGHT $WIDTH 5 $DISKS 2>/tmp/wav_disk
DISK=$(cat /tmp/wav_disk)

dialog --backtitle "WavelessOS Installer" --defaultno --yesno "WARNING: all data on $DISK will be destroyed\n\ncontinue?" 8 50 || exit 0

do_install "$DISK"

while true; do
	dialog --backtitle "WavelessOS Installer" --menu "what now?" 10 50 2 \
		1 "reboot" \
		2 "back to shell" 2>/tmp/wav_done

	case $(cat /tmp/wav_done) in
		1) reboot ;;
		2) exit 0 ;;
	esac
done
