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

	// 1. Cerchiamo il template obbligatorio
	// Proviamo in diversi percorsi comuni per essere sicuri di trovarlo
	templatePaths := []string{
		"variables/variables.yaml",
		"alpineInstaller/variables/variables.yaml",        // Se lanciato dalla root del progetto
		"../variables/variables.yaml",                     // Se lanciato da installer/utility/
		"../../variables/variables.yaml",                  // Se lanciato da installer/utility/makeVariables/
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
		log.Fatalf("[ERRORE FATALE] Impossibile trovare il template obbligatorio. Assicurati che 'alpineInstaller/variables/variables.yaml' esista.")
	}

	fmt.Printf("[INFO] Caricamento default dal template: %s\n", templatePath)

	// 2. Carichiamo i dati
	conf, err := utility.LoadConfig(templatePath)
	if err != nil {
		log.Fatalf("[ERRORE] Errore nel caricamento del template: %v", err)
	}

	// 3. Wizard (Default presi dal file YAML appena caricato)
	conf.User.Unattended = false
	utility.PromptForConfig(conf)

	// 4. Salvataggio
	if err := saveConfigStandalone(conf, *userFilePath); err != nil {
		log.Fatalf("[ERRORE] Salvataggio fallito: %v", err)
	}

	fmt.Printf("[SUCCESSO] Nuovo file generato: %s\n", *userFilePath)
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
