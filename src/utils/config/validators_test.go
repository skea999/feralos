package config

import (
	"strings"
	"testing"
)

func TestValidateDisks(t *testing.T) {
	tests := []struct {
		name    string
		disks   []string
		wantErr bool
	}{
		{"valid sata", []string{"/dev/sda", "/dev/sdb"}, false},
		{"valid nvme", []string{"/dev/nvme0n1"}, false},
		{"valid virtio", []string{"/dev/vda"}, false},
		{"valid mmcblk", []string{"/dev/mmcblk0"}, false},
		{"empty list", nil, false},
		{"partition not disk", []string{"/dev/sda1"}, true},
		{"not absolute", []string{"sda"}, true},
		{"random path", []string{"/dev/disk/by-id/xxx"}, true},
		{"shell meta", []string{"/dev/sda;rm"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDisks(tt.disks)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDisks(%v) error = %v, wantErr %v", tt.disks, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUserName(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid", "skema999", false},
		{"underscore start", "_srv", false},
		{"empty", "", true},
		{"uppercase", "User", true},
		{"digit start", "9user", true},
		{"shell meta", "user;rm", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUserName(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUserName(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid", "skea-server", false},
		{"empty", "", true},
		{"dot not allowed", "host.name", true},
		{"too long", strings.Repeat("a", 64), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHostname(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostname(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid long", "AlpineLinux.2024", false},
		{"valid short", "Alpine.1", false},
		{"too short", "abc12", true},
		{"empty", "", true},
		{"exactly min", strings.Repeat("a", MinPasswordLen), false},
		{"shell meta", "abcdef123456`", true},
		{"unicode ok", "pässwörd-länge", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateLuksPassword(t *testing.T) {
	// Empty is allowed (falls back to login password)
	if err := ValidateLuksPassword(""); err != nil {
		t.Errorf("ValidateLuksPassword(\"\") = %v, want nil", err)
	}
	if err := ValidateLuksPassword("abc;rm"); err == nil {
		t.Errorf("ValidateLuksPassword(shell meta) = nil, want error")
	}
}

func TestValidateMirror(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid https", "https://dl-cdn.alpinelinux.org/alpine", false},
		{"valid http", "http://mirror.example.com", false},
		{"no scheme", "dl-cdn.alpinelinux.org", true},
		{"empty", "", true},
		{"shell meta", "https://x.com/a;b", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMirror(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMirror(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateExtraPackages(t *testing.T) {
	valid := []string{"incus-feature", "qemu-system-x86_64", "btrfs-progs"}
	if err := ValidateExtraPackages(valid); err != nil {
		t.Errorf("ValidateExtraPackages(%v) = %v, want nil", valid, err)
	}
	invalid := []string{"pkg", "bad;pkg"}
	if err := ValidateExtraPackages(invalid); err == nil {
		t.Errorf("ValidateExtraPackages(%v) = nil, want error", invalid)
	}
}

func TestValidateEnum(t *testing.T) {
	allowed := []string{"none", "kde", "niri", "cosmic"}
	if err := ValidateEnum("de", "niri", allowed); err != nil {
		t.Errorf("ValidateEnum(niri) = %v, want nil", err)
	}
	if err := ValidateEnum("de", "gnome", allowed); err == nil {
		t.Errorf("ValidateEnum(gnome) = nil, want error")
	}
	if err := ValidateEnum("de", "", allowed); err == nil {
		t.Errorf("ValidateEnum(empty) = nil, want error")
	}
}

func TestValidateSwapSize(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"auto", "auto", false},
		{"empty", "", false},
		{"gigabytes", "8G", false},
		{"megabytes", "4096M", false},
		{"kilobytes", "4096K", false},
		{"plain number", "4096", false},
		{"bogus unit", "8X", true},
		{"negative", "-8G", true},
		{"overflow", "99999999999999999999G", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSwapSize(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSwapSize(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDisksForLayout(t *testing.T) {
	tests := []struct {
		name    string
		layout  string
		disks   []string
		wantErr bool
	}{
		{"server 2 disks", "btrfsStandardServer", []string{"/dev/sda", "/dev/sdb"}, false},
		{"server 1 disk", "btrfsStandardServer", []string{"/dev/sda"}, true},
		{"raid0 2 disks", "btrfsRaid0", []string{"/dev/sda", "/dev/sdb"}, false},
		{"standard 1 disk", "btrfsStandard", []string{"/dev/sda"}, false},
		{"standard 2 disks", "btrfsStandard", []string{"/dev/sda", "/dev/sdb"}, true},
		{"unencrypted 1 disk", "btrfsUnencrypted", []string{"/dev/sda"}, false},
		{"unknown layout", "lvm", []string{"/dev/sda"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDisksForLayout(tt.layout, tt.disks)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDisksForLayout(%q, %v) error = %v, wantErr %v", tt.layout, tt.disks, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSSHKey(t *testing.T) {
	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... user@host"
	if err := ValidateSSHKey(key); err != nil {
		t.Errorf("ValidateSSHKey(valid) = %v, want nil", err)
	}
	if err := ValidateSSHKey("ssh-ed25519 AAA... $(rm -rf)"); err == nil {
		t.Errorf("ValidateSSHKey(shell meta) = nil, want error")
	}
}

func TestValidateBranchArchInterface(t *testing.T) {
	if err := ValidateBranch("latest-stable"); err != nil {
		t.Errorf("ValidateBranch(latest-stable) = %v", err)
	}
	if err := ValidateBranch("edge"); err != nil {
		t.Errorf("ValidateBranch(edge) = %v", err)
	}
	if err := ValidateBranch("3.22"); err != nil {
		t.Errorf("ValidateBranch(3.22) = %v", err)
	}
	if err := ValidateBranch("bad branch"); err == nil {
		t.Errorf("ValidateBranch(space) = nil, want error")
	}
	if err := ValidateArch("x86_64"); err != nil {
		t.Errorf("ValidateArch(x86_64) = %v", err)
	}
	if err := ValidateInterface("eth0"); err != nil {
		t.Errorf("ValidateInterface(eth0) = %v", err)
	}
	if err := ValidateInterface("enp3s0"); err != nil {
		t.Errorf("ValidateInterface(enp3s0) = %v", err)
	}
	if err := ValidateInterface("eth 0"); err == nil {
		t.Errorf("ValidateInterface(space) = nil, want error")
	}
}
