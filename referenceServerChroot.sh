#! /bin/sh

. /referenceServerVariables.sh

##############


echo "nameserver 208.67.222.222" > /etc/resolv.conf
echo "nameserver 208.67.220.220" >> /etc/resolv.conf

mkdir -p /etc/apk
echo "${mirror}/${branch}/main" > /etc/apk/repositories
echo "${mirror}/${branch}/community" >> /etc/apk/repositories

echo "LABEL=root     /                       btrfs    compress=zstd:3,autodefrag,ssd,subvol=@                                       0       1 " > /etc/fstab
echo "LABEL=root     /swap                   btrfs    compress=no,autodefrag,ssd,nodatacow,subvol=@swap                             0       0 " >> /etc/fstab
echo "LABEL=root     /home                   btrfs    compress=zstd:3,autodefrag,ssd,subvol=@home                                   0       0 " >> /etc/fstab
echo "LABEL=root     /opt                    btrfs    compress=zstd:3,autodefrag,ssd,subvol=@opt                                    0       0 " >> /etc/fstab
echo "LABEL=root     /root                   btrfs    compress=zstd:3,autodefrag,ssd,subvol=@root                                   0       0 " >> /etc/fstab
echo "LABEL=root     /srv                    btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@srv          0       0 " >> /etc/fstab
echo "LABEL=root     /tmp                    btrfs    compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp                      0       0 " >> /etc/fstab
echo "LABEL=root     /var                    btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var                                    0       0 " >> /etc/fstab
echo "LABEL=root     /var/lib/ex             btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-lib-ex   0       0 " >> /etc/fstab
echo "LABEL=root     /var/lib/libvirt        btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt                        0       0 " >> /etc/fstab
echo "#LABEL=root     /var/lib/incus          btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-incus                          0       0 " >> /etc/fstab
echo "LABEL=root     /var/lib/containerd     btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd                     0       0 " >> /etc/fstab
echo "LABEL=root     /var/lib/docker         btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker                         0       0 " >> /etc/fstab
echo "LABEL=root     /var/lib/podman         btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-podman                         0       0 " >> /etc/fstab
echo "LABEL=root     /var/cache              btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-cache                              0       0 " >> /etc/fstab
echo "LABEL=root     /var/log                btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-log      0       0 " >> /etc/fstab
echo "LABEL=root     /var/opt                btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-opt                                0       0 " >> /etc/fstab
echo "LABEL=root     /var/spool              btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-spool    0       0 " >> /etc/fstab
echo "LABEL=root     /var/tmp                btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-tmp      0       0 " >> /etc/fstab
echo "LABEL=root     /usr/local              btrfs    compress=zstd:3,autodefrag,ssd,subvol=@usr-local                              0       0 " >> /etc/fstab
echo "LABEL=boot     /boot                   btrfs    defaults                                                                      0       0 " >> /etc/fstab
echo "LABEL=EFI$(echo ${disk1} | cut -d'/' -f3 | tr '[:lower:]' '[:upper:]')      /boot/efi               vfat     defaults                                                                      0       0 " >> /etc/fstab
echo "LABEL=EFI$(echo ${disk2} | cut -d'/' -f3 | tr '[:lower:]' '[:upper:]')      /boot/efi2               vfat     defaults                                                                      0       0 " >> /etc/fstab
echo "#tmpfs          /tmp                    tmpfs    defaults,nosuid,nodev                                                         0       0 " >> /etc/fstab
echo "LABEL=swap-$(echo ${disk1} | cut -d'/' -f3)     none                    swap     defaults,sw                                                                    0       0 " >> /etc/fstab
echo "LABEL=swap-$(echo ${disk2} | cut -d'/' -f3)     none                    swap     defaults,sw                                                                    0       0 " >> /etc/fstab
echo "LABEL=data-$(echo ${disk1} | cut -d'/' -f3)     ${chrootDir}/data/data-$(echo ${disk1} | cut -d'/' -f3)                   btrfs    defaults                                                                      0       0 " >> /etc/fstab
echo "LABEL=data-$(echo ${disk2} | cut -d'/' -f3)     ${chrootDir}/data/data-$(echo ${disk2} | cut -d'/' -f3)                   btrfs    defaults                                                                      0       0 " >> /etc/fstab

##############add packages##########
apk add micro doas parted mdadm musl-locales lang tzdata kbd-bkeymaps grub-efi efibootmgr linux-lts btrfs-progs zstd cronie chrony mesa mesa-utils mesa-dri-gallium mesa-va-gallium mesa-vdpau-gallium xf86-video-amdgpu vulkan-loader vulkan-tools libvirt-daemon qemu-img qemu-system-x86_64 qemu-modules font-terminus incus-feature incus-feature-vm incus-feature-client incus-feature-utils dbus bridge bridge-utils zram-init
#uutils uutils-coreutils uutils-coreutils-hostname uutils-coreutils-stat grep findutils pciutils usbutils
####################################

echo raid1 >> /etc/modules-load.d/raid1.conf

mdadm --detail --scan > /etc/mdadm.conf

echo 'alias ll="ls -lah"' > /etc/profile.d/alias.sh
chmod +x /etc/profile.d/alias.sh

echo 'alias sudo=doas' >> /etc/profile.d/alias.sh
echo 'alias nano=micro' >> /etc/profile.d/alias.sh
echo 'alias rm="rm -i"' >> /etc/profile.d/alias.sh
echo 'alias apk="apk --progress"' >> /etc/profile.d/alias.sh
echo 'alias untar="tar xzvkf"' >> /etc/profile.d/alias.sh
echo 'alias mv="mv -v"' >> /etc/profile.d/alias.sh


echo 'export EDITOR=micro' > /etc/profile.d/editor.sh
chmod +x /etc/profile.d/editor.sh

touch /etc/profile.d/console.sh
tee /etc/profile.d/console.sh > /dev/null <<'EOF'
COL_YEL="\[\e[1;33m\]"
COL_GRA="\[\033[0;37m\]"
COL_WHI="\[\033[0m\]"
COL_GRE="\[\033[32;1m\]"
COL_RED="\[\033[31;1m\]"

if [ "$(whoami)" = "root" ] ; then
    _COL_USER=$COL_RED
    _p="#"
else
    _COL_USER=$COL_GRE
    _p=">"
fi
PS1="${_COL_USER}\u${COL_WHI}@${COL_YEL}\h${COL_WHI}:\w${_p} \[\033[0m\]"
EOF

chmod +x /etc/profile.d/console.sh


rc-update add devfs sysinit
rc-update add dmesg sysinit
rc-update add mdev sysinit

rc-update add mdadm boot
rc-update add mdadm-raid boot
rc-update add hwclock boot
rc-update add modules boot
rc-update add sysctl boot
rc-update add hostname boot
rc-update add bootmisc boot
rc-update add syslog boot

rc-update add swap default

rc-update add mount-ro shutdown
rc-update add killprocs shutdown
rc-update add savecache shutdown

rc-update add cronie default
rc-update add chronyd default
rc-update add dbus default

rc-update add incusd default 

echo "permit persist :wheel" > /etc/doas.d/doas.conf

echo "ske-server1" > /etc/hostname
echo "127.0.0.1 ske-server1 ske-server1.localdomain" >> /etc/hosts
echo "::1 ske-server1 ske-server1.localdomain" >> /etc/hosts


#locale -a
cp /etc/profile.d/20locale.sh /etc/profile.d/20locale.sh.sh
echo "export CHARSET=it_IT.UTF-8" > /etc/profile.d/20locale.sh.sh
echo "export LANG=it_IT.UTF-8" >> /etc/profile.d/20locale.sh.sh
echo "LC_COLLATE=it_IT.UTF-8" >> /etc/profile.d/20locale.sh.sh

ln -s /usr/share/zoneinfo/Europe/Rome /etc/localtime

echo "KEYMAP=\"/usr/share/bkeymaps/it/it.bmap.gz\"" >> /etc/conf.d/loadkmap
rc-update add loadkmap default #don't know if it is better to start thin on other runlevels

mkdir -p /etc/apk/commit_hooks.d/

touch /etc/apk/commit_hooks.d/keep-old-kernel.sh
tee /etc/apk/commit_hooks.d/keep-old-kernel.sh > /dev/null <<'EOF'
#!/bin/sh

STAGE="${1}"
targetCmd="upgrade"
targetPkg="linux-lts" # Could be a placeholder for the installation script

# This hook make a copy of the old kernel to fall back in case something appen

# Execute the script before apk. Check if apk is doing a command defined in ${targetCmd} (possible command are: upgrade add del fix)
if [ "${STAGE}" = "pre-commit" ] && [ "${APK_COMMAND}" = "${targetCmd}" ]; then
    echo "Executing ${targetPkg}-keep-old-kernel.sh pre-commit hook because apk is doing the command ${targetCmd}..."
    
    # Simulate upgrade and if the package is upgraded than process the hook
    if apk upgrade --simulate 2>&1 | grep -qE "(will be installed|is now available)" | grep -q "${targetPkg}"; then
        echo "Package ${targetPkg} is beeing upgraded, proceed with the hook:"

        # ---------- Functions here ----------

        # -----------------------------------------------------------

        # ---------- Script here ----------

        # Check the current version
        kernelVersion=$(uname -r)
        # Only act if there are the files to copy
        if [ -f "/boot/vmlinuz-${kernelVersion}" ] && [ -f "${CURRENT_INITRD}" ]; then
            # Overwrite any previous fallback copy
            cp -afX "/boot/vmlinuz-${kernelVersion}" "/boot/vmlinuz-fallback"
            cp -afX "/boot/initramfs-${kernelVersion}.img" "/boot/initramfs-fallback.img"
        else
            echo "Warning: current kernel files not found - skipping fallback creation." 
        fi
        
        # -----------------------------------------------------------
        
    else
        echo "Package ${targetPkg} is not beeing upgraded."
    fi

fi
exit 0
EOF

chmod +x /etc/apk/commit_hooks.d/keep-old-kernel.sh


touch /etc/apk/commit_hooks.d/copy-efi.sh
tee /etc/apk/commit_hooks.d/copy-efi.sh > /dev/null <<'EOF'
#!/bin/sh

STAGE="${1}"
targetCmd="upgrade"
targetPkg="linux-lts"


# This hook will do an rsync from /boot/efi to /boot/efi2 to make a sort of raid 1 on the efi partition

# Execute the script after apk. Check if apk is doing a command defined in ${targetCmd} (possible command are: upgrade add del fix)
if [ "${STAGE}" = "post-commit" ] && [ "${APK_COMMAND}" = "${targetCmd}" ]; then
    echo "Executing ${targetPkg}-raid-efi.sh post-commit hook because apk is doing the command ${targetCmd}... "
    
    # Check apk log
    latestLog=$(ls -t /var/log/apk/* | head -n 1)

    # If the file exists
    if [ -f "${latestLog}" ]; then
        
        # Select oly lines containing install or upgrade and than if those lines contain $targetPkg
        if grep -qE "install|upgrade" "${latestLog}" | grep -q "${targetPkg}"; then
            echo "Found ${targetPkg}. Executing script now... "
            
            # ---------- Functions here ----------

            # -----------------------------------------------------------

            # ---------- Script here ----------

            # Check the current version
            kernelVersion=$(uname -r)
            # Only act if there are the files to copy
            if [ -f "/boot/vmlinuz-${kernelVersion}" ] && [ -f "${CURRENT_INITRD}" ]; then
                # Overwrite any previous fallback copy
                rsync -a --delete /boot/efi/ /boot/efi2/
            else
                echo "Warning: current kernel files not found - skipping fallback creation." 
            fi
            
            # -----------------------------------------------------------
        else
            echo "${targetPkg} not found. "
        fi
    else
        echo "Warning: Log file not found. "
    fi
fi

exit 0
EOF

chmod +x /etc/apk/commit_hooks.d/copy-efi.sh

# initialGrubCmdLine=$(cat /etc/default/grub | grep "GRUB_CMDLINE_LINUX_DEFAULT=" | sed "s/GRUB_CMDLINE_LINUX_DEFAULT=//g" | sed "s/\"//g")

# #additionalGrubCmdLineArgument=' cgroup_enable=cpuset cgroup_memory=1 cgroup_enable=memory'
# newGrubCmdLine="loglevel=0 quiet modules=sd-mod,usb-storage,btrfs,nvme rootfstype=btrfs"

# sed -i "s/${initialGrubCmdLine}/${newGrubCmdLine}/g" /etc/default/grub

echo "GRUB_CMDLINE_LINUX_DEFAULT=\"loglevel=0 quiet modules=sd-mod,usb-storage,btrfs,nvme rootfstype=btrfs\"" > /etc/default/grub
echo "GRUB_DISABLE_OS_PROBER=false" >> /etc/default/grub
echo "GRUB_PRELOAD_MODULES=\"part_gpt \""  >> /etc/default/grub 



##### this is not automatic### reference https://wiki.alpinelinux.org/wiki/Full_disk_encryption_secure_boot
#nano /etc/mkinitfs/mkinitfs.conf
#add "ide scsi usb virtio ext4 btrfs nvme keymap cryptsetup cryptkey resume raid" like features="ata base cdrom usb virtio ext4 btrfs keymap kms mmc nvme cryptsetup cryptkey resume raid" be carefull order is important

initialMkinitLine=$(cat /etc/mkinitfs/mkinitfs.conf | grep "features=" | sed "s/features=//g" | sed "s/\"//g")

# additionalMkinitLine=' btrfs nvme resume raid raid'
# newMkinitLine="${initialMkinitLine} ${additionalMkinitLine}"

newMkinitLine="ata base cdrom usb virtio btrfs keymap kms mmc nvme resume raid"

sed -i "s/${initialMkinitLine}/${newMkinitLine}/g" /etc/mkinitfs/mkinitfs.conf


echo "systemd_container=yes" >> /etc/conf.d/lxc

#Allow Qemu to use our bridge.
# echo 'allow br20' > /etc/qemu/bridge.conf
# echo 'allow br30' >> /etc/qemu/bridge.conf
# echo 'allow br50' >> /etc/qemu/bridge.conf
#Write some sysctl knobs to allow bridging to work by creating a file etc/sysctl.d/bridging.conf as follows
# echo '# Enable bridge forwarding.' > /etc/sysctl.d/bridging.conf
# echo 'net.ipv4.conf.br0_bc_forwarding=1' >> /etc/sysctl.d/bridging.conf
# echo '# Ignore iptables on bridge interfaces.' >> /etc/sysctl.d/bridging.conf
# echo 'net.bridge.bridge-nf-call-iptables=0' >> /etc/sysctl.d/bridging.conf

passwd root << EOF
${password}
${password}
EOF

adduser ske << EOF
${password}
${password}
EOF

adduser ske ske
adduser ske wheel
adduser ske lp
adduser ske input
adduser ske cdrom
adduser ske dialout
adduser ske video
adduser ske netdev
addgroup plugdev
adduser ske plugdev
adduser ske users
adduser ske libvirt
adduser ske qemu
adduser ske kvm

echo amdgpu >> /etc/modules
echo fbcon >> /etc/modules
echo tun >> /etc/modules
echo "tap" >> /etc/modules
echo "zram" >> /etc/modules

# echo "session optional pam_cgfs.so -c freezer,memory,name=systemd,unified" >> /etc/pam.d/system-login #?
# echo "lxc.idmap = u 0 100000 65536" >> /etc/incus/default.conf
# echo "lxc.idmap = g 0 100000 65536" >> /etc/incus/default.conf
# echo "systemd_container=yes" >>/etc/conf.d/incusd 

echo "root:100000:65536" >> /etc/subuid
echo "root:100000:65536" >> /etc/subgid

# chsh root -s /bin/zsh
# chsh ske -s /bin/zsh

echo "* hard nofile 100000" >> /etc/security/limits.conf
echo "* soft nofile 100000" >> /etc/security/limits.conf
echo "root hard nofile 100000" >> /etc/security/limits.conf
echo "root soft nofile 100000" >> /etc/security/limits.conf
echo "ske hard nofile 524288" >> /etc/security/limits.conf
echo "ske soft nofile 524288" >> /etc/security/limits.conf

#Configuration zram
echo 'load_on_start=yes' > /etc/conf.d/zram-init
echo 'unload_on_stop=yes' >> /etc/conf.d/zram-init
echo 'num_devices=1' >> /etc/conf.d/zram-init
echo 'type0=swap' >> /etc/conf.d/zram-init
echo 'size0=8000' >> /etc/conf.d/zram-init

rc-update add zram-init default


apk update

apk upgrade

#final staps
echo "installing grub:"
grub-install --target=x86_64-efi --efi-directory=/boot/efi2 ${disk2} --bootloader-id=Alpine2 --recheck --no-floppy
grub-install --target=x86_64-efi --efi-directory=/boot/efi ${disk1} --bootloader-id=Alpine1 --recheck --no-floppy

echo "regenerating initramfs:"
mkinitfs -c /etc/mkinitfs/mkinitfs.conf -b / $(ls /lib/modules/)
echo "regenerating grub:"
grub-mkconfig -o /boot/grub/grub.cfg

exit