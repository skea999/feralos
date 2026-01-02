package main

import (
	"alpineInstaller/installer/utility"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func main() {
	userFilePath := flag.String("file", "variables.yaml", "File da generare/modificare")
	flag.Parse()

	// 1. Look up the mandatory template
	// We try several common paths to make sure we find it
	templatePaths := []string{
		"variables/variables.yaml",
		"alpineInstaller/variables/variables.yaml",        // If launched from the project root
		"../variables/variables.yaml",                     // If launched from installer/utility/
		"../../variables/variables.yaml",                  // If launched from installer/utility/makeVariables/
		"/usr/local/share/alpineInstaller/variables.yaml", // Percorso di sistema ipotetico
	}

	var templatePath string
	for _, p := range templatePaths {
		if _, err := os.Stat(p); err == nil {
			templatePath = p
			break
		}
	}

	if templatePath == "" {
		log.Fatalf("[FATAL ERROR] Cannot find the mandatory template. Make sure 'alpineInstaller/variables/variables.yaml' exists.")
	}

	fmt.Printf("[INFO] Caricamento default dal template: %s\n", templatePath)

	// 2. Carichiamo i dati
	conf, err := utility.LoadConfig(templatePath)
	if err != nil {
		log.Fatalf("[ERROR] Error loading the template: %v", err)
	}

	// 3. Wizard (Default presi dal file YAML appena caricato)
	conf.User.Unattended = false
	utility.PromptForConfig(conf)

	// 4. Salvataggio
	if err := saveConfigStandalone(conf, *userFilePath); err != nil {
		log.Fatalf("[ERROR] Save failed: %v", err)
	}

	fmt.Printf("[SUCCESS] New file generated: %s\n", *userFilePath)
}

func saveConfigStandalone(conf *utility.Config, filename string) error {
	dir := filepath.Dir(filename)
	if dir != "." {
		os.MkdirAll(dir, 0755)
	}
	yamlData := struct {
		Variables utility.UserVariables `yaml:"variables"`
	}{
		Variables: conf.User,
	}
	data, err := yaml.Marshal(&yamlData)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}
