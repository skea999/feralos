# Skea Installer

This is a simple script for installing Void Linux musl on encrypted /boot and encrypted btrfs / with subvolumes.
The init will be changed to s6 and it will be a lxd container for glibc compatibility.
This script is simply hackable, enjoy it.


What the script will do: 
1. Encrypted boot and root, 
2. /boot/efi in fat32 unencrypted, 512 Mb reccomanded, 
3. /boot in ext4 encrypted in LUKS1, 4 Gb reccomended, 
4. / in btrfs encrypted in LUKS2, 
5. / key in initrfs for typing password once, 
6. swapfile in a btrfs subvolume with no COW and no compression, 16 Gb, 
7. subvolumes schemes are copyed by OpenSuse and Void-valut configuration, in that way nothing unnecessary will be included into snapshots, 
8. fstab optimizations for ssds and compression, 
9. snapper confiured, 
10. Void Linux musl 64bit only, 
11. free software only with a lxd container for glibc and non free software to be done (however the kernel incude some blobs that are needed and it's difficult to remove it without knowing on whic hardware areyou installing it), 
12. s6 init instead of runit on the main system and on the container, 
13. the best of all programs (like chrony for ntp, ecc.), 
14. Desktop environments with fewer things and a terminal emulator,  
15. a large choice of software, 
16. flatpak on userspace to install necessary proprietary software (like firefox DRM).

Make sure to follow the instructions, if you want to modify it just do it! 

NOTE: the script is in early stage of development

## Getting started

Download a live iso of Void Linux musl from here https://voidlinux.org/download/
Burn the iso file into a usb
Boot Void Linux live
Download a tar file from here
Go to download, right click, open terminal here
tar -xvf gab-void-linux-installer.tar
cd gab-void-linux-installer
sudo su
chmod +x start.sh
bash start.sh


## Why
I was looking for the best Linux distribution and I haven't found it, than I have done it my own.


## Usage

Void Linux Handbook:
https://docs.voidlinux.org/

66 usage:
https://wiki.obarun.org/doku.php?id=66intro
https://web.obarun.org/software/66/latest/

Most used commands for 66 and this configuration:
66-intree #show all services running
ls /usr/lib/66/service #show services available (note that there are all services, not only what you have "installed")
ls /etc/sv/ #show services "installed"
66-enable <service-name> #enable service
66-disable <service-name> #disable service
66-start <service-name> #start a service
66-stop <service-name> #stop a service
66-inservice <service-name> #show the status of a service

void-66 reference:
https://github.com/mobinmob/void-66-services
Used to change init.


## Roadmap
I want to make this script for all.
Future implementation are included in the TODO.md.
Things that not will be changed are: encryption, btrfs, swapfile.


## Contributing
I'm open for contributors, pls make your own branch.

## Authors and acknowledgment
skea999 (skea999@cryptolab.net)
Osterol ()
Moze ()

Special thanks to:
mobinmob that helped me a lot
all void-66 contributors listed to relative project https://github.com/mobinmob/void-66-services
Void Linux creator and contributors https://voidlinux.org/
66 creator 

## License
ISC License:
Copyright (c) Year(s), Company or Person's Name

Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND ISC DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL ISC BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.


## Project status
The script work, all the missing things can be found in TODO.md

## Troubleshooting
If you have an AMD CPU you can inconter this bug:
https://gist.github.com/wmealing/2dd2b543c4d3cff6cab7
package suggestion
https://github.com/qrwteyrutiyoup/ryzen-stabilizator
I solved mine with:
https://wiki.archlinux.org/title/Dell_G5_SE_5505
adding amdgpu.runpm=0 to the kernel parameters

for steam flatpak also install liberation-fonts-ttf package
and 
flatpak override --user --filesystem=/path/to/your/other/games com.valvesoftware.Steam