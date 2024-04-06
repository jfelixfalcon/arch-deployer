package main

import (
	"bytes"
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

var (
	configFile = flag.String("f", "", "Server configuration file")
)

func cmdExec(args string) error {
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

func configure(config serverConfig) error {

	// Setting Locale
	err := cmdExec("sed -i 's/#en_US.UTF-8/en_US.UTF-8/' /etc/locale.gen")
	if err != nil {
		return err
	}

	err = cmdExec("sed -i 's/#en_US ISO-8859-1/en_US ISO-8859-1/' /etc/locale.gen")
	if err != nil {
		return err
	}

	err = cmdExec("locale-gen")
	if err != nil {
		return err
	}

	// Setting NTP
	err = cmdExecFile("echo LANG=en_US.UTF-8", "/etc/locale.conf", 0644)
	if err != nil {
		return err
	}

	err = cmdExec("ln -sf /usr/share/zoneinfo/America/New_York /etc/localtime")
	if err != nil {
		return err
	}

	// Setting Hostname
	err = cmdExecFile("echo "+config.hostname, "/etc/hostname", 0755)
	if err != nil {
		return err
	}

	// Setting Hosts
	err = cmdExecFile(
		"printf  '127.0.0.1   localhost\n::1   localhost\n127.0.1.1   "+config.hostname+"'",
		"/etc/hosts",
		0755,
	)
	if err != nil {
		return err
	}

	// Creating User
	err = cmdExec("useradd -m -s /usr/bin/zsh -G wheel " + config.username + " -p '" + config.password + "'")
	if err != nil {
		return err
	}

	// Installing Additional Packages

	for _, packet := range config.packages {
		log.Println("Installing package: ", packet)
		err = cmdExec("pacman -S " + packet + " --noconfirm")
		if err != nil {
			return err
		}

		log.Println(packet, " installed...")
	}

	err = cmdExec("mkdir /boot/efi")
	if err != nil {
		return err
	}

	err = cmdExec("mount " + config.bootPartition + " /boot/efi")
	if err != nil {
		return err
	}

	err = cmdExec("grub-install --target=x86_64-efi --bootloader-id=GRUB --efi-directory=/boot/efi")
	if err != nil {
		return err
	}

	err = cmdExec("grub-mkconfig -o /boot/grub/grub.cfg")
	if err != nil {
		return err
	}

	// Setting Services

	for _, service := range config.services {
		log.Println("Enabling ", service)

		err = cmdExec("systemctl enable " + service)
		if err != nil {
			return err
		}

		log.Println(service, " enabled...")

	}

	// Updating fstab

	err = cmdExec("chmod g+w /etc/fstab")
	if err != nil {
		return err
	}

	err = cmdExec("chown :wheel /etc/fstab")
	if err != nil {
		return err
	}

	err = cmdExec("usermod -aG docker " + config.username)
	if err != nil {
		return err
	}

	file, err := os.OpenFile("/home/"+config.username+"/.zshrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.WriteString("alias ls='ls --color=auto'\n")
	if err != nil {
		return err
	}

	file, err = os.OpenFile("/home/"+config.username+"/.histfile", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer file.Close()

	err = cmdExec("chown -R " + config.username + ":" + config.username + " /home/" + config.username)
	if err != nil {
		return err
	}

	return nil
}

// Entry Point
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

	err = configure(config)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println("Configuration Completed !!!")
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

	if (yamlConfig["installer"]) == nil {
		log.Fatalln("installer value not found in config file...")
	}

	installerConfig := yamlConfig["installer"].(map[string]interface{})

	// validating mandatory configFile information

	// validating drive name
	tmp := installerConfig["driveName"]

	if tmp == nil {
		log.Fatalln("Configuration missing required parameter {driveName}")
	}

	// checking if drive exist

	_, err = diskfs.Open(tmp.(string))
	if err != nil {
		log.Fatalln(err)
	}

	config.driveName = tmp.(string)

	// validating username
	tmp = installerConfig["username"]
	if tmp == nil {
		log.Fatalln("Configuration missing required parameter {username}")
	}

	if match, _ := regexp.MatchString("^[a-z_][a-z0-9_-]*[$]?", tmp.(string)); !match {
		log.Fatalln("Invalid username: ", tmp.(string))
	}

	config.username = tmp.(string)

	// validating password
	tmp = installerConfig["password"]
	if tmp == nil {
		log.Fatalln("Configuration missing required parameter {password}")
	}

	config.password = tmp.(string)

	// validating hostname
	tmp = installerConfig["hostname"]
	if tmp == nil {
		log.Fatalln("Configuration missin required parameter {hostname}...")
	}

	if match, _ := regexp.MatchString("^[a-z_][a-z0-9_-]*[$]?", tmp.(string)); !match {
		log.Fatalln("Invalid hostname: ", tmp.(string))
	}

	config.hostname = tmp.(string)

	// validating boot partition
	tmp = installerConfig["bootPartition"]
	if tmp == nil {
		log.Fatalln("Configuration missin required parameter {bootPartition}...")
	}

	// checking if partition exist

	_, err = diskfs.Open(tmp.(string))
	if err != nil {
		log.Fatalln(err)
	}

	config.bootPartition = tmp.(string)

	// validating services
	tmp = installerConfig["services"]
	if tmp != nil {
		for _, svc := range tmp.([]interface{}) {

			config.services = append(config.services, svc.(string))
		}
	}

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
	username      string
	password      string
	driveName     string
	bootPartition string
	packages      []string
	services      []string
}
