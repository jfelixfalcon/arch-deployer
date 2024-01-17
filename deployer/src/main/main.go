package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"

	"github.com/schollz/progressbar/v3"
)

//go:embed bin/installer
var installer []byte

// Arguments to deploy
var (
	drive    = flag.String("d", "", "The drive to partition. Example: nvme0n0")
	bootPart = flag.String("b", "", "The boot partition. Example: p2")
	osPart   = flag.String("o", "", "The boot partition. Example: p1")
	hostname = flag.String("c", "", "The computer host name. Example: arch-master")
	bar      = progressbar.Default(370)
)

// Run commands in terminal
func cmdExec(args string) error {

	var stdin, stdout, stderr bytes.Buffer

	cmd := exec.Command("bash", "-c", args)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = &stdin

	log.Println("Command Executed: " + args)

	err := cmd.Run()
	if err != nil {
		return err
	}

	bar.Add(10)
	return nil
}

// Setting up btrfs
func btrfsSetup() error {

	subvolume := [7]string{"", "home", "root", "srv", "log", "cache", "tmp"}

	err := cmdExec("mount /dev/" + *drive + *osPart + " /mnt")

	if err != nil {
		return err
	}

	for _, volume := range subvolume {
		err = cmdExec("btrfs su cr /mnt/@" + volume)
		if err != nil {
			return err
		}
	}

	err = cmdExec("umount /mnt")
	if err != nil {
		return err
	}

	err = cmdExec("mount -o defaults,noatime,compress=zstd,commit=120,subvol=@ /dev/" + *drive +
		*osPart + " /mnt")
	if err != nil {
		return err
	}

	dirs := [6]string{"home", "root", "srv", "var/log", "var/cache", "tmp"}

	for _, dir := range dirs {
		err = cmdExec("mkdir -p /mnt/" + dir)
		if err != nil {
			return err
		}

	}

	for idx := range dirs {
		err = cmdExec("mount -o defaults,noatime,compress=zstd,commit=120,subvol=@" + subvolume[idx+1] +
			" /dev/" + *drive + *osPart + " /mnt/" + dirs[idx])
		if err != nil {
			return err
		}
	}

	return nil

}

// Setting up disk
func diskSetup() error {

	err := cmdExec("parted --script /dev/" + *drive + " mklabel gpt")
	if err != nil {
		return err
	}

	err = cmdExec("parted --script /dev/" + *drive + " mkpart EFI fat32 1MiB 512MiB")
	if err != nil {
		return err
	}

	err = cmdExec("parted --script /dev/" + *drive + " set " + "1" + " esp on")
	if err != nil {
		return err
	}

	err = cmdExec("parted --script /dev/" + *drive + " mkpart FILESYSTEM btrfs 512MiB 100%")
	if err != nil {
		return err
	}

	err = cmdExec("mkfs.fat -F32 /dev/" + *drive + *bootPart)
	if err != nil {
		return err
	}

	err = cmdExec("mkfs.btrfs /dev/" + *drive + *osPart + " -f")
	if err != nil {
		return err
	}

	err = btrfsSetup()
	if err != nil {
		return err
	}

	return nil

}

func cmdExecFile(args string, filename string, perms fs.FileMode) error {
	var stdin, stdout, stderr bytes.Buffer

	cmd := exec.Command("bash", "-c", args)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = &stdin

	log.Println("Command: " + args)

	err := cmd.Run()
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, stdout.Bytes(), perms)
	if err != nil {
		return err
	}

	bar.Add(10)
	return nil
}

// Pacstrap
func install() error {

	packets := [7]string{"base", "linux", "linux-firmware", "sudo", "vim", "python", "python-pip"}

	for _, packet := range packets {
		err := cmdExec("pacstrap -i /mnt " + packet + " --noconfirm")
		if err != nil {
			return err
		}
	}

	err := cmdExecFile("genfstab -U /mnt", "/mnt/etc/fstab", 0644)
	if err != nil {
		return err
	}

	log.Println("Copying installer to /mnt")
	err = os.WriteFile("/mnt/installer", installer, 0777)
	if err != nil {
		return err
	}

	fmt.Println("Run the following command to configure OS")
	fmt.Println("arch-chroot /mnt /installer -d " + *drive + " -h " + *hostname + " -b " + *bootPart)

	return nil
}

func main() {

	// Setting Logs record
	logfile, err := os.OpenFile("/tmp/deployer_log", os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Unable to set logfile: ", err)
		log.Fatalln("Unable to set logfile: ", err)
		return
	}

	defer logfile.Close()

	log.SetOutput(logfile)

	flag.Parse()

	if *drive == "" || *hostname == "" || *bootPart == "" || *osPart == "" {
		log.Println("All arguments are mandatory!")
		fmt.Println("All arguments are mandatory!")
		return
	}

	fmt.Println("Configuring Arch Linux!!!")

	err = cmdExec("pacman -Syy")
	if err != nil {
		log.Panic(err)
		return
	}

	err = diskSetup()
	if err != nil {
		log.Println(err.Error())
		return
	}

	err = install()
	if err != nil {
		log.Fatalln(err.Error())
		return
	}

}
