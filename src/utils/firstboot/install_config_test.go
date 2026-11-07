package firstboot

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// fixture returns a fully populated InstallConfig (every field set — nil vs
// empty slices must not sneak through the round-trip).
func fixtureAllFields() InstallConfig {
	return InstallConfig{
		DiskLayout:      "btrfsStandard",
		Disks:           []string{"/dev/sda"},
		LuksUUIDs:       []string{"1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"},
		UserName:        "skea999",
		Password:        "pass",
		Hostname:        "feral-host",
		Mirror:          "https://dl-cdn.alpinelinux.org/alpine",
		Branch:          "v3.24",
		KernelPackage:   "linux-lts",
		KernelFlavor:    "lts",
		EnablePodman:    true,
		EnableWebmin:    true,
		WebminListenAll: true,
	}
}

// WantKeys are the YAML tags the standalone firstBoot binaries parse
// (utils/firstboot/tpm/main.go etc. read /opt/feralos/config.yaml): a tag
// drift silently breaks every reader — pin the wire format.
var wantKeys = map[string]bool{
	"disk_layout": true, "disks": true, "luks_uuids": true,
	"user_name": true, "password": true, "hostname": true,
	"mirror": true, "branch": true,
	"kernel_package": true, "kernel_flavor": true,
	"enable_podman": true, "enable_webmin": true, "webmin_listen_all": true,
}

func TestInstallConfigYAMLShape(t *testing.T) {
	t.Parallel()

	want := fixtureAllFields()
	data, err := yaml.Marshal(&want)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// 1. Top-level keys are EXACTLY the pinned set (serialized map).
	var asMap map[string]any
	if err := yaml.Unmarshal(data, &asMap); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}
	gotKeys := make(map[string]bool, len(asMap))
	for k := range asMap {
		gotKeys[k] = true
	}
	for k := range wantKeys {
		if !gotKeys[k] {
			t.Errorf("missing yaml key %q (a firstBoot reader would see no data)\ngot: %s", k, data)
			continue
		}
		delete(gotKeys, k)
	}
	for k := range gotKeys {
		t.Errorf("unexpected yaml key %q (wire format drift)", k)
	}

	// 2. Round-trip: unmarshal back into InstallConfig — DeepEqual intact.
	var back InstallConfig
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(back, want) {
		t.Errorf("round-trip mismatch:\nback = %+v\nwant = %+v", back, want)
	}
}

func TestInstallConfigYAMLBooleans(t *testing.T) {
	t.Parallel()

	// False bools must still SERIALIZE (writer emits full struct);
	// readers rely on explicit false, not on yaml zero-value defaults.
	data, err := yaml.Marshal(&InstallConfig{Password: "pass"})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	for _, k := range []string{"enable_podman", "enable_webmin", "webmin_listen_all"} {
		if !strings.Contains(string(data), k+":") {
			t.Errorf("zero-value bool %q dropped from yaml:\n%s", k, data)
		}
	}
}
