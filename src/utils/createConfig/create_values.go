package createConfig

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"feralos/src/utils/config"

	"golang.org/x/term"
)

// errInputClosed marks stdin EOF/close. It is the only error the wizard
// chain propagates: readLine produces it, every intermediate ask* function
// returns it unchanged, and the top-level CLI caller (PromptForConfig)
// turns it into the same fatal-abort message the library used to print
// itself — CLI behavior stays byte-identical
// ("\n[FATAL ERROR] input closed — aborting." + exit code 1).
var errInputClosed = errors.New("input closed")

// PromptForConfig runs the interactive configuration wizard.
//
// Flow (confirmed spec):
//  1. Every user option is asked, with defaults from the YAML template.
//  2. Each answer is validated immediately — invalid input re-asks the field.
//  3. A summary is printed at the end: "yes" proceeds, "no" restarts the
//     whole loop with the previous answers as new defaults.
//
// Skips entirely when unattended is set in the YAML file.
// Stdin EOF/close prints the fatal-abort message and exits 1 there (this
// is the ONLY os.Exit in the library — the ask* chain behind it stays
// exit-free and scanner-testable).
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
		if err := askAll(reader, conf); err != nil {
			fmt.Println("\n[FATAL ERROR] input closed — aborting.")
			os.Exit(1)
		}
		printSummary(conf)
		proceed, err := askConfirm(reader)
		if err != nil {
			fmt.Println("\n[FATAL ERROR] input closed — aborting.")
			os.Exit(1)
		}
		if proceed {
			return
		}
		fmt.Println("\n[INFO] Restarting input — your previous answers are now the defaults.")
	}
}

// askAll asks every user-variable field in order. Layout comes first so the
// disk count can be validated against it immediately. Errors from the
// read chain (stdin EOF/close) abort the wizard via the top-level caller.
func askAll(reader *bufio.Scanner, conf *config.Config) error {
	u := &conf.User
	layoutOptions := enumOptions(conf.Internal.DiskLayouts)
	deOptions := append(enumOptions(conf.Internal.DesktopEnvironments), "none")
	videoOptions := append(enumOptions(conf.Internal.VideoDrivers), "none")

	fmt.Println("\n--- General ---")
	unattended, err := askBool(reader, "Unattended (installer skips wizard)", u.Unattended)
	if err != nil {
		return err
	}
	u.Unattended = unattended

	fmt.Println("\n--- Layout ---")
	diskLayout, err := askEnum(reader, "Disk Layout", u.DiskLayout, layoutOptions,
		func(s string) error { return config.ValidateEnum("diskLayout", s, layoutOptions) })
	if err != nil {
		return err
	}
	u.DiskLayout = diskLayout
	disks, err := askDisks(reader, u.DiskLayout, u.Disks)
	if err != nil {
		return err
	}
	u.Disks = disks

	fmt.Println("\n--- User and Security ---")
	userName, err := askValid(reader, "Username", u.UserName, nil, config.ValidateUserName)
	if err != nil {
		return err
	}
	u.UserName = userName
	fmt.Println("(password: minimum " + fmt.Sprint(config.MinPasswordLen) + " characters)")
	password, err := askSecret(reader, "User Password", false, config.ValidatePassword)
	if err != nil {
		return err
	}
	u.Password = password
	fmt.Println("(empty = reuse the user password)")
	luksPassword, err := askSecret(reader, "LUKS Passphrase", true, config.ValidateLuksPassword)
	if err != nil {
		return err
	}
	u.LuksPassword = luksPassword
	if u.LuksPassword == "" {
		u.LuksPassword = u.Password
	}

	fmt.Println("\n--- System Identity ---")
	hostname, err := askValid(reader, "Hostname", u.Hostname, nil, config.ValidateHostname)
	if err != nil {
		return err
	}
	u.Hostname = hostname
	timezone, err := askValid(reader, "Timezone", u.Timezone, nil, config.ValidateTimezone)
	if err != nil {
		return err
	}
	u.Timezone = timezone
	locale, err := askValid(reader, "Locale", u.Locale, nil, config.ValidateLocale)
	if err != nil {
		return err
	}
	u.Locale = locale
	keymap, err := askValid(reader, "Keymap", u.Keymap, nil, config.ValidateKeymap)
	if err != nil {
		return err
	}
	u.Keymap = keymap

	fmt.Println("\n--- Repository ---")
	mirror, err := askValid(reader, "Mirror URL", u.Mirror, nil, config.ValidateMirror)
	if err != nil {
		return err
	}
	u.Mirror = mirror
	arch, err := askValid(reader, "Architecture", u.Arch, nil, config.ValidateArch)
	if err != nil {
		return err
	}
	u.Arch = arch
	branch, err := askValid(reader, "Branch", u.Branch, nil, config.ValidateBranch)
	if err != nil {
		return err
	}
	u.Branch = branch
	edgeRepos, err := askBool(reader, "Enable Edge Repositories (@edge tag, for pkg@edge extras)", u.EnableEdgeRepos)
	if err != nil {
		return err
	}
	u.EnableEdgeRepos = edgeRepos

	fmt.Println("\n--- Network and SSH ---")
	iface, err := askValid(reader, "Default Network Interface", u.DefaultNetworkInterface, nil, config.ValidateInterface)
	if err != nil {
		return err
	}
	u.DefaultNetworkInterface = iface
	sshKey, err := askValid(reader, "SSH Authorized Key (empty = none)", u.SshAuthorizedKey, nil, config.ValidateSSHKey)
	if err != nil {
		return err
	}
	u.SshAuthorizedKey = sshKey

	fmt.Println("\n--- Desktop ---")
	de, err := askEnum(reader, "Desktop Environment", u.DesktopEnvironment, deOptions,
		func(s string) error { return config.ValidateEnum("desktopEnvironment", s, deOptions) })
	if err != nil {
		return err
	}
	u.DesktopEnvironment = de
	videoDriver, err := askEnum(reader, "Video Driver", u.VideoDriver, videoOptions,
		func(s string) error { return config.ValidateEnum("videoDriver", s, videoOptions) })
	if err != nil {
		return err
	}
	u.VideoDriver = videoDriver

	fmt.Println("\n--- Feature Toggles ---")
	secureboot, err := askBool(reader, "Enable Secure Boot", u.EnableSecureboot)
	if err != nil {
		return err
	}
	u.EnableSecureboot = secureboot
	tpm, err := askBool(reader, "Enable TPM (LUKS auto-unlock)", u.EnableTPM)
	if err != nil {
		return err
	}
	u.EnableTPM = tpm
	hardening, err := askBool(reader, "Enable Hardening", u.EnableHardening)
	if err != nil {
		return err
	}
	u.EnableHardening = hardening
	zswap, err := askBool(reader, "Enable Zswap", u.EnableZswap)
	if err != nil {
		return err
	}
	u.EnableZswap = zswap
	nix, err := askBool(reader, "Enable Nix", u.EnableNix)
	if err != nil {
		return err
	}
	u.EnableNix = nix
	enableSwap, err := askBool(reader, "Enable Swap", u.EnableSwap)
	if err != nil {
		return err
	}
	u.EnableSwap = enableSwap
	swapSize, err := askValid(reader, "Swap Size (auto = 2x RAM, or e.g. 8G)", u.SwapSize, nil, config.ValidateSwapSize)
	if err != nil {
		return err
	}
	u.SwapSize = swapSize
	debug, err := askBool(reader, "Enable Debug", u.EnableDebug)
	if err != nil {
		return err
	}
	u.EnableDebug = debug
	podman, err := askBool(reader, "Enable Podman", u.EnablePodman)
	if err != nil {
		return err
	}
	u.EnablePodman = podman
	webmin, err := askBool(reader, "Enable Webmin (requires Podman)", u.EnableWebmin)
	if err != nil {
		return err
	}
	u.EnableWebmin = webmin
	listenAll, err := askBool(reader, "Webmin Listen on All Interfaces", u.WebminListenAll)
	if err != nil {
		return err
	}
	u.WebminListenAll = listenAll
	dinit, err := askBool(reader, "Enable Dinit (init system, false = OpenRC)", u.EnableDinit)
	if err != nil {
		return err
	}
	u.EnableDinit = dinit

	fmt.Println("\n--- Extra Packages ---")
	extraPackages, err := askExtraPackages(reader, u.ExtraPackages)
	if err != nil {
		return err
	}
	u.ExtraPackages = extraPackages
	return nil
}

// askValid prompts with a default; empty input keeps the default (if it
// validates). Invalid input prints the validation error and re-asks.
func askValid(reader *bufio.Scanner, label, current string, options []string, validate func(string) error) (string, error) {
	for {
		if len(options) > 0 {
			fmt.Printf("%s (options: %s) [%s]: ", label, strings.Join(options, ", "), current)
		} else {
			fmt.Printf("%s [%s]: ", label, current)
		}
		input, err := readLine(reader)
		if err != nil {
			return "", err
		}
		if input == "" {
			input = current
		}
		if validate == nil {
			return input, nil
		}
		if verr := validate(input); verr != nil {
			fmt.Printf("[ERROR] %v — try again.\n", verr)
			continue
		}
		return input, nil
	}
}

// askEnum wraps askValid with the option list shown on every prompt.
func askEnum(reader *bufio.Scanner, label, current string, options []string, validate func(string) error) (string, error) {
	return askValid(reader, label, current, options, validate)
}

// askDisks reads a comma-separated disk list and validates both paths and
// the count against the chosen layout. Re-asks until valid.
func askDisks(reader *bufio.Scanner, layout string, current []string) ([]string, error) {
	def := strings.Join(current, ",")
	for {
		input, err := askValid(reader, "Target Disks (comma separated)", def, nil, nil)
		if err != nil {
			return nil, err
		}
		disks := make([]string, 0, 2)
		for _, d := range strings.Split(input, ",") {
			if d = strings.TrimSpace(d); d != "" {
				disks = append(disks, d)
			}
		}
		if verr := config.ValidateDisks(disks); verr != nil {
			fmt.Printf("[ERROR] %v — try again.\n", verr)
			continue
		}
		if verr := config.ValidateDisksForLayout(layout, disks); verr != nil {
			fmt.Printf("[ERROR] %v — try again.\n", verr)
			continue
		}
		return disks, nil
	}
}

// askSecret reads a password TWICE (entry + confirmation) with masked
// terminal input; the two entries must match or BOTH are re-asked.
// Falls back to plain line reading when stdin is not a terminal (pipes,
// automation — confirmation still enforced). Empty input is only accepted
// when allowEmpty is set (LUKS fallback: empty = reuse the user password)
// and skips confirmation (nothing to mistype). Terminal read errors retry
// with the usual message; stdin EOF/close (errInputClosed) propagates so
// the top-level caller can abort.
func askSecret(reader *bufio.Scanner, label string, allowEmpty bool, validate func(string) error) (string, error) {
	readOnce := func() (string, error) {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return "", err
			}
			return string(pwd), nil
		}
		return readLine(reader)
	}
	for {
		fmt.Printf("%s: ", label)
		s, err := readOnce()
		if err != nil {
			if errors.Is(err, errInputClosed) {
				return "", err
			}
			fmt.Println("[ERROR] failed to read input — try again.")
			continue
		}
		if s == "" && allowEmpty {
			return "", nil
		}
		if verr := validate(s); verr != nil {
			fmt.Printf("[ERROR] %v — try again.\n", verr)
			continue
		}
		fmt.Printf("Confirm %s: ", label)
		c, err := readOnce()
		if err != nil {
			if errors.Is(err, errInputClosed) {
				return "", err
			}
			fmt.Println("[ERROR] failed to read input — try again.")
			continue
		}
		if c != s {
			fmt.Println("[ERROR] passwords do not match — try again.")
			continue
		}
		return s, nil
	}
}

// askBool prompts for true/false. Empty input keeps the default.
func askBool(reader *bufio.Scanner, label string, current bool) (bool, error) {
	def := "false"
	if current {
		def = "true"
	}
	for {
		fmt.Printf("%s [true/false] [%s]: ", label, def)
		input, err := readLine(reader)
		if err != nil {
			return false, err
		}
		if input == "" {
			return current, nil
		}
		switch strings.ToLower(input) {
		case "true", "yes", "y", "1":
			return true, nil
		case "false", "no", "n", "0":
			return false, nil
		default:
			fmt.Println("[ERROR] please answer true or false — try again.")
		}
	}
}

// askExtraPackages reads a space-separated package list and validates each
// name. Empty input keeps the default.
func askExtraPackages(reader *bufio.Scanner, current []string) ([]string, error) {
	def := strings.Join(current, " ")
	for {
		input, err := askValid(reader, "Extra Packages (space separated)", def, nil, nil)
		if err != nil {
			return nil, err
		}
		pkgs := strings.Fields(input)
		if verr := config.ValidateExtraPackages(pkgs); verr != nil {
			fmt.Printf("[ERROR] %v — try again.\n", verr)
			continue
		}
		return pkgs, nil
	}
}

// askConfirm waits for "yes" (proceed) or "no" (restart input).
func askConfirm(reader *bufio.Scanner) (bool, error) {
	for {
		fmt.Printf("\nProceed with this configuration? Type \"yes\" to proceed, \"no\" to re-enter: ")
		input, err := readLine(reader)
		if err != nil {
			return false, err
		}
		switch strings.ToLower(input) {
		case "yes", "y":
			return true, nil
		case "no", "n":
			return false, nil
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
	fmt.Printf("  enableEdgeRepos:        %v\n", u.EnableEdgeRepos)
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

// enumOptions returns the map keys for enum prompts.
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

// readLine reads one trimmed line; EOF/stdin close surfaces as
// errInputClosed instead of aborting, so the wizard chain stays testable
// with strings.Reader-backed scanners.
func readLine(reader *bufio.Scanner) (string, error) {
	if reader.Scan() {
		return strings.TrimSpace(reader.Text()), nil
	}
	return "", errInputClosed
}
