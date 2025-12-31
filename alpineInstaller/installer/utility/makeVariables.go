package utility

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PromptForConfig usa ESCLUSIVAMENTE i valori presenti in conf come default.
func PromptForConfig(conf *Config) {
	if conf.User.Unattended {
		fmt.Println("[INFO] Modalità Unattended attiva nel file YAML. Salto il wizard.")
		return
	}

	// Validazione: Se i campi essenziali sono vuoti (segno che il file YAML non era completo), diamo errore.
	validateOrDie(conf)

	reader := bufio.NewScanner(os.Stdin)
	fmt.Println("\n==============================================")
	fmt.Println("   ALPINE GO INSTALLER: CONFIGURATION WIZARD  ")
	fmt.Println("==============================================")
	fmt.Println("I valori tra [ ] provengono dal file variables.yaml.")

	// Unattended
	uStr := "false"
	if conf.User.Unattended {
		uStr = "true"
	}
	conf.User.Unattended = strings.ToLower(askQuestion(reader, "Abilitare Unattended?", uStr)) == "true"

	// Disks
	currentDisks := strings.Join(conf.User.Disks, ",")
	diskInput := askQuestion(reader, "Dischi Target (separati da virgola)", currentDisks)
	conf.User.Disks = strings.Split(diskInput, ",")

	// Identity & Auth
	conf.User.Hostname = askQuestion(reader, "Hostname", conf.User.Hostname)
	conf.User.UserName = askQuestion(reader, "Username", conf.User.UserName)
	conf.User.Password = askQuestion(reader, "Password", conf.User.Password)

	// Localization
	conf.User.Timezone = askQuestion(reader, "Timezone", conf.User.Timezone)
	conf.User.Locale = askQuestion(reader, "Locale", conf.User.Locale)

	// Repo
	conf.User.Mirror = askQuestion(reader, "Mirror URL", conf.User.Mirror)
	conf.User.Arch = askQuestion(reader, "Architettura", conf.User.Arch)
	conf.User.Branch = askQuestion(reader, "Branch", conf.User.Branch)

	// Layout
	conf.User.DiskLayout = askQuestion(reader, "Disk Layout", conf.User.DiskLayout)
	conf.User.DesktopEnvironment = askQuestion(reader, "Ambiente Desktop", conf.User.DesktopEnvironment)
	conf.User.VideoDriver = askQuestion(reader, "Driver Video", conf.User.VideoDriver)

	// Extra Packages
	currentPkgs := strings.Join(conf.User.ExtraPackages, " ")
	pkgInput := askQuestion(reader, "Pacchetti Extra", currentPkgs)
	conf.User.ExtraPackages = strings.Fields(pkgInput)
}

func validateOrDie(conf *Config) {
	// Se l'hostname è vuoto, significa che il file YAML era malformato o mancante
	if conf.User.Hostname == "" || conf.User.UserName == "" {
		fmt.Println("[ERRORE CRITICO] Il file variables.yaml non contiene i campi necessari.")
		fmt.Println("Assicurati che il file template sia presente e popolato.")
		os.Exit(1)
	}
}

func askQuestion(reader *bufio.Scanner, label, defaultValue string) string {
	fmt.Printf("%s [%s]: ", label, defaultValue)
	if reader.Scan() {
		input := strings.TrimSpace(reader.Text())
		if input == "" {
			return defaultValue
		}
		return input
	}
	return defaultValue
}
