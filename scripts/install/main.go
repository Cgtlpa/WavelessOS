package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

var in = bufio.NewReader(os.Stdin)

func run(cmd string, args ...string) {
	c := exec.C

func gray(ommand(cmd, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Run()
}

func clear() {
	fmt.Print("\033[2J\033[H")
}

func blue(s string) string {
	return "\033[38;5;75m" + s + "\033[0m"
}s string) string {
	return "\033[38;5;245m" + s + "\033[0m"
}

func cyan(s string) string {
	return "\033[38;5;117m" + s + "\033[0m"
}

func dim(s string) string {
	return "\033[38;5;240m" + s + "\033[0m"
}

func readKey() string {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return ""
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	buf := make([]byte, 3)
	n, _ := os.Stdin.Read(buf[:])
	if n == 0 {
		return ""
	}
	if buf[0] == 27 && n >= 3 && buf[1] == 91 {
		if buf[2] == 65 {
			return "up"
		}
		if buf[2] == 66 {
			return "down"
		}
	}
	if buf[0] == 10 || buf[0] == 13 {
		return "enter"
	}
	return ""
}

func pick(items []string, prompt string) string {
	sel := 0
	for {
		clear()
		fmt.Println()
		fmt.Println("  " + blue("WavelessOS Installer"))
		fmt.Println()
		fmt.Println("  " + cyan(prompt))
		fmt.Println()
		for i, item := range items {
			if i == sel {
				fmt.Println("  " + blue("> ") + item)
			} else {
				fmt.Println("  " + dim("  ") + gray(item))
			}
		}
		fmt.Println()
		fmt.Println(dim("  [up/down] navigate  [enter] select"))
		key := readKey()
		if key == "up" && sel > 0 {
			sel--
		}
		if key == "down" && sel < len(items)-1 {
			sel++
		}
		if key == "enter" {
			return items[sel]
		}
	}
}

func input(prompt string) string {
	fmt.Print("  " + gray(prompt) + ": ")
	text, _ := in.ReadString('\n')
	return strings.TrimSpace(text)
}

func getDisks() []string {
	out, _ := exec.Command("lsblk", "-dno", "NAME,SIZE,TYPE").Output()
	var disks []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "disk") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				disks = append(disks, "/dev/"+f[0]+"  "+f[1])
			}
		}
	}
	return disks
}

func main() {
	clear()
	fmt.Println()
	fmt.Println("  " + blue("WavelessOS Installer"))
	fmt.Println()

	disks := getDisks()
	if len(disks) == 0 {
		fmt.Println("  " + dim("No disks found."))
		return
	}
	diskLine := pick(disks, "Select disk to install to")
	disk := strings.Fields(diskLine)[0]

	hostname := input("Hostname [waveless]")
	if hostname == "" {
		hostname = "waveless"
	}

	timezone := input("Timezone [Europe/London]")
	if timezone == "" {
		timezone = "Europe/London"
	}

	locale := input("Locale [en_US.UTF-8]")
	if locale == "" {
		locale = "en_US.UTF-8"
	}

	password := input("Root password")

	clear()
	fmt.Println()
	fmt.Println("  " + blue("WavelessOS Installer"))
	fmt.Println()
	fmt.Println("  " + cyan("Disk:      ") + disk)
	fmt.Println("  " + cyan("Hostname:  ") + hostname)
	fmt.Println("  " + cyan("Timezone:  ") + timezone)
	fmt.Println("  " + cyan("Locale:    ") + locale)
	fmt.Println()
	fmt.Println("  " + dim("This will erase all data on "+disk))
	fmt.Println()

	confirm := pick([]string{"Yes, install", "No, go back"}, "Proceed with installation?")
	if confirm == "No, go back" {
		fmt.Println()
		fmt.Println("  " + dim("Aborted."))
		return
	}

	fmt.Println()
	fmt.Println("  " + gray(">> Partitioning..."))
	run("sgdisk", "-Z", disk)
	run("sgdisk", "-n", "1:0:+512M", "-t", "1:ef00", disk)
	run("sgdisk", "-n", "2:0:0", "-t", "2:8300", disk)
	run("partprobe", disk)

	fmt.Println("  " + gray(">> Formatting..."))
	run("mkfs.vfat", "-F", "32", disk+"p1")
	run("mkfs.ext4", "-F", disk+"p2")

	fmt.Println("  " + gray(">> Mounting..."))
	run("mount", disk+"p2", "/mnt")
	run("mkdir", "-p", "/mnt/boot/efi")
	run("mount", disk+"p1", "/mnt/boot/efi")

	fmt.Println("  " + gray(">> Installing base system..."))
	run("xbps-install", "-Sy", "-r", "/mnt", "base-system", "runit", "doas", "openssh", "dhcpcd")

	fmt.Println("  " + gray(">> Configuring..."))
	run("sh", "-c", "echo "+hostname+" > /mnt/etc/hostname")
	run("ln", "-sf", "/usr/share/zoneinfo/"+timezone, "/mnt/etc/localtime")
	run("sh", "-c", "echo LANG="+locale+" > /mnt/etc/locale.conf")
	run("arch-chroot", "/mnt", "/bin/sh", "-c", "echo 'root:"+password+"' | chpasswd")

	fmt.Println("  " + gray(">> Installing GRUB..."))
	run("arch-chroot", "/mnt", "grub-install", "--target=x86_64-efi", "--efi-directory=/boot/efi", "--bootloader-id=WavelessOS")
	run("arch-chroot", "/mnt", "grub-mkconfig", "-o", "/boot/grub/grub.cfg")

	fmt.Println("  " + gray(">> Enabling services..."))
	run("ln", "-sf", "/etc/sv/sshd", "/mnt/var/service/sshd")
	run("ln", "-sf", "/etc/sv/dhcpcd", "/mnt/var/service/dhcpcd")

	fmt.Println()
	fmt.Println("  " + blue("Installation complete!"))
	fmt.Println()

	reboot := pick([]string{"Yes, reboot", "No, stay here"}, "Reboot now?")
	if reboot == "Yes, reboot" {
		run("umount", "-R", "/mnt")
		run("reboot")
	}
}
