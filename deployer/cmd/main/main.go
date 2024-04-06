package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"regexp"

	"github.com/diskfs/go-diskfs"
	"gopkg.in/yaml.v3"
)

//go:embed bin/installer
var installer []byte

// Arguments to deploy
var (
	configFile = flag.String("f", "", "Server configuration file")
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
		fmt.Println(cmd.Stderr)
		return err
	}

	return nil
}

// Setting up btrfs
func btrfsSetup(config serverConfig) error {

	subvolume := [7]string{"", "home", "root", "srv", "log", "cache", "tmp"}

	err := cmdExec("mount " + config.osPartition + " /mnt")

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

	err = cmdExec("mount -o defaults,noatime,compress=zstd,commit=120,subvol=@ " + config.osPartition + " /mnt")
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
			" " + config.osPartition + " /mnt/" + dirs[idx])
		if err != nil {
			return err
		}
	}

	return nil

}

// Setting up disk
func diskSetup(config serverConfig) error {

	err := cmdExec("parted --script " + config.driveName + " mklabel gpt")
	if err != nil {
		return err
	}

	err = cmdExec("parted --script " + config.driveName + " mkpart EFI fat32 1MiB 512MiB")
	if err != nil {
		return err
	}

	err = cmdExec("parted --script " + config.driveName + " set " + "1" + " esp on")
	if err != nil {
		return err
	}

	err = cmdExec("parted --script " + config.driveName + " mkpart FILESYSTEM btrfs 512MiB 100%")
	if err != nil {
		return err
	}

	err = cmdExec("mkfs.btrfs " + config.osPartition + " -f")
	if err != nil {
		return err
	}

	err = cmdExec("mkfs.fat -F32 " + config.bootPartition)
	if err != nil {
		return err
	}

	err = btrfsSetup(config)
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

	return nil
}

// Pacstrap
func install(config serverConfig) error {

	for _, packet := range config.packages {
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
	fmt.Println("arch-chroot /mnt /installer -f {configFile.yaml}")

	return nil
}

func main() {

	// Setting logfile
	logfile, err := os.OpenFile("/tmp/arch-installer.log", os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Unable to set logfile:", err)
		log.Fatalln("Unable to set logfile:", err)
	}

	defer logfile.Close()

	// Sending output to both stdout and file
	mw := io.MultiWriter(os.Stdout, logfile)

	log.SetOutput(mw)

	flag.Parse()

	if len(*configFile) == 0 {
		log.Fatalln("ConfigFile argument missing...")
	}

	config := validateConfig(*configFile)

	log.Println("Configuring Arch Linux!!!")

	err = cmdExec("pacman -Syy")
	if err != nil {
		log.Panic(err)
		return
	}

	err = diskSetup(config)
	if err != nil {
		log.Println(err.Error())
		return
	}

	err = install(config)
	if err != nil {
		log.Fatalln(err.Error())
		return
	}

}

func validateConfig(configFile string) serverConfig {

	// structure to store configuration file
	var config serverConfig

	cfile, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalln(err, " ...")
	}

	yamlConfig := make(map[interface{}]interface{})

	err = yaml.Unmarshal(cfile, &yamlConfig)
	if err != nil {
		log.Fatalln(err, "...")
	}

	if (yamlConfig["deployer"]) == nil {
		log.Fatalln("deployer value not found in config file...")

	}

	installerConfig := yamlConfig["deployer"].(map[string]interface{})

	// validating hostname
	tmp := installerConfig["hostname"]
	if tmp == nil {
		log.Fatalln("Configuration missin required parameter {hostname}...")
	}

	if match, _ := regexp.MatchString("^[a-z_][a-z0-9_-]*[$]?", tmp.(string)); !match {
		log.Fatalln("Invalid hostname: ", tmp.(string))
	}

	config.hostname = tmp.(string)

	// validating drive name
	tmp = installerConfig["driveName"]

	if tmp == nil {
		log.Fatalln("Configuration missing required parameter {driveName}")
	}

	// checking if drive exist

	dsk, err := diskfs.Open(tmp.(string))
	if err != nil {
		log.Fatalln(err)
	}

	config.driveName = tmp.(string)
	dsk.File.Close()

	// validating boot partition
	tmp = installerConfig["bootPartition"]
	if tmp == nil {
		log.Fatalln("Configuration missin required parameter {bootPartition}...")
	}

	config.bootPartition = tmp.(string)

	// validating os partition
	tmp = installerConfig["osPartition"]
	if tmp == nil {
		log.Fatalln("Configuration missin required parameter {osPartition}...")
	}

	config.osPartition = tmp.(string)

	// validating packages
	tmp = installerConfig["packages"]
	if tmp != nil {
		for _, pkg := range tmp.([]interface{}) {
			config.packages = append(config.packages, pkg.(string))

		}
	}

	return config
}

type serverConfig struct {
	hostname      string
	osPartition   string
	bootPartition string
	driveName     string
	packages      []string
}
