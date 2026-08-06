## first case study for memory safe software

# alternatives
 memory safe: micro incus-feature incus-feature-vm incus-feature-client incus-feature-utils

 sudo, doas = sudo-rs (Rust, memory-safe replacement)

 musl-locales =
 lang =
 icu4x https://github.com/unicode-org/icu4x not packaged yet
 sys-locale fluent-rs not packaged
 an alternative is to not install it at all and rely on musl that is c but is not another thing to install but it is valid only for english speakers

 tzdata =
 contains data but the parser is musl and therefore is C. alternatives are Chrono Tz-rs ICU4X

 kbd-bkeymaps =
 contains data 
 uutils will change the parser from busybox (C) to uutils (rust)

 zstd =
 we could disable compression on btrfs and initramfs but i think that is a bad idea, is like giving up a usefull feature and this could also broke something

 chrony =
 ntpd-rs in testing

 font-terminus =
 contains data, uutils can replace the C parser

 qemu-img, qemu-system-x86_64, qemu-modules =
 those are for incus to work with VM but there is 
 cloud-hypervisor 
 that can replace those. unfortunately it is in testing now.

 busybox =
 uutils

non memorysafe remaining without alternatives
 mdadm grub-efi efibootmgr linux-lts btrfs-progs cronie mesa mesa-utils mesa-dri-gallium mesa-va-gallium mesa-vdpau-gallium xf86-video-amdgpu vulkan-loader vulkan-tools dbus musl
