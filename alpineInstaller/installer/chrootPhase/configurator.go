package chrootPhase

import (
	"alpineInstaller/installer/utility"
	"fmt"
	"strings"
)

// Run esegue tutti i passaggi di configurazione finale all'interno dell'ambiente chroot Alpine.
func Run(conf *utility.Config) error {
	u := conf.User
	i := conf.Internal
	chroot := i.ChrootDir

	fmt.Println("[INFO] Starting ChrootPhase.Run...")

	// 1. Aggiornamento e Installazione Pacchetti
	// Usiamo u.AllPackages che contiene già Standard + Extra + pacchetti derivati dalle regole
	fmt.Println("[DEBUG] Updating repositories and installing all selected packages")
	if _, err := utility.RunChrootCommand(chroot, "apk update"); err != nil {
		return fmt.Errorf("apk update failed: %w", err)
	}

	pkgList := strings.Join(u.AllPackages, " ")
	if _, err := utility.RunChrootCommand(chroot, "apk add "+pkgList); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	// 2. Setup Utente e Password
	fmt.Printf("[DEBUG] Configuring users: root and %s\n", u.UserName)
	// Password di Root
	utility.RunChrootCommand(chroot, fmt.Sprintf("echo -e '%s\n%s' | passwd root", u.Password, u.Password))

	// Creazione utente senza gruppi predefiniti (solo il suo gruppo primario)
	utility.RunChrootCommand(chroot, "adduser -D "+u.UserName)
	utility.RunChrootCommand(chroot, fmt.Sprintf("echo -e '%s\n%s' | passwd %s", u.Password, u.Password, u.UserName))

	// Aggiunta ai gruppi DINAMICI (incluso wheel se presente nelle regole di doas/sudo)
	for _, group := range u.UserGroups {
		fmt.Printf("[DEBUG] Adding user %s to group %s\n", u.UserName, group)
		utility.RunChrootCommand(chroot, fmt.Sprintf("addgroup %s %s", u.UserName, group))
	}

	// 3. Timezone
	fmt.Printf("[DEBUG] Setting timezone to %s\n", u.Timezone)
	utility.RunChrootCommand(chroot, fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /etc/localtime", u.Timezone))

	// 4. Runlevels (OpenRC)
	// Questa mappa u.Runlevels contiene i default di sistema + quelli aggiunti dai pacchetti extra
	fmt.Println("[DEBUG] Configuring OpenRC runlevels")
	setupRunlevels(chroot, u.Runlevels)

	// 5. Finalizzazione Boot (GRUB e Initramfs)
	fmt.Println("[DEBUG] Generating Initramfs and installing GRUB bootloader")
	if err := finalizeBoot(chroot); err != nil {
		return fmt.Errorf("boot finalization failed: %w", err)
	}

	fmt.Println("[INFO] ChrootPhase.Run completed successfully.")
	return nil
}

func setupRunlevels(chroot string, levels map[string][]string) {
	for level, services := range levels {
		for _, svc := range services {
			// Esegue: rc-update add <servizio> <runlevel>
			utility.RunChrootCommand(chroot, fmt.Sprintf("rc-update add %s %s", svc, level))
		}
	}
}

func finalizeBoot(chroot string) error {
	// Installazione GRUB su entrambi i dischi EFI per ridondanza (RAID1)
	// Disco 1
	if _, err := utility.RunChrootCommand(chroot, "grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=Alpine1 --recheck --no-floppy --removable"); err != nil {
		return err
	}
	// Disco 2
	if _, err := utility.RunChrootCommand(chroot, "grub-install --target=x86_64-efi --efi-directory=/boot/efi2 --bootloader-id=Alpine2 --recheck --no-floppy --removable"); err != nil {
		return err
	}

	// Rigenerazione Initramfs (necessario per includere i moduli raid1/btrfs caricati in precedenza)
	// Cerchiamo la versione del kernel presente per mkinitfs
	kernelCmd := "ls /lib/modules/ | head -n1"
	kVer, _ := utility.RunChrootCommand(chroot, kernelCmd)
	kVer = strings.TrimSpace(kVer)

	if _, err := utility.RunChrootCommand(chroot, fmt.Sprintf("mkinitfs -c /etc/mkinitfs/mkinitfs.conf -b / %s", kVer)); err != nil {
		return err
	}

	// Generazione file di configurazione GRUB finale
	if _, err := utility.RunChrootCommand(chroot, "grub-mkconfig -o /boot/grub/grub.cfg"); err != nil {
		return err
	}

	return nil
}
