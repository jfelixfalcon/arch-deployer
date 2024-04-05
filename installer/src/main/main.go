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
	err = cmdExec("useradd -m -s /usr/bin/zsh -G wheel " + config.username + " -p '$y$j9T$jOPk2UThDe2fZxuc5jOnV1$CEp3s0fbtRNeVzmMJcoYF8ydCHI7Tzjw/pev//gkwOC'")
	if err != nil {
		return err
	}

	// Setting Grub
	err = cmdExec("pacman -S grub efibootmgr --noconfirm")
	if err != nil {
		return err
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
		log.Println("Installing ", service.name)

		err = cmdExec("pacman -S " + service.name + " --noconfirm")
		if err != nil {
			return err
		}

		log.Println("Enabling ", service.svc)

		err = cmdExec("systemctl enable " + service.svc)
		if err != nil {
			return err
		}

	}

	// Installing arch-install-scripts (dep)

	err = cmdExec("pacman -S arch-install-scripts --noconfirm")
	if err != nil {
		return err
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

	// Installing Additional Tools

	packets := []string{
		"xorg",
		"plasma",
		"plasma-wayland-session",
		"zsh",
		"zsh-completions",
		"zsh-lovers",
		"zsh-history-substring-search",
		"zsh-autosuggestions",
		"zsh-syntax-highlighting",
		"zsh-theme-powerlevel10k",
		"nerd-fonts",
		"git",
		"npm",
		"unzip",
		"okular",
		"docker",
		"docker-compose",
		"xclip",
		"pipewire",
		"firefox",
		"sshfs",
		"packagekit-qt5",
		"opensc",
		"pcsc-tools",
		"go",
		"base-devel",
		"dolphin",
		"neovim",
		"konsole",
	}

	for _, packet := range packets {
		err = cmdExec("pacman -S " + packet + " --noconfirm")
		if err != nil {
			return err
		}
	}

	// Enable additional services
	err = cmdExec("systemctl enable docker")
	if err != nil {
		return err
	}

	err = cmdExec("systemctl enable pcscd")
	if err != nil {
		return err
	}

	err = cmdExec("usermod -aG docker " + config.username)
	if err != nil {
		return err
	}

	err = cmdExec("systemctl enable sddm.service")
	if err != nil {
		return err
	}

	err = cmdExec("systemctl enable bluetooth")
	if err != nil {
		return err
	}

	file, err := os.OpenFile("/home/"+config.username+"/.zshrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.WriteString(
		"HISTSIZE=1000\nSAVEHIST=1000\nbindkey -e\nzstyle :compinstall filename '/home/predfalcn/.zshrc'\nautoload -Uz compinit\ncompinit\n",
	)
	if err != nil {
		return err
	}

	_, err = file.WriteString("source /usr/share/zsh-theme-powerlevel10k/powerlevel10k.zsh-theme\n")
	if err != nil {
		return err
	}

	_, err = file.WriteString("alias ls='ls --color=auto'\n")
	if err != nil {
		return err
	}

	err = cmdExec("chown " + config.username + ":" + config.username + " /home/predfalcn/.zshrc")
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

	// _, err = diskfs.Open(tmp.(string))
	// if err != nil {
	// 	log.Fatalln(err)
	// }

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

	// _, err = diskfs.Open(tmp.(string))
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	config.bootPartition = tmp.(string)

	// validating services
	tmp = installerConfig["services"]
	if tmp != nil {
		for _, svc := range tmp.([]interface{}) {

			st := svc.(map[string]interface{})

			config.services = append(config.services, service{name: st["name"].(string), svc: st["service"].(string)})
		}
	}

	// validating packages
	tmp = installerConfig["packages"]
	if tmp != nil {
		for _, pkg := range tmp.([]interface{}) {
			config.packets = append(config.packets, pkg.(string))

		}
	}

	return config
}

type serverConfig struct {
	hostname      string
	username      string
	driveName     string
	bootPartition string
	packets       []string
	services      []service
}

type service struct {
	name string
	svc  string
}
