package createConfig

import (
	"os"

	"gopkg.in/yaml.v3"
)

// rulesCandidatePaths mirrors config.LoadInternalRules' search paths.
// Duplicated LOCALLY on purpose: createConfig is standalone-confined (builds
// as its own binary) and gains no dependency on the installer's config
// loader — the rules file was already being read from here; this keeps the
// read order-conservative instead of decoding it into maps first.
var rulesCandidatePaths = []string{
	"src/variables/internal_rules.yaml",
	"variables/internal_rules.yaml",
	"../variables/internal_rules.yaml",
}

// ruleEnumOrder carries the key order of internal_rules.yaml's three
// enum-source mappings (diskLayouts / desktopEnvironments / videoDrivers)
// exactly as they appear in the file. Empty slices mean "no order info
// here" — the wizard then falls back to map iteration via enumOptions.
type ruleEnumOrder struct {
	diskLayouts         []string
	desktopEnvironments []string
	videoDrivers        []string
}

// readInternalRulesBytes reads internal_rules.yaml from the first probe
// path that exists (nil = no path worked). Silent on failure: the installer
// already hard-fails when config.LoadInternalRules cannot load the rules,
// so a nil result only means "no order data" (unordered prompt fallback) —
// keeping stdout quiet preserves the prompt stream byte-stability.
func readInternalRulesBytes() []byte {
	for _, p := range rulesCandidatePaths {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	return nil
}

// parseRuleEnumOrder walks internal_rules.yaml WITHOUT unmarshalling into
// maps: yaml.v3's yaml.Node keeps document order, so the enum-source keys
// come out in file order. Mapping shape problems (root/key absent or not a
// mapping) yield an empty slice for that map only — never an error.
func parseRuleEnumOrder(data []byte) ruleEnumOrder {
	var out ruleEnumOrder
	if data == nil {
		return out
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return out
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return out
	}
	top := root.Content[0]
	out.diskLayouts = mappingKeyOrder(findMappingChild(top, "diskLayouts"))
	out.desktopEnvironments = mappingKeyOrder(findMappingChild(top, "desktopEnvironments"))
	out.videoDrivers = mappingKeyOrder(findMappingChild(top, "videoDrivers"))
	return out
}

// findMappingChild returns the value node of key inside a yaml MappingNode,
// or nil when absent or not a mapping.
func findMappingChild(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// mappingKeyOrder returns the keys of a yaml MappingNode in file order.
func mappingKeyOrder(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		keys = append(keys, node.Content[i].Value)
	}
	return keys
}
