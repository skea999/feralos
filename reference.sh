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


mount -o compress=zstd:3,autodefrag,ssd /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt

btrfs subvolume create /mnt/@
btrfs subvolume create /mnt/@swapfile
btrfs subvolume create /mnt/@home
btrfs subvolume create /mnt/@opt
btrfs subvolume create /mnt/@root
btrfs subvolume create /mnt/@srv
btrfs subvolume create /mnt/@tmp
btrfs subvolume create /mnt/@var
btrfs subvolume create /mnt/@var-lib-ex
btrfs subvolume create /mnt/@var-lib-flatpak
btrfs subvolume create /mnt/@var-lib-libvirt
btrfs subvolume create /mnt/@var-lib-lxd
btrfs subvolume create /mnt/@var-lib-containerd
btrfs subvolume create /mnt/@var-lib-docker
btrfs subvolume create /mnt/@var-lib-podman
btrfs subvolume create /mnt/@var-cache
btrfs subvolume create /mnt/@var-log
btrfs subvolume create /mnt/@var-opt
btrfs subvolume create /mnt/@var-spool
btrfs subvolume create /mnt/@var-tmp
btrfs subvolume create /mnt/@usr-local


umount /mnt

mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@ /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt


mkdir -p /mnt/swapfile
mount -t btrfs -o compress=no,autodefrag,ssd,nodatacow,subvol=@swapfile /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/swapfile
chattr +C /mnt/swapfile
btrfs property set /mnt/swapfile compression none



mkdir -p /mnt/home
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@home /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/home


mkdir -p /mnt/opt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@opt /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/opt


mkdir -p /mnt/root
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@root /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/root


mkdir -p /mnt/srv
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@srv /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/srv
chattr +C /mnt/srv
btrfs property set /mnt/srv compression none



mkdir -p /mnt/tmp
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/tmp


mkdir -p /mnt/var
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var


mkdir -p /mnt/var/lib/ex
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-lib-ex /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/lib/ex
chattr +C /mnt/var/lib/ex
btrfs property set /mnt/var/lib/ex compression none



mkdir -p /mnt/var/lib/flatpak
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-flatpak /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/lib/flatpak


mkdir -p /mnt/var/lib/libvirt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/lib/libvirt


mkdir -p /mnt/var/lib/lxd
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-lxd /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/lib/lxd


mkdir -p /mnt/var/lib/containerd
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/lib/containerd


mkdir -p /mnt/var/lib/docker
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/lib/docker


mkdir -p /mnt/var/lib/podman
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-podman /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/lib/podman


mkdir -p /mnt/var/cache
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-cache /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/cache


mkdir -p /mnt/var/log
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-log /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/log
chattr +C /mnt/var/log
btrfs property set /mnt/var/log compression none



mkdir -p /mnt/var/opt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-opt /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/opt


mkdir -p /mnt/var/spool
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-spool /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/spool
chattr +C /mnt/var/spool
btrfs property set /mnt/var/spool compression none



mkdir -p /mnt/var/tmp
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-tmp /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/var/tmp
chattr +C /mnt/var/tmp
btrfs property set /mnt/var/tmp compression none



mkdir -p /mnt/usr/local
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@usr-local /dev/mapper/luks-$(blkid -o value -s UUID ${disk}3) /mnt/usr/local



mkdir /mnt/boot
mount /dev/mapper/luks-$(blkid -o value -s UUID ${disk}2) /mnt/boot

mkdir /mnt/boot/efi
mount ${disk}1 /mnt/boot/efi


