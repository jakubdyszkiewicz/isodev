// Command schemagen generates the JSON Schema and Markdown reference for
// config.yaml from the config package. Run it with `go generate ./...`.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/invopop/jsonschema"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/gendoc"
)

const (
	schemaID  = "https://jakubdyszkiewicz.github.io/isodev/config.schema.json"
	docSource = "the field comments in internal/config/config.go"
)

func main() {
	schemaPath := flag.String("schema", "", "JSON Schema output file")
	docsPath := flag.String("docs", "", "Markdown file whose generated reference section is replaced")
	flag.Parse()
	if *schemaPath == "" || *docsPath == "" {
		log.Fatal("usage: schemagen -schema FILE -docs FILE")
	}
	s, err := reflectSchema()
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*schemaPath, append(b, '\n'), 0644); err != nil { //nolint:gosec // A published file in the repository.
		log.Fatal(err)
	}
	if err := updateDocs(*docsPath, s); err != nil {
		log.Fatal(err)
	}
}

// reflectSchema builds the schema from the config types and their doc
// comments. It must run in the config package directory, as go generate does.
func reflectSchema() (*jsonschema.Schema, error) {
	r := &jsonschema.Reflector{
		FieldNameTag:               "yaml",
		RequiredFromJSONSchemaTags: true, // Only fields tagged `jsonschema:"required"`.
		ExpandedStruct:             true,
		// Unknown fields are rejected, matching the strict YAML decoder.
		AllowAdditionalProperties: false,
	}
	if err := r.AddGoComments("github.com/jakubdyszkiewicz/isodev/internal/config", "."); err != nil {
		return nil, err
	}
	s := r.Reflect(&config.Config{})
	s.ID = schemaID
	s.Title = "Isodev configuration"
	s.Description = "Global isodev configuration in ~/.config/isodev/config.yaml."
	setDefault(s, "image", config.DefaultImage)
	setDefault(s, "command", config.DefaultCommand())
	setDefault(definition(s, s.Properties.Value("build")), "dockerfile", config.DefaultDockerfile)
	return s, nil
}

func setDefault(s *jsonschema.Schema, property string, value any) {
	p := s.Properties.Value(property)
	if p == nil {
		log.Fatalf("schema has no property %q", property)
	}
	p.Default = value
}

// definition resolves a property that refers to a shared definition.
func definition(root, s *jsonschema.Schema) *jsonschema.Schema {
	if s == nil || s.Ref == "" {
		return s
	}
	name := strings.TrimPrefix(s.Ref, "#/$defs/")
	d, ok := root.Definitions[name]
	if !ok {
		log.Fatalf("unknown schema reference %s", s.Ref)
	}
	return d
}

// updateDocs replaces the generated reference in path.
func updateDocs(path string, root *jsonschema.Schema) error {
	var ref strings.Builder
	writeFields(&ref, root, root, "")
	return gendoc.Splice(path, docSource, ref.String())
}

// writeFields writes a heading per field, followed by its type, default and
// description. Nested fields use dotted names, and list items use [].
func writeFields(b *strings.Builder, root, s *jsonschema.Schema, prefix string) {
	s = definition(root, s)
	if s.Properties == nil {
		return
	}
	required := map[string]bool{}
	for _, name := range s.Required {
		required[name] = true
	}
	for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
		name, p := prefix+pair.Key, pair.Value
		facts := []string{typeName(root, p)}
		if required[pair.Key] {
			facts = append(facts, "required")
		}
		if text, ok := p.Default.(string); ok {
			facts = append(facts, "default `"+text+"`")
		} else if p.Default != nil {
			v, _ := json.Marshal(p.Default)
			facts = append(facts, "default `"+string(v)+"`")
		}
		fmt.Fprintf(b, "### `%s`\n\n%s\n\n%s\n\n", name, strings.Join(facts, " · "), oneLine(p.Description))
		if items := definition(root, p.Items); p.Type == "array" && items != nil && items.Properties != nil {
			writeFields(b, root, items, name+"[].")
		} else if d := definition(root, p); d.Properties != nil {
			writeFields(b, root, d, name+".")
		}
	}
}

func typeName(root, p *jsonschema.Schema) string {
	if len(p.Enum) > 0 {
		values := make([]string, len(p.Enum))
		for i, v := range p.Enum {
			values[i] = fmt.Sprintf("`%v`", v)
		}
		return strings.Join(values, " or ")
	}
	if p.Ref != "" {
		return "object"
	}
	switch p.Type {
	case "array":
		if items := definition(root, p.Items); items != nil && items.Properties != nil {
			return "list of objects"
		}
		return "list of " + p.Items.Type + "s"
	case "object":
		if p.AdditionalProperties != nil && p.AdditionalProperties.Type != "" {
			return "map of " + p.AdditionalProperties.Type + "s"
		}
	}
	return p.Type
}

// oneLine joins a doc comment's lines into one paragraph.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
