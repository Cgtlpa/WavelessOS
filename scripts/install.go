package main

import (
	"fmt"
	"os/exec"
)

func main() {
	fmt.Println("Welcome to the WavelessOS installer!")

	DiskPartition() 
}

func DiskPartition() {
	diskSelect := ""
	sure := ""
	fmt.Println("Please enter the SSD you would like to install the system on:")
	fmt.Println("(example: /dev/sdb or /dev/nvme0n1)")
	fmt.Scanf("%s", &diskSelect)
	fmt.Printf("You have selected: %s\n continue? y/n \n", diskSelect)
	fmt.Scanf("%s" &sure)
	if sure == y{
		exec.Command("sudo umount %s", diskSelect)
		exec.Command("sudo dd if=/dev/zero of=%s bs=4M status=progress", diskSelect)
		exec.Command("sudo parted %s --script mklabel gpt", diskSelect)
		exec.Command("sudo parted %s --script mkpart FAT32 1MiB 513MiB", diskSelect)
		exec.Command("sudo parted  %s --script set 1 esp on", diskSelect)
		exec.Command("sudo parted %s --script mkpart ext4 513MiB 100%", diskSelect)

		
	}
	exec.Command("clear")

}