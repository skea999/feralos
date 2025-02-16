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

