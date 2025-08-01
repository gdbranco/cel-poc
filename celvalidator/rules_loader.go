package celvalidator

import (
	"fmt"
	"os"
	"reflect"

	"gopkg.in/yaml.v3"
)

// LoadRuleSetMapFromYAML loads the nested rule set YAML
func LoadRuleSetMapFromYAML(path string) (RuleSetMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading rule file: %w", err)
	}

	var rules RuleSetMap
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("unmarshalling YAML: %w", err)
	}

	return rules, nil
}

// StructName returns the type name of a struct (without pointer or package prefix)
func StructName(obj interface{}) string {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
