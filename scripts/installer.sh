#!/bin/sh
set -e

if [ "$(id -u)" != 0 ]; then
	echo "run as root"
	exit 1
fi

if command -v dialog >/dev/null 2>&1; then
	DIALOG=dialog
elif command -v whiptail >/dev/null 2>&1; then
	DIALOG=whiptail
else
	DIALOG=
fi

die() {
	msgbox "$1"
	exit 1
}

msgbox() {
	local text="$1" h="${2:-6}" w="${3:-50}"
	if [ -n "$DIALOG" ]; then
		if [ "$DIALOG" = whiptail ]; then
			whiptail --msgbox "$text" "$h" "$w"
		else
			dialog --backtitle "WavelessOS Installer" --msgbox "$text" "$h" "$w"
		fi
	else
		echo "$text"
		echo "Press Enter to continue..."
		read dummy
	fi
}

infobox() {
	local text="$1" h="${2:-3}" w="${3:-40}"
	if [ -n "$DIALOG" ]; then
		if [ "$DIALOG" = whiptail ]; then
			echo "$text"
		else
			dialog --backtitle "WavelessOS Installer" --infobox "$text" "$h" "$w"
		fi
	else
		echo "$text"
	fi
}

yesno() {
	local text="$1" h="${2:-8}" w="${3:-50}" default="$4"
	if [ -n "$DIALOG" ]; then
		local def=""
		[ "$default" = no ] && def="--defaultno"
		if [ "$DIALOG" = whiptail ]; then
			whiptail $def --yesno "$text" "$h" "$w"
		else
			dialog --backtitle "WavelessOS Installer" $def --yesno "$text" "$h" "$w"
		fi
	else
		local prompt="${text} [y/N]: "
		[ "$default" != no ] && prompt="${text} [Y/n]: "
		echo -n "$prompt"
		read ans
		if [ "$default" = no ]; then
			[ "$ans" != y ] && [ "$ans" != Y ] && return 1
		else
			[ "$ans" = n ] || [ "$ans" = N ] && return 1
		fi
		return 0
	fi
}

menu() {
	local text="$1" h="$2" w="$3" mh="$4"
	shift 4
	if [ -n "$DIALOG" ]; then
		if [ "$DIALOG" = whiptail ]; then
			whiptail --menu "$text" "$h" "$w" "$mh" "$@" 2>/dev/tty
		else
			dialog --backtitle "WavelessOS Installer" --menu "$text" "$h" "$w" "$mh" "$@" 2>/dev/tty
		fi
	else
		echo "$text" >&2
		local i=1
		while [ $# -ge 2 ]; do
			echo "  $i) $2" >&2
			i=$((i + 1))
			shift 2
		done
		echo -n "select [1-$((i-1))]: " >&2
		read sel
		echo "$sel"
	fi
}

XFS_AVAIL=0; command -v mkfs.xfs >/dev/null 2>&1 && XFS_AVAIL=1
PARTED_AVAIL=0; command -v parted >/dev/null 2>&1 && PARTED_AVAIL=1
GRUB_AVAIL=0; command -v grub-install >/dev/null 2>&1 && GRUB_AVAIL=1
LSBLK_AVAIL=0; command -v lsblk >/dev/null 2>&1 && LSBLK_AVAIL=1

MKE2FS=""
for e in mkfs.ext4 mkfs.ext3 mkfs.ext2; do
	command -v "$e" >/dev/null 2>&1 && MKE2FS="$e" && break
done

MKDOS=""
for e in mkfs.fat mkfs.vfat mkdosfs; do
	command -v "$e" >/dev/null 2>&1 && MKDOS="$e" && break
done

HEIGHT=20
WIDTH=70

check_net() {
	ping -c 1 -W 3 1.1.1.1 >/dev/null 2>&1 || ping -c 1 -W 3 8.8.8.8 >/dev/null 2>&1
}

do_install() {
	DISK=$1

	infobox "partitioning $DISK..."
	if [ "$PARTED_AVAIL" = 1 ]; then
		parted -s "$DISK" mklabel gpt
		parted -s "$DISK" mkpart primary 1MiB 513MiB
		parted -s "$DISK" set 1 esp on
		parted -s "$DISK" mkpart primary 513MiB 100%
	else
		printf "g\nn\n1\n\n+512M\nt\n\n1\nn\n2\n\n\nw\n" | fdisk "$DISK" >/dev/null 2>&1 || die "fdisk failed"
	fi
	PART1="${DISK}1"
	PART2="${DISK}2"
	if [ ! -b "$PART2" ]; then
		PART2="${DISK}p2"
		PART1="${DISK}p1"
	fi

	infobox "formatting partitions..."
	if [ -n "$MKDOS" ]; then
		$MKDOS -F32 "$PART1" >/dev/null 2>&1 || die "failed to format $PART1"
	else
		die "no FAT formatter found (install dosfstools or enable mkfs.vfat in busybox)"
	fi
	if [ "$XFS_AVAIL" = 1 ]; then
		mkfs.xfs -f "$PART2" >/dev/null 2>&1 || die "failed to format $PART2"
	elif [ -n "$MKE2FS" ]; then
		$MKE2FS -F "$PART2" >/dev/null 2>&1 || die "failed to format $PART2"
	else
		die "no filesystem formatter found (install xfsprogs)"
	fi

	infobox "mounting partitions..."
	mount "$PART2" /mnt || die "failed to mount $PART2 on /mnt"
	mkdir -p /mnt/boot
	mount "$PART1" /mnt/boot || die "failed to mount $PART1 on /mnt/boot"

	ROOTFS="/usr/local/waveless"
	if [ ! -d "$ROOTFS" ]; then
		ROOTFS="/mnt"
		infobox "no sysroot found, installing minimal..."
		mkdir -p /mnt/{bin,sbin,etc,dev,proc,sys,tmp,var/log,run,mnt,usr/{bin,sbin,lib,share},lib/modules,boot}
		echo "waveless" > /mnt/etc/hostname 2>/dev/null
	else
		infobox "copying system files..."
		cp -a "$ROOTFS"/. /mnt/
	fi

	if [ -f /mnt/sbin/init ]; then
		:
	elif [ -f /mnt/bin/busybox ]; then
		ln -sf /bin/busybox /mnt/sbin/init 2>/dev/null
	fi

	mkdir -p /mnt/etc/init.d
	if [ ! -f /mnt/etc/inittab ]; then
		cat > /mnt/etc/inittab << EOF
::sysinit:/etc/rc.init
::respawn:-/usr/bin/bash
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

	if [ ! -f /mnt/etc/doas.conf ]; then
		echo "permit persist keepenv root" > /mnt/etc/doas.conf
		echo "permit persist keepenv :wheel" >> /mnt/etc/doas.conf
		mkdir -p /mnt/etc/group
	fi

	GENFSTAB="/mnt/etc/fstab"
	{
		echo "proc      /proc   proc   defaults    0 0"
		echo "sysfs     /sys    sysfs  defaults    0 0"
		echo "tmpfs     /run    tmpfs  mode=0755   0 0"
		echo "devtmpfs  /dev    devtmpfs defaults  0 0"
		PART2_UUID=$(blkid -s UUID -o value "$PART2" 2>/dev/null)
		PART2_FSTYPE=$(blkid -s TYPE -o value "$PART2" 2>/dev/null)
		if [ -n "$PART2_UUID" ]; then
			echo "UUID=$PART2_UUID / ${PART2_FSTYPE:-xfs} defaults,relatime 0 1"
		fi
		PART1_UUID=$(blkid -s UUID -o value "$PART1" 2>/dev/null)
		if [ -n "$PART1_UUID" ]; then
			echo "UUID=$PART1_UUID /boot vfat defaults 0 2"
		fi
	} > "$GENFSTAB"

	if [ "$GRUB_AVAIL" = 1 ]; then
		infobox "installing grub..."
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
			PART2_UUID=$(blkid -s UUID -o value "$PART2" 2>/dev/null || true)
			if [ -z "$PART2_UUID" ]; then
				die "failed to get UUID for $PART2"
			fi
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
	else
		infobox "grub not found, skipping bootloader install"
	fi

	mkdir -p /mnt/etc
	echo "waveless" > /mnt/etc/hostname

	infobox "set root password..."
	if [ -x /mnt/bin/busybox ]; then
		echo "root:waveless" | chroot /mnt /bin/busybox chpasswd 2>/dev/null || echo "warning: failed to set root password"
	elif [ -x /mnt/usr/bin/bash ]; then
		echo "root:waveless" | chroot /mnt /usr/bin/bash -c "chpasswd" 2>/dev/null || echo "warning: failed to set root password"
	fi

	for m in /mnt/proc /mnt/sys /mnt/dev /mnt/boot /mnt; do
		umount "$m" 2>/dev/null || true
	done

	msgbox "installation complete\n\nreboot and remove the install media" 7 50
}

while true; do
	if check_net; then
		break
	fi

	if command -v nmtui >/dev/null 2>&1; then
		net_sel=$(menu "no internet connection detected" 12 60 3 \
			1 "launch nmtui to configure wifi" \
			2 "skip and continue anyway" \
			3 "abort installer")
	else
		net_sel=$(menu "no internet connection detected" 10 60 2 \
			1 "skip and continue anyway" \
			2 "abort installer")
		if [ -n "$net_sel" ]; then
			net_sel=$((net_sel + 1))
		fi
	fi

	case $net_sel in
		1) nmtui ;;
		2) break ;;
		3) exit 0 ;;
		*) break ;;
	esac
done

DISKS=""
if [ "$LSBLK_AVAIL" = 1 ]; then
	for d in $(lsblk -ndo NAME,TYPE -e7 2>/dev/null | awk '$2 == "disk" {print $1}'); do
		dev="/dev/$d"
		size=$(lsblk -ndo SIZE "$dev" 2>/dev/null)
		DISKS="$DISKS $dev ${d}(${size})"
	done
else
	for d in $(fdisk -l 2>/dev/null | grep "^Disk /dev/" | grep -v loop | awk '{print $2}' | tr -d :); do
		dev="$d"
		dname=$(basename "$d")
		size=$(fdisk -l "$dev" 2>/dev/null | head -1 | awk '{print $3 $4}' | tr -d ,)
		DISKS="$DISKS $dev ${dname}(${size})"
	done
fi

if [ -z "$DISKS" ]; then
	msgbox "no disks found" 5 40
	exit 1
fi

DISK=$(menu "select target disk" $HEIGHT $WIDTH 5 $DISKS)

[ -z "$DISK" ] && exit 0
yesno "WARNING: all data on $DISK will be destroyed\n\ncontinue?" 8 50 no || exit 0

do_install "$DISK"

while true; do
	done_sel=$(menu "what now?" 10 50 2 \
		1 "reboot" \
		2 "back to shell")

	case $done_sel in
		1) reboot ;;
		2) exit 0 ;;
		*) exit 0 ;;
	esac
done
