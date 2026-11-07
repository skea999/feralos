package main

import (
	"testing"

	"gopkg.in/yaml.v3"

	"feralos/src/utils/firstboot"
)

// yamlKeys marshals v and returns the serialized yaml key set — the same
// shape the /opt/feralos/config.yaml file really carries.
func yamlKeys(t *testing.T, v any) map[string]bool {
	t.Helper()
	data, err := yaml.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var asMap map[string]any
	if err := yaml.Unmarshal(data, &asMap); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	out := make(map[string]bool, len(asMap))
	for k := range asMap {
		out[k] = true
	}
	return out
}

func fixtureInstallConfig() InstallConfig {
	return InstallConfig{
		DiskLayout:      "btrfsStandard",
		Disks:           []string{"/dev/sda"},
		LuksUUIDs:       []string{"1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"},
		UserName:        "skea999",
		Password:        "pass",
		Hostname:        "feral-host",
		Mirror:          "https://dl-cdn.alpinelinux.org/alpine",
		Branch:          "v3.24",
		EnablePodman:    true,
		EnableWebmin:    true,
		WebminListenAll: true,
	}
}

// TestInstallConfigYAMLKeysMatchLib pins the private struct against the
// writer's wire format (utils/firstboot.InstallConfig): every key a typo
// would rename silently breaks the reader. The private struct carries a
// SUBSET of the lib keys (no luks_uuids/kernel_* here) — keyset-subset-of
// is the drift guard.
func TestInstallConfigYAMLKeysMatchLib(t *testing.T) {
	libKeys := yamlKeys(t, firstboot.InstallConfig{
		DiskLayout:      "btrfsStandard",
		Disks:           []string{"/dev/sda"},
		LuksUUIDs:       []string{"1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"},
		UserName:        "skea999",
		Password:        "pass",
		Hostname:        "feral-host",
		Mirror:          "x",
		Branch:          "v3.24",
		KernelPackage:   "linux-lts",
		KernelFlavor:    "lts",
		EnablePodman:    true,
		EnableWebmin:    true,
		WebminListenAll: true,
	})
	ownKeys := yamlKeys(t, fixtureInstallConfig())
	for k := range ownKeys {
		if !libKeys[k] {
			t.Errorf("webmin InstallConfig yaml key %q not written by the lib InstallConfig (wire drift) — lib set: %v", k, libKeys)
		}
	}

	// Explicitly asserted-consumed keys must be present.
	for _, required := range []string{"password", "user_name", "enable_webmin", "webmin_listen_all"} {
		if !ownKeys[required] {
			t.Errorf("webmin reader lost required yaml key %q (a consumed field would be zero)", required)
		}
	}

	// Wire compat: a config written by the lib must parse without error.
	data, err := yaml.Marshal(firstboot.InstallConfig{})
	if err != nil {
		t.Fatalf("marshal lib: %v", err)
	}
	var back InstallConfig
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Errorf("lib-written config unparseable by webmin InstallConfig: %v", err)
	}
}
