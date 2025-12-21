# Skea Installer

This is a simple script for installing Alpine Linux with a raid 1 and btrfs / with subvolumes.

This script is simply hackable, enjoy it.


## Prerequisites

At least 2 disks of 8gb each

## What the script will do: 

1. / in raid1 btrfs with a lot of subvolumes for all possible scenarios.
2. /boot in raid1.
3. /boot/efi in FAT32 and a mirror in /boot/efi2.
6. partition space will done by a formula.
7. 2 swap space for speed (one each disk), maximum swap for offset low hardware on selfhost environment.
8. zram for the same thing.
9. 2 partitions with no raid for putting things not needed there.
10. Incus installed but not started, it is better to take another 2 disks (or what disk you want), make a raid, mount them in /var/lib/incus, this way all incus configuration are on the other disks, similar to proxmox.

Make sure to follow the instructions, if you want to modify it just do it! 

## Getting started

Download a live iso Linux Mint.
Burn the iso file into a usb
Boot Linux Mint live
Download a tar file from here
Go to download, right click, open terminal here
tar -xvf gab-void-linux-installer.tar
cd gab-void-linux-installer
sudo su
chmod +x start.sh
bash start.sh


## Why
I was looking for the best Hypervisor, capable of compete with cloud providers but more secure and easier.


## Roadmap
It misses web-ui, encryption, secure boot and I want to make alpine to work replacing all possible software with memory safe one.


## Contributing
I'm open for contributors.

## Authors and acknowledgment
skea999 (skea999@cryptolab.net)
Moze ()

## License
ISC License:
Copyright (c) Year(s), Company or Person's Name

Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND ISC DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL ISC BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.


## Project status
The script works but it is the easy one, i will make a go installer to simplify things. 
NOTE: the script is in early stage of development
