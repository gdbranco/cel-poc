package main

import (
	"fmt"
	"log"

	"github.com/gdbranco/celvalidator"
)

type Address struct {
	City string
}

type User struct {
	Name     string
	Age      int
	Email    string
	IsActive bool
	Address  Address
}

func main() {
	user := User{
		Name:     "Bob",
		Age:      17,
		Email:    "",
		IsActive: false,
		Address:  Address{City: "LA"},
	}

	rulesMap, err := celvalidator.LoadRuleSetMapFromYAML("./assets/rules.yaml")
	if err != nil {
		log.Fatal(err)
	}

	createRules := celvalidator.GetRulesFor(user, "Create", rulesMap)
	if len(createRules) == 0 {
		log.Println("No rules found for User.Create")
	}

	validator := celvalidator.NewValidator(celvalidator.WithPartialEval())
	results, err := validator.Validate(user, createRules, celvalidator.NewValidationMetadata(user, "Create", rulesMap))
	if err != nil {
		log.Fatal("Validation error:", err)
	}

	for _, r := range results {
		fmt.Printf("[%v] %s\n", r.Passed, r.Rule)
		if r.Error != nil {
			fmt.Println(" Error:", r.Error)
		}
	}
}
