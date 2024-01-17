# Arch Deployer

## Default Basic Configuration

### Storage
  - Partitions
    - OS (BTRFS)
    - BOOT (UEFI)
  - Subvolumes
    - home -> home
    - root -> root
    - srv -> srv
    - log -> var/log
    - cache -> var/cache
    - tmp -> tmp

### Packages
  - pacman
    - xorg
    - plasma
    - plasma-wayland-session
    - zsh
    - zsh-completions
    - zsh-lovers
    - zsh-history-substring-search
    - zsh-autosuggestions
    - zsh-syntax-highlighting
    - zsh-theme-powerlevel10k
    - nerd-fonts
    - git
    - npm
    - unzip
    - okular
    - docker
    - docker-compose
    - xclip
    - pipewire
    - firefox
    - sshfs
    - packagekit-qt5
    - opensc
    - pcsc-tools
    - go
    - base-devel
    - dolphin
    - neovim
    - konsole
    - networkmanager
    - openssh
    - firewalld
    - arch-installer-scripts

## Build binaries
  Note: Update the Makefiles for your specific environment and update the code as well if needed.

    make build

## Run Command Example
  - Environment
    - Drive: sda
    - Hostname: arch-main
    - Username: admin

  - Deploy

        ./arch-deployer -d sda -b 1 -o 2 -c arch-main
  - Install
   
        arch-chroot /mnt/ /installer -d sda -b 1 -c arch-main -u admin