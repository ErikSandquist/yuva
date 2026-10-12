package api

import (
	"regexp"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

var credentialName = regexp.MustCompile(`(?i)pass(word|phrase)?|secret|token|credential|user_?name|login|api_?key|auth|bearer|cookie`)

// No assistant may hand Yuva a password: the person enters it on the page create_migration
// returns. This fails the build when any tool's input can carry something named like a credential.
func TestNoMCPToolTakesACredential(t *testing.T) {
	var walk func(tool, path string, sc *jsonschema.Schema)
	walk = func(tool, path string, sc *jsonschema.Schema) {
		if sc == nil {
			return
		}
		for name, prop := range sc.Properties {
			if credentialName.MatchString(name) {
				t.Errorf("%s takes %s%s, which is named like a credential", tool, path, name)
			}
			walk(tool, path+name+".", prop)
		}
		walk(tool, path, sc.Items)
		walk(tool, path, sc.AdditionalProperties)
	}
	for _, tool := range mcpTools {
		if tool.input == nil {
			t.Fatalf("%s has no input schema", tool.name)
		}
		walk(tool.name, "", tool.input)
	}
}
