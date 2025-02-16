###############this file contains command that the script will execute in a more readable way#########
#####be carefull, this isn't an executable, you need to copy paste every line to make this to work####

setxkbmap it
sudo su -

chroot_dir=/mnt
mirror=https://dl-cdn.alpinelinux.org/alpine
arch=x86_64
branch=latest-stable
###### "apk-tools-static" version, find it here https://dl-cdn.alpinelinux.org/alpine/latest-stable/main/x86_64/
version=2.14.6-r3

######### disk where to install alpine, find it with "fdsik -l"##########
disk=/dev/vda

fdisk $disk
g# create new GPT partition
n# add new partition
1# partition number
# default - first sector 
+512M# partition size
n# add new partition
2# partition number
# default - first sector 
+2048M# partition size
n# add new partition
3# partition number
# default - first sector 
# partition size
t# change partition type
1# partition number
1# EFI
t# change partition type
2# partition number
20# Linux filesystem
t# change partition type
3# partition number
20# Linux filesystem
w# write partition table and exit



cryptsetup luksFormat --type luks1 ${disk}2
cryptsetup luksFormat --type luks2 ${disk}3

cryptsetup luksOpen ${disk}2 luks-$(blkid -o value -s UUID ${disk}2)
cryptsetup luksOpen ${disk}3 luks-$(blkid -o value -s UUID ${disk}3)

mkfs.vfat -F 32 ${disk}1
fatlabel ${disk}1 EFI

mkfs.ext4 -L grub /dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)
mkfs.btrfs -L root /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3)


mount -o compress=zstd:3,autodefrag,ssd /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}

btrfs subvolume create ${chroot_dir}/@
btrfs subvolume create ${chroot_dir}/@swapfile
btrfs subvolume create ${chroot_dir}/@home
btrfs subvolume create ${chroot_dir}/@opt
btrfs subvolume create ${chroot_dir}/@root
btrfs subvolume create ${chroot_dir}/@srv
btrfs subvolume create ${chroot_dir}/@tmp
btrfs subvolume create ${chroot_dir}/@var
btrfs subvolume create ${chroot_dir}/@var-lib-ex
btrfs subvolume create ${chroot_dir}/@var-lib-flatpak
btrfs subvolume create ${chroot_dir}/@var-lib-libvirt
btrfs subvolume create ${chroot_dir}/@var-lib-lxd
btrfs subvolume create ${chroot_dir}/@var-lib-containerd
btrfs subvolume create ${chroot_dir}/@var-lib-docker
btrfs subvolume create ${chroot_dir}/@var-lib-podman
btrfs subvolume create ${chroot_dir}/@var-cache
btrfs subvolume create ${chroot_dir}/@var-log
btrfs subvolume create ${chroot_dir}/@var-opt
btrfs subvolume create ${chroot_dir}/@var-spool
btrfs subvolume create ${chroot_dir}/@var-tmp
btrfs subvolume create ${chroot_dir}/@usr-local


umount ${chroot_dir}

mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@ /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}


mkdir -p ${chroot_dir}/swapfile
mount -t btrfs -o compress=no,autodefrag,ssd,nodatacow,subvol=@swapfile /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/swapfile
chattr +C ${chroot_dir}/swapfile
btrfs property set ${chroot_dir}/swapfile compression none



mkdir -p ${chroot_dir}/home
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@home /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/home


mkdir -p ${chroot_dir}/opt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@opt /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/opt


mkdir -p ${chroot_dir}/root
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@root /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/root


mkdir -p ${chroot_dir}/srv
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@srv /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/srv
chattr +C ${chroot_dir}/srv
btrfs property set ${chroot_dir}/srv compression none



mkdir -p ${chroot_dir}/tmp
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/tmp


mkdir -p ${chroot_dir}/var
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var


mkdir -p ${chroot_dir}/var/lib/ex
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-lib-ex /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/lib/ex
chattr +C ${chroot_dir}/var/lib/ex
btrfs property set ${chroot_dir}/var/lib/ex compression none



mkdir -p ${chroot_dir}/var/lib/flatpak
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-flatpak /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/lib/flatpak


mkdir -p ${chroot_dir}/var/lib/libvirt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/lib/libvirt


mkdir -p ${chroot_dir}/var/lib/lxd
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-lxd /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/lib/lxd


mkdir -p ${chroot_dir}/var/lib/containerd
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/lib/containerd


mkdir -p ${chroot_dir}/var/lib/docker
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/lib/docker


mkdir -p ${chroot_dir}/var/lib/podman
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-podman /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/lib/podman


mkdir -p ${chroot_dir}/var/cache
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-cache /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/cache


mkdir -p ${chroot_dir}/var/log
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-log /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/log
chattr +C ${chroot_dir}/var/log
btrfs property set ${chroot_dir}/var/log compression none



mkdir -p ${chroot_dir}/var/opt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-opt /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/opt


mkdir -p ${chroot_dir}/var/spool
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-spool /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/spool
chattr +C ${chroot_dir}/var/spool
btrfs property set ${chroot_dir}/var/spool compression none



mkdir -p ${chroot_dir}/var/tmp
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-tmp /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/var/tmp
chattr +C ${chroot_dir}/var/tmp
btrfs property set ${chroot_dir}/var/tmp compression none



mkdir -p ${chroot_dir}/usr/local
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@usr-local /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) ${chroot_dir}/usr/local



mkdir ${chroot_dir}/boot
mount /dev/mapper/luks-$(blkid -o value -s UUID ${disk}2) ${chroot_dir}/boot

mkdir ${chroot_dir}/boot/efi
mount ${disk}1 ${chroot_dir}/boot/efi



mkdir ${chroot_dir}/sys
mount --rbind /sys ${chroot_dir}/sys && mount --make-rslave ${chroot_dir}/sys
mkdir ${chroot_dir}/dev
mount --rbind /dev ${chroot_dir}/dev && mount --make-rslave ${chroot_dir}/dev
mkdir ${chroot_dir}/proc
mount --rbind /proc ${chroot_dir}/proc && mount --make-rslave ${chroot_dir}/proc



curl -LO ${mirror}/latest-stable/main/${arch}/apk-tools-static-${version}.apk
tar -xzf apk-tools-static-*.apk
./sbin/apk.static -X ${mirror}/latest-stable/main -U --allow-untrusted -p ${chroot_dir} --initdb add alpine-base

echo "nameserver 8.8.8.8" > ${chroot_dir}/etc/resolv.conf

mkdir -p ${chroot_dir}/etc/apk
echo "${mirror}/${branch}/main" > ${chroot_dir}/etc/apk/repositories
echo "${mirror}/${branch}/community" >> ${chroot_dir}/etc/apk/repositories

echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /                       btrfs    compress=zstd:3,autodefrag,ssd,subvol=@                                       0       1 " > ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /swapfile               btrfs    compress=no,autodefrag,ssd,nodatacow,subvol=@swapfile                         0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /home                   btrfs    compress=zstd:3,autodefrag,ssd,subvol=@home                                   0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /opt                    btrfs    compress=zstd:3,autodefrag,ssd,subvol=@opt                                    0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /root                   btrfs    compress=zstd:3,autodefrag,ssd,subvol=@root                                   0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /srv                    btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@srv          0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /tmp                    btrfs    compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp                      0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var                    btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var                                    0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/lib/ex             btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-lib-ex   0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/lib/flatpak        btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-flatpak                        0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/lib/libvirt        btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt                        0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/lib/lxd            btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-lxd                            0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/lib/containerd     btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd                     0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/lib/docker         btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker                         0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/lib/podman         btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-podman                         0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/cache              btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-cache                              0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/log                btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-log      0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/opt                btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-opt                                0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/spool              btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-spool    0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /var/tmp                btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-tmp      0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /usr/local              btrfs    compress=zstd:3,autodefrag,ssd,subvol=@usr-local                              0       0 " >> ${chroot_dir}/etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID ${disk}2)     /boot                   ext4     defaults                                                                      0       0 " >> ${chroot_dir}/etc/fstab
echo "UUID=$(blkid -o value -s UUID ${disk}1)                                            /boot/efi               vfat     defaults                                                                      0       0 " >> ${chroot_dir}/etc/fstab
echo "#tmpfs                                                   /tmp                    tmpfs    defaults,nosuid,nodev                                                        0       0" >> ${chroot_dir}/etc/fstab
echo "/swapfile/swapfile                                       none                    swap     defaults                                                                      0       0 " >> ${chroot_dir}/etc/fstab

chroot ${chroot_dir} /bin/ash -l

#########################Continue in chroot###############

rc-update add devfs sysinit
rc-update add dmesg sysinit
rc-update add mdev sysinit

rc-update add hwclock boot
rc-update add modules boot
rc-update add sysctl boot
rc-update add hostname boot
rc-update add bootmisc boot
rc-update add syslog boot

rc-update add mount-ro shutdown
rc-update add killprocs shutdown
rc-update add savecache shutdown

##############add packages##########
apk add doas musl-locales lang tzdata kbd-bkeymaps 
####################################

echo "permit persist :wheel" > /etc/doas.d/doas.conf

echo "ske-nb" > /etc/hostname
echo "127.0.0.1 ske-nb ske-nb.localdomain" >> /etc/hosts
echo "::1 ske-nb ske-nb.localdomain" >> /etc/hosts


#locale -a
cp /etc/profile.d/20locale.sh /etc/profile.d/20locale.sh.sh
echo "export CHARSET=it_IT.UTF-8" > /etc/profile.d/20locale.sh.sh
echo "export LANG=it_IT.UTF-8" >> /etc/profile.d/20locale.sh.sh
echo "LC_COLLATE=it_IT.UTF-8" >> /etc/profile.d/20locale.sh.sh

ln -s /usr/share/zoneinfo/Europe/Rome /etc/localtime

echo "KEYMAP=\"/usr/share/bkeymaps/it/it.bmap.gz\"" >> /etc/conf.d/loadkmap
rc-update add loadkmap default #don't know if it is better to start thin on other runlevels



