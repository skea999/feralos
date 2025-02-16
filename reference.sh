###############this file contains command that the script will execute in a more readable way#########

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

