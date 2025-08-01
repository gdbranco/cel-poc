package celvalidator

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type Address struct {
	City    string
	Country string
	Zip     int
}

type User struct {
	Name     string
	Age      int
	Email    string
	IsActive bool
	Address  Address
}

type Sample struct {
	Active  bool
	Age     int
	Email   string
	Details map[string]string
}

func TestValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validator Suite")
}

var _ = Describe("Validator", func() {
	var v *Validator
	var obj Sample

	BeforeEach(func() {
		v = NewValidator()
		obj = Sample{
			Active:  true,
			Age:     21,
			Email:   "test@example.com",
			Details: map[string]string{"type": "admin"},
		}
	})

	It("errors early if not partial eval", func() {
		ruleMap := RuleSetMap{
			"Sample": map[string][]RuleEntry{
				"Create": {
					{
						Rule:    "UnknownField == true", // invalid rule
						Enabled: true,
					},
					{
						Rule:    "Age > 18",
						Enabled: true,
					},
					{
						Rule:    "Email != ''",
						Enabled: true,
					},
				},
			},
		}
		results, err := v.Validate(obj, GetRulesFor(obj, "Create", ruleMap), NewValidationMetadata(obj, "Create", ruleMap))
		Expect(err).To(HaveOccurred())
		Expect(results).To(HaveLen(1))

		Expect(results[0].Passed).To(BeFalse())
		Expect(results[0].Error).To(HaveOccurred())
	})

	It("checks message associated with rule", func() {
		ruleMap := RuleSetMap{
			"Sample": map[string][]RuleEntry{
				"Create": {
					{
						Rule:           "Details.type != 'admin'",
						Enabled:        true,
						FailureMessage: "type should not be admin",
					},
					{
						Rule:           "Details['target'] != 'guest'",
						Enabled:        true,
						FailureMessage: "target should not be guest",
					},
					{
						Rule:    "Email != ''",
						Enabled: true,
					},
				},
			},
		}
		obj := Sample{
			Active:  true,
			Age:     20,
			Email:   "test@example.com",
			Details: map[string]string{"type": "admin", "target": "guest"},
		}
		results, err := v.Validate(obj, GetRulesFor(obj, "Create", ruleMap), NewValidationMetadata(obj, "Create", ruleMap))
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(3))

		Expect(results[0].Passed).To(BeFalse())
		Expect(results[0].Rule).To(Equal(ruleMap["Sample"]["Create"][0].Rule))
		Expect(results[0].Message).To(Equal(ruleMap["Sample"]["Create"][0].FailureMessage))
		Expect(results[0].Error).To(BeNil()) // No runtime error, just evaluated false

		Expect(results[1].Passed).To(BeFalse())
		Expect(results[1].Rule).To(Equal(ruleMap["Sample"]["Create"][1].Rule))
		Expect(results[1].Message).To(Equal(ruleMap["Sample"]["Create"][1].FailureMessage))
		Expect(results[1].Error).To(BeNil()) // No runtime error, just evaluated false

		Expect(results[2].Passed).To(BeTrue())
		Expect(results[2].Rule).To(Equal(ruleMap["Sample"]["Create"][2].Rule))
		Expect(results[2].Message).To(Equal(ruleMap["Sample"]["Create"][2].FailureMessage))
		Expect(results[2].Error).To(BeNil())
	})

	It("continues evaluation if AllowPartialEval is enabled", func() {
		v := NewValidator(WithPartialEval())
		ruleMap := RuleSetMap{
			"Sample": map[string][]RuleEntry{
				"Create": {
					{
						Rule:    "Age > 18",
						Enabled: true,
					},
					{
						Rule:    "UnknownField == true", // fails
						Enabled: true,
					},
					{
						Rule:    "Email != ''", // Should still be evaluated
						Enabled: true,
					},
				},
			},
		}
		results, err := v.Validate(obj, GetRulesFor(obj, "Create", ruleMap), NewValidationMetadata(obj, "Create", ruleMap))
		Expect(err).To(BeNil())
		Expect(results).To(HaveLen(3))
		Expect(results[0].Passed).To(BeTrue())
		Expect(results[1].Passed).To(BeFalse())
		Expect(results[2].Passed).To(BeTrue())
	})

	It("loads CRUD-specific rules", func() {
		yaml := `User:
  Create:
    - rule: "Age > 18"
      enabled: true
    - rule: "Email != ''"
      enabled: true
  Delete:
    - rule: "IsActive == false"
      enabled: true`
		os.WriteFile("temp_rules.yaml", []byte(yaml), 0644)
		defer os.Remove("temp_rules.yaml")

		rulesMap, err := LoadRuleSetMapFromYAML("temp_rules.yaml")
		Expect(err).To(BeNil())

		user := User{Age: 20, Email: "x@x.com", IsActive: false}

		createRules := GetRulesFor(user, "Create", rulesMap)
		Expect(createRules).To(HaveLen(2))

		deleteRules := GetRulesFor(user, "Delete", rulesMap)
		Expect(deleteRules).To(HaveLen(1))
	})

	It("merges Default and operation-specific rules", func() {
		yaml := `User:
  Default:
    - rule: "Email != ''"
      enabled: true
  Create:
    - rule: "Age > 18"
      enabled: true`
		os.WriteFile("test_rules.yaml", []byte(yaml), 0644)
		defer os.Remove("test_rules.yaml")

		rulesMap, err := LoadRuleSetMapFromYAML("test_rules.yaml")
		Expect(err).To(BeNil())

		user := User{Age: 25, Email: "valid@example.com"}
		rules := GetRulesFor(user, "Create", rulesMap)

		Expect(rules).To(ContainElements(
			RuleEntry{
				Rule:    "Email != ''",
				Enabled: true,
			},
			RuleEntry{
				Rule:    "Age > 18",
				Enabled: true,
			},
		))
	})

	It("deduplicates rules between Default and operation", func() {
		yaml := `User:
  Default:
    - rule: "Email != ''"
      enabled: true	
    - rule: "Age >= 18"
      enabled: true
  Create:
    - rule: "Age >= 18"
      enabled: true
    - rule: "IsActive == true"
      enabled: true`
		os.WriteFile("dedup_rules.yaml", []byte(yaml), 0644)
		defer os.Remove("dedup_rules.yaml")

		rulesMap, err := LoadRuleSetMapFromYAML("dedup_rules.yaml")
		Expect(err).To(BeNil())

		user := User{Age: 30, Email: "x@x.com", IsActive: true}
		rules := GetRulesFor(user, "Create", rulesMap)

		Expect(rules).To(HaveLen(3))
		Expect(rules).To(ConsistOf(
			RuleEntry{
				Rule:    "Email != ''",
				Enabled: true,
			},
			RuleEntry{
				Rule:    "Age >= 18",
				Enabled: true,
			},
			RuleEntry{
				Rule:    "IsActive == true",
				Enabled: true,
			},
		))
	})

	It("ignores disabled rules in YAML", func() {
		yaml := `User:
  Default:
    - rule: "Email != ''"
      enabled: true
    - rule: "Age >= 18"
      enabled: false
  Create:
    - rule: "IsActive == true"
      enabled: true`
		os.WriteFile("enabled_rules.yaml", []byte(yaml), 0644)
		defer os.Remove("enabled_rules.yaml")

		rulesMap, err := LoadRuleSetMapFromYAML("enabled_rules.yaml")
		Expect(err).To(BeNil())

		user := User{Age: 30, Email: "x@x.com", IsActive: true}
		rules := GetRulesFor(user, "Create", rulesMap)

		Expect(rules).To(HaveLen(2))
		Expect(rules).To(ConsistOf(
			RuleEntry{
				Rule:    "Email != ''",
				Enabled: true,
			},
			RuleEntry{
				Rule:    "IsActive == true",
				Enabled: true,
			},
		))
	})

	Context("with nested struct fields", func() {
		var user User
		var validator *Validator

		BeforeEach(func() {
			user = User{
				Name:     "Alice",
				Age:      35,
				Email:    "alice@example.com",
				IsActive: true,
				Address: Address{
					City:    "Toronto",
					Country: "CA",
					Zip:     12345,
				},
			}
			validator = NewValidator()
		})

		It("validates rules on nested struct fields", func() {
			ruleMap := RuleSetMap{
				"User": map[string][]RuleEntry{
					"Create": {
						{
							Rule:    "Address.City == 'Toronto'",
							Enabled: true,
						},
						{
							Rule:    "Address.Zip > 10000",
							Enabled: true,
						},
						{
							Rule:    "IsActive == true",
							Enabled: true,
						},
					},
				},
			}

			results, err := validator.Validate(user, GetRulesFor(user, "Create", ruleMap), NewValidationMetadata(user, "Create", ruleMap))
			Expect(err).To(BeNil())
			Expect(results).To(HaveLen(3))

			for _, res := range results {
				Expect(res.Passed).To(BeTrue(), "Rule failed: %s", res.Rule)
			}
		})

		It("returns detailed error when a nested rule fails", func() {
			ruleMap := RuleSetMap{
				"User": map[string][]RuleEntry{
					"Create": {
						{
							Rule:    "Address.City == 'Toronto'",
							Enabled: true,
						},
						{
							Rule:    "Address.Zip < 100", // should fail
							Enabled: true,
						},
						{
							Rule:    "Email != ''",
							Enabled: true,
						},
					},
				},
			}

			results, err := validator.Validate(user, GetRulesFor(user, "Create", ruleMap), NewValidationMetadata(user, "Create", ruleMap))
			Expect(err).To(BeNil())
			Expect(results).To(HaveLen(3))

			Expect(results[1].Passed).To(BeFalse())
			Expect(results[1].Error).To(BeNil()) // Evaluation failed, not a runtime error
		})

		It("fails on invalid nested field if partial eval is off", func() {
			ruleMap := RuleSetMap{
				"User": map[string][]RuleEntry{
					"Create": {
						{
							Rule:    "Address.Zip > 0",
							Enabled: true,
						},
						{
							Rule:    "Address.UnknownField == 'oops'",
							Enabled: true,
						},
						{
							Rule:    "IsActive == true",
							Enabled: true,
						},
					},
				},
			}

			results, err := validator.Validate(user, GetRulesFor(user, "Create", ruleMap), NewValidationMetadata(user, "Create", ruleMap))
			Expect(err).To(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(results[1].Passed).To(BeFalse())
			Expect(results[1].Error).To(HaveOccurred())
		})

		It("evaluates all with partial eval enabled", func() {
			validator = NewValidator(WithPartialEval())
			ruleMap := RuleSetMap{
				"User": map[string][]RuleEntry{
					"Create": {
						{
							Rule:    "Address.Zip > 0",
							Enabled: true,
						},
						{
							Rule:    "Address.UnknownField == 'oops'",
							Enabled: true,
						},
						{
							Rule:    "Age > 30",
							Enabled: true,
						},
					},
				},
			}

			results, err := validator.Validate(user, GetRulesFor(user, "Create", ruleMap), NewValidationMetadata(user, "Create", ruleMap))
			Expect(err).To(BeNil())
			Expect(results).To(HaveLen(3))
			Expect(results[0].Passed).To(BeTrue())
			Expect(results[1].Passed).To(BeFalse())
			Expect(results[1].Error).To(HaveOccurred())
			Expect(results[2].Passed).To(BeTrue())
		})
	})

	Context("Nested then rules", func() {
		type Sample struct {
			Name  string
			Count int
		}
		var rules RuleSetMap

		BeforeEach(func() {
			rules = RuleSetMap{
				"Sample": {
					"Default": []RuleEntry{
						{
							Rule:    "Count > 0",
							Enabled: true,
							Then: []RuleEntry{
								{
									Rule:    "Name != ''",
									Enabled: true,
								},
								{
									Rule:    "Name != 'test'",
									Enabled: false,
								},
							},
						},
						{
							Rule:    "falseRule",
							Enabled: false,
						},
					},
					"Create": []RuleEntry{
						{
							Rule:    "Count < 100",
							Enabled: true,
						},
					},
				},
			}
		})

		It("should include only enabled top-level and nested rules", func() {
			result := GetRulesFor(Sample{}, "Default", rules)
			Expect(result).To(HaveLen(1)) // Only one top-level enabled rule

			Expect(result[0].Rule).To(Equal("Count > 0"))
			Expect(result[0].Then).To(HaveLen(1)) // Only one enabled Then rule
			Expect(result[0].Then[0].Rule).To(Equal("Name != ''"))
		})

		It("should include operation-specific rules", func() {
			result := GetRulesFor(Sample{}, "Create", rules)
			Expect(result).To(HaveLen(2))
			Expect(result).To(ContainElement(RuleEntry{Rule: "Count < 100", Enabled: true}))
		})

		It("should merge default and operation-specific rules without duplicates", func() {
			result := GetRulesFor(Sample{}, "Create", rules)
			Expect(result).To(ContainElement(HaveField("Rule", "Count > 0")))
			Expect(result).To(ContainElement(HaveField("Rule", "Count < 100")))
		})

		It("should return empty if all rules are disabled", func() {
			rules["Sample"]["Default"][0].Enabled = false
			rules["Sample"]["Default"][1].Enabled = false
			result := GetRulesFor(Sample{}, "Default", rules)
			Expect(result).To(BeEmpty())
		})
	})
})
