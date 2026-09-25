package createConfig

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"alpineInstaller/utils/config"

	"golang.org/x/term"
)

// PromptForConfig runs the interactive configuration wizard.
//
// Flow (confirmed spec):
//  1. Every user option is asked, with defaults from the YAML template.
//  2. Each answer is validated immediately — invalid input re-asks the field.
//  3. A summary is printed at the end: "yes" proceeds, "no" restarts the
//     whole loop with the previous answers as new defaults.
//
// Skips entirely when unattended is set in the YAML file.
func PromptForConfig(conf *config.Config) {
	if conf.User.Unattended {
		fmt.Println("[INFO] Unattended mode active in YAML file. Skipping wizard.")
		return
	}

	reader := bufio.NewScanner(os.Stdin)
	reader.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	fmt.Println("\n==============================================")
	fmt.Println("        FERALOS INSTALLER: CONFIGURATION      ")
	fmt.Println("==============================================")
	fmt.Println("Press Enter to keep the value shown in [ ].")

	for {
		askAll(reader, conf)
		printSummary(conf)
		if askConfirm(reader) {
			return
		}
		fmt.Println("\n[INFO] Restarting input — your previous answers are now the defaults.")
	}
}

// askAll asks every user-variable field in order. Layout comes first so the
// disk count can be validated against it immediately.
func askAll(reader *bufio.Scanner, conf *config.Config) {
	u := &conf.User
	layoutOptions := enumOptions(conf.Internal.DiskLayouts)
	deOptions := append(enumOptions(conf.Internal.DesktopEnvironments), "none")
	videoOptions := append(enumOptions(conf.Internal.VideoDrivers), "none")

	fmt.Println("\n--- General ---")
	u.Unattended = askBool(reader, "Unattended (installer skips wizard)", u.Unattended)

	fmt.Println("\n--- Layout ---")
	u.DiskLayout = askEnum(reader, "Disk Layout", u.DiskLayout, layoutOptions,
		func(s string) error { return config.ValidateEnum("diskLayout", s, layoutOptions) })
	u.Disks = askDisks(reader, u.DiskLayout, u.Disks)

	fmt.Println("\n--- User and Security ---")
	u.UserName = askValid(reader, "Username", u.UserName, nil, config.ValidateUserName)
	fmt.Println("(password: minimum " + fmt.Sprint(config.MinPasswordLen) + " characters)")
	u.Password = askSecret(reader, "User Password", false, config.ValidatePassword)
	fmt.Println("(empty = reuse the user password)")
	u.LuksPassword = askSecret(reader, "LUKS Passphrase", true, config.ValidateLuksPassword)
	if u.LuksPassword == "" {
		u.LuksPassword = u.Password
	}

	fmt.Println("\n--- System Identity ---")
	u.Hostname = askValid(reader, "Hostname", u.Hostname, nil, config.ValidateHostname)
	u.Timezone = askValid(reader, "Timezone", u.Timezone, nil, config.ValidateTimezone)
	u.Locale = askValid(reader, "Locale", u.Locale, nil, config.ValidateLocale)
	u.Keymap = askValid(reader, "Keymap", u.Keymap, nil, config.ValidateKeymap)

	fmt.Println("\n--- Repository ---")
	u.Mirror = askValid(reader, "Mirror URL", u.Mirror, nil, config.ValidateMirror)
	u.Arch = askValid(reader, "Architecture", u.Arch, nil, config.ValidateArch)
	u.Branch = askValid(reader, "Branch", u.Branch, nil, config.ValidateBranch)

	fmt.Println("\n--- Network and SSH ---")
	u.DefaultNetworkInterface = askValid(reader, "Default Network Interface", u.DefaultNetworkInterface, nil, config.ValidateInterface)
	u.SshAuthorizedKey = askValid(reader, "SSH Authorized Key (empty = none)", u.SshAuthorizedKey, nil, config.ValidateSSHKey)

	fmt.Println("\n--- Desktop ---")
	u.DesktopEnvironment = askEnum(reader, "Desktop Environment", u.DesktopEnvironment, deOptions,
		func(s string) error { return config.ValidateEnum("desktopEnvironment", s, deOptions) })
	u.VideoDriver = askEnum(reader, "Video Driver", u.VideoDriver, videoOptions,
		func(s string) error { return config.ValidateEnum("videoDriver", s, videoOptions) })

	fmt.Println("\n--- Feature Toggles ---")
	u.EnableSecureboot = askBool(reader, "Enable Secure Boot", u.EnableSecureboot)
	u.EnableTPM = askBool(reader, "Enable TPM (LUKS auto-unlock)", u.EnableTPM)
	u.EnableHardening = askBool(reader, "Enable Hardening", u.EnableHardening)
	u.EnableZswap = askBool(reader, "Enable Zswap", u.EnableZswap)
	u.EnableNix = askBool(reader, "Enable Nix", u.EnableNix)
	u.EnableSwap = askBool(reader, "Enable Swap", u.EnableSwap)
	u.SwapSize = askValid(reader, "Swap Size (auto = 2x RAM, or e.g. 8G)", u.SwapSize, nil, config.ValidateSwapSize)
	u.EnableDebug = askBool(reader, "Enable Debug", u.EnableDebug)
	u.EnablePodman = askBool(reader, "Enable Podman", u.EnablePodman)
	u.EnableWebmin = askBool(reader, "Enable Webmin (requires Podman)", u.EnableWebmin)
	u.WebminListenAll = askBool(reader, "Webmin Listen on All Interfaces", u.WebminListenAll)
	u.EnableDinit = askBool(reader, "Enable Dinit (init system, false = OpenRC)", u.EnableDinit)

	fmt.Println("\n--- Extra Packages ---")
	u.ExtraPackages = askExtraPackages(reader, u.ExtraPackages)
}

// askValid prompts with a default; empty input keeps the default (if it
// validates). Invalid input prints the validation error and re-asks.
func askValid(reader *bufio.Scanner, label, current string, options []string, validate func(string) error) string {
	for {
		if len(options) > 0 {
			fmt.Printf("%s (options: %s) [%s]: ", label, strings.Join(options, ", "), current)
		} else {
			fmt.Printf("%s [%s]: ", label, current)
		}
		input := readLine(reader)
		if input == "" {
			input = current
		}
		if validate == nil {
			return input
		}
		if err := validate(input); err != nil {
			fmt.Printf("[ERROR] %v — try again.\n", err)
			continue
		}
		return input
	}
}

// askEnum wraps askValid with the option list shown on every prompt.
func askEnum(reader *bufio.Scanner, label, current string, options []string, validate func(string) error) string {
	return askValid(reader, label, current, options, validate)
}

// askDisks reads a comma-separated disk list and validates both paths and
// the count against the chosen layout. Re-asks until valid.
func askDisks(reader *bufio.Scanner, layout string, current []string) []string {
	def := strings.Join(current, ",")
	for {
		input := askValid(reader, "Target Disks (comma separated)", def, nil, nil)
		disks := make([]string, 0, 2)
		for _, d := range strings.Split(input, ",") {
			if d = strings.TrimSpace(d); d != "" {
				disks = append(disks, d)
			}
		}
		if err := config.ValidateDisks(disks); err != nil {
			fmt.Printf("[ERROR] %v — try again.\n", err)
			continue
		}
		if err := config.ValidateDisksForLayout(layout, disks); err != nil {
			fmt.Printf("[ERROR] %v — try again.\n", err)
			continue
		}
		return disks
	}
}

// askSecret reads a password with masked terminal input; falls back to
// plain line reading when stdin is not a terminal (pipes, automation).
// Empty input is only accepted when allowEmpty is set (LUKS fallback).
func askSecret(reader *bufio.Scanner, label string, allowEmpty bool, validate func(string) error) string {
	for {
		fmt.Printf("%s: ", label)
		var s string
		if term.IsTerminal(int(os.Stdin.Fd())) {
			pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				fmt.Println("[ERROR] failed to read input — try again.")
				continue
			}
			s = string(pwd)
		} else {
			s = readLine(reader)
		}
		if s == "" && allowEmpty {
			return ""
		}
		if err := validate(s); err != nil {
			fmt.Printf("[ERROR] %v — try again.\n", err)
			continue
		}
		return s
	}
}

// askBool prompts for true/false. Empty input keeps the default.
func askBool(reader *bufio.Scanner, label string, current bool) bool {
	def := "false"
	if current {
		def = "true"
	}
	for {
		fmt.Printf("%s [true/false] [%s]: ", label, def)
		input := readLine(reader)
		if input == "" {
			return current
		}
		switch strings.ToLower(input) {
		case "true", "yes", "y", "1":
			return true
		case "false", "no", "n", "0":
			return false
		default:
			fmt.Println("[ERROR] please answer true or false — try again.")
		}
	}
}

// askExtraPackages reads a space-separated package list and validates each
// name. Empty input keeps the default.
func askExtraPackages(reader *bufio.Scanner, current []string) []string {
	def := strings.Join(current, " ")
	for {
		input := askValid(reader, "Extra Packages (space separated)", def, nil, nil)
		pkgs := strings.Fields(input)
		if err := config.ValidateExtraPackages(pkgs); err != nil {
			fmt.Printf("[ERROR] %v — try again.\n", err)
			continue
		}
		return pkgs
	}
}

// askConfirm waits for "yes" (proceed) or "no" (restart input).
func askConfirm(reader *bufio.Scanner) bool {
	for {
		fmt.Printf("\nProceed with this configuration? Type \"yes\" to proceed, \"no\" to re-enter: ")
		switch strings.ToLower(readLine(reader)) {
		case "yes", "y":
			return true
		case "no", "n":
			return false
		default:
			fmt.Println("[ERROR] please type \"yes\" or \"no\".")
		}
	}
}

// printSummary shows every value; passwords are masked.
func printSummary(conf *config.Config) {
	u := &conf.User
	fmt.Println("\n================ CONFIGURATION SUMMARY ================")
	fmt.Printf("  diskLayout:             %s\n", u.DiskLayout)
	fmt.Printf("  disks:                  %s\n", strings.Join(u.Disks, ", "))
	fmt.Printf("  userName:               %s\n", u.UserName)
	fmt.Printf("  password:               %s\n", maskDefault(u.Password))
	fmt.Printf("  luksPassword:           %s\n", maskDefault(u.LuksPassword))
	fmt.Printf("  hostname:               %s\n", u.Hostname)
	fmt.Printf("  timezone:               %s\n", u.Timezone)
	fmt.Printf("  locale:                 %s\n", u.Locale)
	fmt.Printf("  keymap:                 %s\n", u.Keymap)
	fmt.Printf("  mirror:                 %s\n", u.Mirror)
	fmt.Printf("  arch:                   %s\n", u.Arch)
	fmt.Printf("  branch:                 %s\n", u.Branch)
	fmt.Printf("  defaultNetworkIface:    %s\n", u.DefaultNetworkInterface)
	fmt.Printf("  sshAuthorizedKey:       %s\n", truncDefault(u.SshAuthorizedKey))
	fmt.Printf("  desktopEnvironment:     %s\n", u.DesktopEnvironment)
	fmt.Printf("  videoDriver:            %s\n", u.VideoDriver)
	fmt.Printf("  enableSecureboot:       %v\n", u.EnableSecureboot)
	fmt.Printf("  enableTPM:              %v\n", u.EnableTPM)
	fmt.Printf("  enableHardening:        %v\n", u.EnableHardening)
	fmt.Printf("  enableZswap:            %v\n", u.EnableZswap)
	fmt.Printf("  enableNix:              %v\n", u.EnableNix)
	fmt.Printf("  enableSwap:             %v\n", u.EnableSwap)
	fmt.Printf("  swapSize:               %s\n", u.SwapSize)
	fmt.Printf("  enableDebug:            %v\n", u.EnableDebug)
	fmt.Printf("  enablePodman:           %v\n", u.EnablePodman)
	fmt.Printf("  enableWebmin:           %v\n", u.EnableWebmin)
	fmt.Printf("  webminListenAll:        %v\n", u.WebminListenAll)
	fmt.Printf("  enableDinit:            %v\n", u.EnableDinit)
	fmt.Printf("  extraPackages:          %s\n", strings.Join(u.ExtraPackages, " "))
	fmt.Println("=======================================================")
}

// enumOptions returns the sorted map keys for enum prompts.
func enumOptions[T any](m map[string]T) []string {
	opts := make([]string, 0, len(m))
	for k := range m {
		opts = append(opts, k)
	}
	return opts
}

func maskDefault(s string) string {
	if s == "" {
		return "(empty)"
	}
	return strings.Repeat("*", utf8.RuneCountInString(s))
}

func truncDefault(s string) string {
	if len(s) > 40 {
		return s[:37] + "..."
	}
	return s
}

// readLine reads one trimmed line; EOF/stdin close aborts instead of
// looping forever on a closed terminal.
func readLine(reader *bufio.Scanner) string {
	if reader.Scan() {
		return strings.TrimSpace(reader.Text())
	}
	fmt.Println("\n[FATAL ERROR] input closed — aborting.")
	os.Exit(1)
	return ""
}
