package docs

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

// decodeFrontmatter parses a YAML mapping. A non-mapping document (a scalar or
// list) is an error so callers can report it rather than silently ignore it.
func decodeFrontmatter(source []byte) (map[string]any, error) {
	var value any
	if err := yaml.Unmarshal(source, &value); err != nil {
		return nil, err
	}
	switch mapping := value.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return mapping, nil
	default:
		return nil, fmt.Errorf("frontmatter is not a mapping")
	}
}
