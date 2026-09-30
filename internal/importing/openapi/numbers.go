package openapi

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAML's generic decoder rounds large integers and decimals through float64.
// Restore numeric scalars from their source nodes after its validation and merge
// handling. Aliases are already cycle/expansion checked by the YAML decoder;
// this walk additionally limits work and follows the same merge precedence.
func exactYAMLNumbers(node *yaml.Node, value any, budget *int) (any, error) {
	*budget--
	if *budget < 0 {
		return nil, fmt.Errorf("OpenAPI YAML expansion exceeds 250000 nodes")
	}
	switch node.Kind {
	case yaml.DocumentNode:
		return exactYAMLNumbers(node.Content[0], value, budget)
	case yaml.AliasNode:
		return exactYAMLNumbers(node.Alias, value, budget)
	case yaml.MappingNode:
		values, ok := value.(object)
		if !ok {
			return value, nil
		}
		fields := map[string]*yaml.Node{}
		if err := yamlFields(node, fields, budget); err != nil {
			return nil, err
		}
		for _, key := range keys(values) {
			source := fields[key]
			if source == nil {
				continue
			}
			v, err := exactYAMLNumbers(source, values[key], budget)
			if err != nil {
				return nil, err
			}
			values[key] = v
		}
	case yaml.SequenceNode:
		values, ok := value.([]any)
		if !ok {
			return value, nil
		}
		for i, source := range node.Content {
			v, err := exactYAMLNumbers(source, values[i], budget)
			if err != nil {
				return nil, err
			}
			values[i] = v
		}
	case yaml.ScalarNode:
		s := node.Value
		// The YAML decoder misclassifies out-of-range exponents as strings. Only
		// implicit plain scalars may be recovered; quoted and !!str values stay text.
		numberTag := node.Tag == "!!int" || node.Tag == "!!float"
		implicit := node.Style == 0 && node.Tag == "!!str"
		if numberTag || implicit {
			if jsonNumber(s) {
				return json.Number(s), nil
			}
		}
		if node.Tag == "!!int" || implicit {
			if n, ok := new(big.Int).SetString(strings.ReplaceAll(s, "_", ""), 0); ok {
				return json.Number(n.String()), nil
			}
		}
		if node.Tag == "!!float" || implicit {
			s = strings.ReplaceAll(s, "_", "")
			s = strings.TrimPrefix(s, "+")
			if strings.HasPrefix(s, ".") {
				s = "0" + s
			}
			if strings.HasPrefix(s, "-.") {
				s = "-0" + s[1:]
			}
			// YAML permits a decimal point with no following digits.
			s = strings.ReplaceAll(strings.ReplaceAll(s, ".e", ".0e"), ".E", ".0E")
			if strings.HasSuffix(s, ".") {
				s += "0"
			}
			// YAML also permits leading zeros in decimal floating-point values.
			sign := ""
			if strings.HasPrefix(s, "-") {
				sign, s = "-", s[1:]
			}
			split := strings.IndexAny(s, ".eE")
			if split < 0 {
				split = len(s)
			}
			integer := strings.TrimLeft(s[:split], "0")
			if integer == "" {
				integer = "0"
			}
			s = sign + integer + s[split:]
			if jsonNumber(s) {
				return json.Number(s), nil
			}
		}
	}
	return value, nil
}

func jsonNumber(s string) bool {
	return len(s) > 0 && (s[0] == '-' || s[0] >= '0' && s[0] <= '9') && json.Valid([]byte(s))
}

func yamlFields(node *yaml.Node, out map[string]*yaml.Node, budget *int) error {
	*budget--
	if *budget < 0 {
		return fmt.Errorf("OpenAPI YAML expansion exceeds 250000 nodes")
	}
	if node.Kind == yaml.AliasNode {
		return yamlFields(node.Alias, out, budget)
	}
	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			if err := yamlFields(child, out, budget); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	// Explicit keys beat merges, and the first merged mapping beats later ones.
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		if key.Tag != "!!merge" && out[key.Value] == nil {
			out[key.Value] = node.Content[i+1]
		}
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Tag == "!!merge" {
			if err := yamlFields(node.Content[i+1], out, budget); err != nil {
				return err
			}
		}
	}
	return nil
}
