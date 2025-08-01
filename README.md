# celvalidator

`celvalidator` is a lightweight, extensible validation engine for Go that uses Google's [Common Expression Language (CEL)](https://github.com/google/cel-spec) to define and execute validation rules against struct-based input.

It allows you to associate dynamic, declarative validation rules with struct types and operations (like `"Create"`, `"Update"`, etc.), supporting complex conditional logic through rule chaining.

---

## ✨ Features

- **CEL-powered** – expressive, type-safe validation expressions
- **Rule chaining** – conditionally trigger sub-rules using `Then`
- **Partial evaluation** – option to continue validation on failure
- **Struct-based mapping** – associate rules by struct name and operation
- **Contextual metadata results** – includes detailed metadata for every rule outcome

## Usage
1. Define Your Rules
You can define rules in Go, JSON, or YAML. Example in YAML:
```yaml
- rule: "Amount > 0"
  enabled: true
  message: "Amount must be positive"
  then:
    - rule: "Currency == 'USD'"
      enabled: true
      message: "Currency must be USD if amount is positive"
```
2. Map Rules to Structs and Operations
Use a RuleSetMap to group rules by struct name and operation (e.g., "Create", "Update"):
```go
rules := celvalidator.RuleSetMap{
  "PaymentRequest": {
    "Create": []celvalidator.RuleEntry{
      {
        Rule:    "Amount > 0",
        Enabled: true,
        Message: "Amount must be positive",
        Then: []celvalidator.RuleEntry{
          {
            Rule:    "Currency == 'USD'",
            Enabled: true,
            Message: "Currency must be USD if amount is positive",
          },
        },
      },
    },
  },
}
```
3. Validate Your Data
```go
request := PaymentRequest{Amount: 100, Currency: "EUR"}

validator := celvalidator.NewValidator(celvalidator.WithPartialEval())
metadata := celvalidator.NewValidationMetadata(request, "Create", rules)
ruleSet := celvalidator.GetRulesFor(request, "Create", rules)

results, err := validator.Validate(request, ruleSet, metadata)

for _, res := range results {
  fmt.Printf("Rule: %s | Passed: %v | Msg: %s\n", res.Rule, res.Passed, res.Message)
}
```
#### Rule Evaluation Flow
* Rules are compiled using the CEL environment.
* If a rule passes and has a Then clause, its child rules are evaluated.
* Each result includes:
* * Whether the rule passed
* * Compilation or runtime errors
* * Message (if provided)
* * Context (struct, operation, rule index, parent rule, etc.)

#### Partial Evaluation
Use WithPartialEval() to prevent early termination on failure:
```go
validator := celvalidator.NewValidator(celvalidator.WithPartialEval())
```


### CEL Rule Syntax
CEL allows you to write rules like:
```cel
Amount > 100 && Currency == "USD"
User.Age >= 18
Metadata["source"] == "api"
```
For full syntax and features, refer to the [CEL Go documentation](https://github.com/google/cel-spec/blob/master/doc/langdef.md).