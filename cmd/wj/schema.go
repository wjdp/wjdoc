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

type schemaPriority struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type schemaTags struct {
	Pattern string `json:"pattern"`
	Example string `json:"example"`
}

type schemaFolder struct {
	Path  string   `json:"path"`
	Types []string `json:"types"`
}

type schemaLayout struct {
	Default  string         `json:"default"`
	OptIn    string         `json:"opt_in"`
	Folders  []schemaFolder `json:"folders"`
	AboutDoc string         `json:"about_doc"`
	Move     string         `json:"move"`
}

type schemaFrontmatter struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

type schemaDoc struct {
	Filename      schemaFilename    `json:"filename"`
	Layout        schemaLayout      `json:"layout"`
	Types         []schemaType      `json:"types"`
	Statuses      []schemaStatus    `json:"statuses"`
	Priorities    []schemaPriority  `json:"priorities"`
	Tags          schemaTags        `json:"tags"`
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

var priorityDescriptions = map[string]string{
	"p0": "drop everything",
	"p1": "high",
	"p2": "normal",
	"p3": "low",
	"p4": "nice to have",
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

	priorities := make([]schemaPriority, 0, len(docs.KnownPriorities))
	for _, name := range docs.KnownPriorities {
		priorities = append(priorities, schemaPriority{Name: name, Description: priorityDescriptions[name]})
	}

	return schemaDoc{
		Filename: schemaFilename{
			Pattern: `^\d+-[A-Z0-9].*\.md$`,
			Example: "007-Sync-robustness.md",
		},
		Layout: schemaLayout{
			Default: "flat: every document directly in docs/",
			OptIn:   "foldered when docs/ref/ or docs/task/ exists",
			Folders: []schemaFolder{
				{Path: "docs/" + docs.RefFolder + "/", Types: []string{"reference"}},
				{Path: "docs/" + docs.TaskFolder + "/", Types: []string{"*"}},
			},
			AboutDoc: "docs/",
			Move:     "wj move <ref>, or wj move --all to migrate",
		},
		Types:      types,
		Statuses:   statuses,
		Priorities: priorities,
		Tags: schemaTags{
			Pattern: docs.TagPattern.String(),
			Example: "area:engine",
		},
		Frontmatter: schemaFrontmatter{
			Required: []string{"type"},
			Optional: []string{"status", "priority", "tags"},
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
