package main

import (
	"github.com/spf13/pflag"
	"github.com/wjdp/wjdoc/internal/docs"
)

// addPriorityTagFlags registers --priority and --tag, which every command that
// selects documents shares. verb completes each flag's help, e.g. "keep only".
func addPriorityTagFlags(flags *pflag.FlagSet, criteria *docs.Criteria, verb string) {
	flags.StringSliceVarP(&criteria.Priorities, "priority", "p", nil, verb+" these priorities (comma-separated, repeatable)")
	flags.StringArrayVar(&criteria.Tags, "tag", nil, verb+" documents with this `tag`; repeat to require several, end with : for a namespace")
}

// addTypeStatusFlags registers --type and --status.
func addTypeStatusFlags(flags *pflag.FlagSet, criteria *docs.Criteria, verb string) {
	flags.StringSliceVarP(&criteria.Types, "type", "t", nil, verb+" these types (comma-separated, repeatable)")
	flags.StringSliceVarP(&criteria.Statuses, "status", "s", nil, verb+" these statuses (comma-separated, repeatable)")
}
