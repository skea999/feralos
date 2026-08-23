package main

import (
	"alpineInstaller/utils/config"
	"alpineInstaller/utils/createConfig"
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

	// 1. Find the required template
	// Try common paths to ensure we find it
	templatePaths := []string{
		"src/variables/variables.yaml",            // If run from project root
		"variables/variables.yaml",                // If run from src/
		"../variables/variables.yaml",             // If run from src/utils/createConfig/
		"../../variables/variables.yaml",          // If run from src/utils/createConfig/createValues/
		"/usr/local/share/feralos/variables.yaml", // Hypothetical system path
	}

	var templatePath string
	for _, p := range templatePaths {
		if _, err := os.Stat(p); err == nil {
			templatePath = p
			break
		}
	}

	if templatePath == "" {
		log.Fatalf("[FATAL ERROR] Cannot find required template. Make sure 'alpineInstaller/variables/variables.yaml' exists.")
	}

	fmt.Printf("[INFO] Loading defaults from template: %s\n", templatePath)

	// 2. Load data (template only, no internal_rules needed)
	conf, err := config.LoadTemplate(templatePath)
	if err != nil {
		log.Fatalf("[ERROR] Error loading template: %v", err)
	}

	// 3. Wizard (Defaults taken from just loaded YAML file)
	conf.User.Unattended = false
	createConfig.PromptForConfig(conf)

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
	return os.WriteFile(filename, data, 0600)
}
