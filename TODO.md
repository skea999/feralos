- [TODO](#todo)
  - [Essenziali](#essenziali)
  - [Opzionali](#opzionali)
  - [Ulteriori implementazioni](#ulteriori-implementazioni)
# TODO

## Essenziali
* [ ] implementare USE_XORG
* [x] logical volumes e volumes group
* [x] driver video
* [ ] confirm before formatting
* [ ] resolvconf.conf
* [ ] DE with minimal installation + terminal
* [ ] ulteriori tweak voidvalut
* [x] import the key for the repositories
* [x] command success checking (if $? = 1 then )
* [x] in s6 reboot and shutdown do not work (to fix with symlinks); even with the symlink shutdown does not work after it: sudo shutdown -p now — in theory switch-inutils does this automatically but not sure it works
* [x] mirrorselect (it is manual now)

----
##  Opzionali
* [x] switch s6 nel bootloader
* [ ] luks part not in the bootloader but in the initrfs

----
## Ulteriori implementazioni
* [ ] vedere truncate e chattr su /swapfile
* [ ] functions
* [x] s6 does not work because /etc/locale.conf is not done well (though with my fix it now works)
* [ ] language is it_IT.UTF-8 and not it (to make everything uppercase `echo ${name^^}` ) but it seems to work anyway
* [x] container
* [ ] initial dependency installation for the most common package managers (apt, pacman, dnf, maybe others)
* [ ] lista dipendenze
* [ ] convert to a working installation on any distribution (i.e. with the tar)
* [ ] colorare l'output
* [ ] dare un comando di output per controllo sulle cose fatte
* [x] make wayland work
* [ ] install only packages without meta packages (e.g. base system too)
* [ ] install all packages together with packagename&!packagevariable
* [ ] for this you need to identify the hardware
* [ ] color the terminal with the installed version
* [ ] custom kde
* [ ] custom sddm e lingua sddm
* [ ] test all DEs for minimal installation


* [ ] run post-install scripts automatically




* [ ] eliminazione dei file d'appoggio





