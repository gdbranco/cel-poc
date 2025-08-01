package celvalidator

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// RuleEntry defines a CEL rule with optional dependent rules
type RuleEntry struct {
	Rule           string      `yaml:"rule"`
	Enabled        bool        `yaml:"enabled"`
	FailureMessage string      `yaml:"message,omitempty"`
	Then           []RuleEntry `yaml:"then,omitempty"`
}

// RuleSetMap maps StructName -> Operation -> Rules
type RuleSetMap map[string]map[string][]RuleEntry

// ValidationMetadata tracks where the rule came from and how it was activated
type ValidationMetadata struct {
	StructName string
	Operation  string
	ChainPath  string
	RuleIndex  int
	ParentRule string
}

// ValidationResult represents the outcome of a single rule evaluation
type ValidationResult struct {
	Rule     string
	Passed   bool
	Error    error
	Message  string
	Metadata ValidationMetadata
}

// Validator encapsulates options for validation
type Validator struct {
	partialEval bool
}

type ValidatorOption func(*Validator)

// New creates a new Validator
func NewValidator(opts ...ValidatorOption) *Validator {
	v := &Validator{}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// WithPartialEval enables partial evaluation mode
func WithPartialEval() ValidatorOption {
	return func(v *Validator) {
		v.partialEval = true
	}
}

// Validate evaluates rules and returns results with structured context
func (v *Validator) Validate(
	obj any,
	rules []RuleEntry,
	metadata ValidationMetadata,
) ([]ValidationResult, error) {
	results := []ValidationResult{}
	env, vars, err := v.buildEnv(obj)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}

	var eval func(entries []RuleEntry, metadata ValidationMetadata) error
	eval = func(entries []RuleEntry, metadata ValidationMetadata) error {
		for i, entry := range entries {
			if !entry.Enabled || seen[entry.Rule] {
				continue
			}
			seen[entry.Rule] = true

			ast, iss := env.Compile(entry.Rule)
			if iss != nil && iss.Err() != nil {
				results = append(results, ValidationResult{
					Rule:   entry.Rule,
					Passed: false,
					Error:  iss.Err(),
					Metadata: ValidationMetadata{
						StructName: metadata.StructName,
						Operation:  metadata.Operation,
						ChainPath:  metadata.ChainPath + " > compileError",
						RuleIndex:  i,
						ParentRule: metadata.ParentRule,
					},
				})
				if !v.partialEval {
					return iss.Err()
				}
				continue
			}

			prg, err := env.Program(ast)
			if err != nil {
				results = append(results, ValidationResult{
					Rule:   entry.Rule,
					Passed: false,
					Error:  err,
					Metadata: ValidationMetadata{
						StructName: metadata.StructName,
						Operation:  metadata.Operation,
						ChainPath:  metadata.ChainPath + " > programError",
						RuleIndex:  i,
						ParentRule: metadata.ParentRule,
					},
				})
				if !v.partialEval {
					return err
				}
				continue
			}

			out, _, err := prg.Eval(vars)
			passed := err == nil && out.Value() == true
			validationResult := ValidationResult{
				Rule:   entry.Rule,
				Passed: passed,
				Error:  err,
				Metadata: ValidationMetadata{
					StructName: metadata.StructName,
					Operation:  metadata.Operation,
					ChainPath:  metadata.ChainPath,
					RuleIndex:  i,
					ParentRule: metadata.ParentRule,
				},
			}
			if !passed {
				validationResult.Message = entry.FailureMessage
			}

			results = append(results, validationResult)

			if passed && len(entry.Then) > 0 {
				childMetadata := ValidationMetadata{
					StructName: metadata.StructName,
					Operation:  metadata.Operation,
					ChainPath:  extendChainPath(metadata.ChainPath, "then"),
					RuleIndex:  -1,
					ParentRule: entry.Rule,
				}
				if err := eval(entry.Then, childMetadata); err != nil && !v.partialEval {
					return err
				}
			}
		}
		return nil
	}

	err = eval(rules, metadata)
	return results, err
}

func extendChainPath(current, next string) string {
	if current == "" {
		return next
	}
	return current + " > " + next
}

// GetRulesFor retrieves rules for a struct (default) + operation from the rule set
func GetRulesFor(obj any, operation string, rules RuleSetMap) []RuleEntry {
	name := getStructName(obj)

	var merged []RuleEntry
	seen := map[string]bool{}

	if structRules, ok := rules[name]; ok {
		// Include Default rules if present
		if defaultRules, ok := structRules["Default"]; ok {
			for _, r := range defaultRules {
				if _, exists := seen[r.Rule]; !exists && r.Enabled {
					filtered := filterEnabledRules(r)
					merged = append(merged, filtered)
					seen[r.Rule] = true
				}
			}
		}

		// Include specific operation rules
		if opRules, ok := structRules[operation]; ok {
			for _, r := range opRules {
				if _, exists := seen[r.Rule]; !exists && r.Enabled {
					filtered := filterEnabledRules(r)
					merged = append(merged, filtered)
					seen[r.Rule] = true
				}
			}
		}
	}

	return merged
}

// filterEnabledRules returns a deep copy of a RuleEntry with only enabled nested rules
func filterEnabledRules(rule RuleEntry) RuleEntry {
	filtered := RuleEntry{
		Rule:           rule.Rule,
		Enabled:        rule.Enabled,
		FailureMessage: rule.FailureMessage,
	}

	for _, child := range rule.Then {
		if child.Enabled {
			filtered.Then = append(filtered.Then, filterEnabledRules(child))
		}
	}

	return filtered
}

// NewValidationMetadata creates a context from struct type and rule set
func NewValidationMetadata(obj any, operation string, rules RuleSetMap) ValidationMetadata {
	structName := getStructName(obj)
	op := operation

	if structRules, ok := rules[structName]; ok {
		if op == "" {
			if len(structRules) == 1 {
				for k := range structRules {
					op = k
				}
			} else {
				op = "Default"
			}
		}
	} else {
		op = "Default"
	}

	return ValidationMetadata{
		StructName: structName,
		Operation:  op,
		ChainPath:  "",
		RuleIndex:  -1,
		ParentRule: "",
	}
}

// buildEnv prepares the CEL environment and flattened variables
func (v *Validator) buildEnv(obj any) (*cel.Env, map[string]any, error) {
	fields := flattenStruct(obj)
	declarations := make([]*expr.Decl, 0, len(fields))
	for name, val := range fields {
		declarations = append(declarations, decls.NewVar(name, inferType(val)))
	}
	env, err := cel.NewEnv(cel.Declarations(declarations...))
	if err != nil {
		return nil, nil, err
	}
	return env, fields, nil
}

// flattenStruct flattens struct fields (including nested)
func flattenStruct(obj any) map[string]any {
	result := make(map[string]any)
	val := reflect.ValueOf(obj)
	typ := reflect.TypeOf(obj)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		if !value.CanInterface() {
			continue
		}

		name := field.Name

		switch value.Kind() {
		case reflect.Struct:
			nested := flattenStruct(value.Interface())
			for k, v := range nested {
				result[name+"."+k] = v
			}
		default:
			result[name] = value.Interface()
		}
	}
	return result
}

// inferType maps Go values to CEL types
func inferType(val any) *expr.Type {
	switch val.(type) {
	case map[string]any:
		// if you want to expose the map itself, use this:
		return decls.NewMapType(decls.String, decls.Dyn)
	case string:
		return decls.String
	case int, int64:
		return decls.Int
	case float32, float64:
		return decls.Double
	case bool:
		return decls.Bool
	default:
		return decls.Dyn
	}
}

// getStructName extracts the type name
func getStructName(obj any) string {
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
