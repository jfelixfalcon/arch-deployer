package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"

	"github.com/schollz/progressbar/v3"
)

var (
	drive    = flag.String("d", "", "The drive to partition. Example: nvme0n0")
	hostname = flag.String("c", "", "The computer host name. Example: arch-master")
	bootPart = flag.String("b", "", "The boot partition. Example: p2")
	uname    = flag.String("u", "", "The username to create. Example: admin")
	bar      = progressbar.Default(570)
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

	bar.Add(10)
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

func configure() error {
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
	err = cmdExecFile("echo "+*hostname, "/etc/hostname", 0755)
	if err != nil {
		return err
	}

	// Setting Hosts
	err = cmdExecFile(
		"printf  '127.0.0.1   localhost\n::1   localhost\n127.0.1.1   "+*hostname+"'",
		"/etc/hosts",
		0755,
	)
	if err != nil {
		return err
	}

	// Creating User
	err = cmdExec("useradd -m -s /usr/bin/zsh -G wheel " + *uname + " -p '$y$j9T$jOPk2UThDe2fZxuc5jOnV1$CEp3s0fbtRNeVzmMJcoYF8ydCHI7Tzjw/pev//gkwOC'")
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

	err = cmdExec("mount /dev/" + *drive + *bootPart + " /boot/efi")
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

	// Installing Services

	// Setting Network
	err = cmdExec("pacman -S networkmanager --noconfirm")
	if err != nil {
		return err
	}

	err = cmdExec("systemctl enable NetworkManager")
	if err != nil {
		return err
	}

	// Installing Openssh
	err = cmdExec("pacman -S openssh --noconfirm")
	if err != nil {
		return err
	}

	err = cmdExec("systemctl enable sshd")
	if err != nil {
		return err
	}

	// Install Firewalld
	err = cmdExec("pacman -S firewalld --noconfirm")
	if err != nil {
		return err
	}

	err = cmdExec("systemctl enable firewalld")
	if err != nil {
		return err
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

	err = cmdExec("usermod -aG docker " + *uname)
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

	file, err := os.OpenFile("/home/"+*uname+"/.zshrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

	err = cmdExec("chown " + *uname + ":" + *uname + " /home/predfalcn/.zshrc")
	if err != nil {
		return err
	}

	return nil
}

func main() {
	logfile, err := os.OpenFile("installer_log", os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Unable to set logfile:", err)
		log.Fatalln("Unable to set logfile:", err)
	}

	defer logfile.Close()

	log.SetOutput(logfile)

	flag.Parse()

	if *drive == "" || *hostname == "" || *uname == "" {
		log.Println("Drive Uname, Name and Hostname arguments are mandatory!")
		fmt.Println("Drive Uname, Name and Hostname arguments are mandatory!")
		return
	}

	flag.Parse()

	err = configure()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println("Configuration Completed !!!")
}
