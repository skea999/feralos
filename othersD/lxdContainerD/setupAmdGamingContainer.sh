#!/bin/sh

lxc ubuntu wgames <<EOF

sudo su -
dpkg --add-architecture i386

apt update
apt upgrade

add-apt-repository -y ppa:kisak/kisak-mesa

add-apt-repository -y ppa:lutris-team/lutris

apt update
apt upgrade -y
#
#apt install -y lutris x11-apps mesa-utils libgl1-mesa-glx libcanberra-gtk-module wine64 wine32 libasound2-plugins:i386 libsdl2-2.0-0:i386 libdbus-1-3:i386 libsqlite3-0:i386 libgnutls30:i386 libgpg-error0:i386 libsqlite3-0:i386 libgl1-mesa-dri:i386 mesa-vulkan-drivers mesa-vulkan-drivers:i386

apt install -y libgl1-mesa-dri:i386 mesa-vulkan-drivers mesa-vulkan-drivers:i386 wine64 wine32 libasound2-plugins:i386 libsdl2-2.0-0:i386 libdbus-1-3:i386 libsqlite3-0:i386 lutris

apt install dolphin --no-install-recommends --no-install-suggests

EOF


lxc config set wgames limits.kernel.nofile 524288

lxc restart wgames
