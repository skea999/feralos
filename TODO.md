- [TODO](#todo)
  - [Essenziali](#essenziali)
  - [Opzionali](#opzionali)
  - [Ulteriori implementazioni](#ulteriori-implementazioni)
# TODO

## Essenziali
* [ ] implementare USE_XORG
* [x] logical volumes e volumes group
* [x] driver video
* [ ] conferma prima di formattare
* [ ] resolvconf.conf
* [ ] DE con installazione minima + terminale
* [ ] ulteriori tweak voidvalut
* [x] importare la chiave per i repository
* [x] controllo sulla riuscita dei comandi (if $? = 1 then )
* [x] in s6 non funziona riavvio e spegnimento (da aggiustare con i symlink) neanche con il symlink funziona per spegnere la macchina dopo il symlink: sudo shutdown -p now in teoria switch-inutils fa questo in automatica ma non so se funziona
* [x] mirrorselect (it is manual now)

----
##  Opzionali
* [x] switch s6 nel bootloader
* [ ] la parte di luks non nel bootloader ma negli initrfs

----
## Ulteriori implementazioni
* [ ] vedere truncate e chattr su /swapfile
* [ ] funzioni
* [x] s6 non funziona perché /etc/locale.conf non è fatto bene (anche se come ho corretto ora funziona)
* [ ] language is it_IT.UTF-8 e non it (per mettere tutto maiuscolo `echo ${name^^}` ) ma sembra funzionare anche così
* [x] container
* [ ] installazione iniziale dipendenze per i package manager più diffusi (apt, pacman, dnf, forse altri)
* [ ] lista dipendenze
* [ ] convertire a installazione funzionante su qualsiasi distribuzione (quindi con il tar)
* [ ] colorare l'output
* [ ] dare un comando di output per controllo sulle cose fatte
* [x] far funzionare wayland
* [ ] installare solo i pacchetti senza meta pachetti (anche base system per esempio)
* [ ] installare i pacchetti tutti insieme con nomepacchetto&!variabilepacchetto
* [ ] per fare questo bisogna individuare l'hardware
* [ ] colora il terminale della versione installata
* [ ] custom kde
* [ ] custom sddm e lingua sddm
* [ ] testare tutti i DE per l'installazione minimale


* [ ] esecuzione degli script post install in automatico




* [ ] eliminazione dei file d'appoggio





