package main

import (
	"feralos/src/utils/config"
	"feralos/src/utils/createConfig"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func main() {
	userFilePath := flag.String("file", "variables.yaml", "File to generate/modify")
	flag.Parse()

	// 1. Find the required template. Single probe source:
	// config.FindTemplatePath (TemplateCandidatePaths in utils/config,
	// mirroring the internal_rules resolver) — the former locally
	// duplicated 5-path list is gone; createValues already imports
	// utils/config (no new coupling). The not-found message stays local:
	// only this CLI owns its failure wording.
	templatePath, err := config.FindTemplatePath()
	if err != nil {
		log.Fatalf("[FATAL ERROR] Cannot find required template. Make sure 'feralos/src/variables/variables.yaml' exists.")
	}

	fmt.Printf("[INFO] Loading defaults from template: %s\n", templatePath)

	// 2. Load data (template + internal_rules for wizard enum options)
	conf, err := config.LoadTemplate(templatePath)
	if err != nil {
		log.Fatalf("[ERROR] Error loading template: %v", err)
	}
	if err := config.LoadInternalRules(conf); err != nil {
		log.Fatalf("[ERROR] Error loading internal_rules.yaml: %v", err)
	}

	// 3. Wizard (Defaults taken from just loaded YAML file)
	conf.User.Unattended = false
	// Stdin close surfaces as an error; the same fatal-abort message the
	// former in-library exit path printed is printed HERE + exit 1 —
	// os.Exit stays in the CLI layer, byte-identical output and code.
	if err := createConfig.PromptForConfig(conf); err != nil {
		fmt.Println("\n[FATAL ERROR] input closed — aborting.")
		os.Exit(1)
	}

	// 4. Save
	if err := saveConfigStandalone(conf, *userFilePath); err != nil {
		log.Fatalf("[ERROR] Save failed: %v", err)
	}

	fmt.Printf("[SUCCESS] New file generated: %s\n", *userFilePath)
}

func saveConfigStandalone(conf *config.Config, filename string) error {
	dir := filepath.Dir(filename)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	yamlData := struct {
		Variables config.UserVariables `yaml:"variables"`
	}{
		Variables: conf.User,
	}
	data, err := yaml.Marshal(&yamlData)
	if err != nil {
		return err
	}
	// 0600, not 0755/etc: the config carries the user password + LUKS
	// passphrase in clear text (saveConfigStandalone is the wizard's save
	// path) — owner-only read/write. MkdirAll above deliberately keeps
	// 0755 (directories, not secret content).
	return os.WriteFile(filename, data, 0600)
}
