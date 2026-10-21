package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wjdp/wjdoc/internal/docs"
)

type schemaFilename struct {
	Pattern string `json:"pattern"`
	Example string `json:"example"`
}

type schemaType struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	StatusRequired bool   `json:"status_required"`
}

type schemaStatus struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Types       []string `json:"types"`
}

type schemaFrontmatter struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

type schemaDoc struct {
	Filename      schemaFilename    `json:"filename"`
	Types         []schemaType      `json:"types"`
	Statuses      []schemaStatus    `json:"statuses"`
	Frontmatter   schemaFrontmatter `json:"frontmatter"`
	UnknownValues string            `json:"unknown_values"`
}

var statusDescriptions = map[string]string{
	"todo":        "immediate",
	"planned":     "longer term",
	"in-progress": "in progress",
	"blocked":     "blocked",
	"open":        "open reviews only",
	"shelved":     "parked indefinitely, tasks and bugs only",
	"done":        "done",
	"rejected":    "decided against, tasks and bugs only",
	"superseded":  "superseded",
}

func buildSchema() schemaDoc {
	types := make([]schemaType, 0, len(docs.Types))
	for _, docType := range docs.Types {
		types = append(types, schemaType{Name: docType.Name, Description: docType.Description, StatusRequired: docType.StatusRequired})
	}

	statuses := make([]schemaStatus, 0, len(docs.KnownStatuses))
	for _, name := range docs.KnownStatuses {
		statuses = append(statuses, schemaStatus{Name: name, Description: statusDescriptions[name], Types: docs.StatusTypesFor(name)})
	}

	return schemaDoc{
		Filename: schemaFilename{
			Pattern: `^\d+-[A-Z0-9].*\.md$`,
			Example: "007-Sync-robustness.md",
		},
		Types:    types,
		Statuses: statuses,
		Frontmatter: schemaFrontmatter{
			Required: []string{"type"},
			Optional: []string{"status"},
		},
		UnknownValues: "warn",
	}
}

func schemaCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Print the docs/ convention as JSON",
		Long: `Print the filename, frontmatter, type and status rules for docs/ as JSON, for machines.

Same rules as wj conventions, structured for programmatic use: an agent or
script can read types, statuses and which statuses apply to which types
without parsing prose.

Examples:
  wj schema                jump straight into the convention as JSON
  wj schema | jq .types    list the known types`,
		Args: cobra.NoArgs,
		// Runs anywhere: no project root is needed, so skip the root lookup.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(command *cobra.Command, _ []string) error {
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(buildSchema()); err != nil {
				return fmt.Errorf("encode schema: %w", err)
			}
			return nil
		},
	}
}
