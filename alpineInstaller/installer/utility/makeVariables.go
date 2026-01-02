package utility

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PromptForConfig uses EXCLUSIVELY the values present in conf as defaults.
func PromptForConfig(conf *Config) {
	if conf.User.Unattended {
		fmt.Println("[INFO] Unattended mode active in the YAML file. Skipping wizard.")
		return
	}

	// Validation: If essential fields are empty (sign the YAML file was incomplete), we error out.
	validateOrDie(conf)

	reader := bufio.NewScanner(os.Stdin)
	fmt.Println("\n==============================================")
	fmt.Println("   ALPINE GO INSTALLER: CONFIGURATION WIZARD  ")
	fmt.Println("==============================================")
	fmt.Println("The values in [ ] come from the variables.yaml file.")

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
	pkgInput := askQuestion(reader, "Extra Packages", currentPkgs)
	conf.User.ExtraPackages = strings.Fields(pkgInput)
}

func validateOrDie(conf *Config) {
	// If the hostname is empty, the YAML file was malformed or missing
	if conf.User.Hostname == "" || conf.User.UserName == "" {
		fmt.Println("[CRITICAL ERROR] The variables.yaml file is missing required fields.")
		fmt.Println("Make sure the template file is present and populated.")
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
