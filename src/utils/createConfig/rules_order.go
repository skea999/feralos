package createConfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"feralos/src/utils/config"
)

// fallbackInfo is the one-line notice emitted when the rules read for
// option ordering fails — the wizard then falls back to unordered enum
// options, and the fallback stays VISIBLE instead of silent.
const fallbackInfo = "[INFO] internal_rules unreadable for option ordering — using map order"

// ruleEnumOrder carries the key order of internal_rules.yaml's three
// enum-source mappings (diskLayouts / desktopEnvironments / videoDrivers)
// exactly as they appear in the file. Empty slices mean "no order info
// here" — the wizard then falls back to map iteration via enumOptions.
type ruleEnumOrder struct {
	diskLayouts         []string
	desktopEnvironments []string
	videoDrivers        []string
}

// readInternalRulesBytes reads internal_rules.yaml through the shared
// resolver (config.FindInternalRulesPath — the single probe-list source;
// the former locally duplicated path list is gone; createValues already
// imports utils/config for validators — no new coupling). Failure keeps
// the unordered fallback but no longer silently: one INFO line marks it
// (the installer still hard-fails when config.LoadInternalRules cannot
// load the rules, so nil here only means "no order data").
func readInternalRulesBytes() []byte {
	path, err := config.FindInternalRulesPath()
	if err != nil {
		fmt.Println(fallbackInfo)
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(fallbackInfo)
		return nil
	}
	return data
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
