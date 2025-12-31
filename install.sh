#!/bin/bash

# Install Go dependencies
sudo apt-get update
sudo apt-get install -y golang

# Run main.go
cd alpine_installer
sudo go run main.go
