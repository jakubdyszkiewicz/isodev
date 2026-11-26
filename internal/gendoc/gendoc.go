// Package gendoc updates generated sections of the documentation site.
package gendoc

import (
	"bytes"
	"fmt"
	"os"
)

// Markers returns the comments that delimit a generated section. source tells
// readers what to edit instead of the generated text.
func Markers(source string) (string, string) {
	return "<!-- BEGIN GENERATED REFERENCE: edit " + source + " and run `go generate ./...` -->",
		"<!-- END GENERATED REFERENCE -->"
}

// Splice replaces the generated section of the Markdown file at path with content.
func Splice(path, source, content string) error {
	begin, end := Markers(source)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	b := bytes.Index(data, []byte(begin))
	e := bytes.Index(data, []byte(end))
	if b < 0 || e < b {
		return fmt.Errorf("%s must contain the generated reference markers:\n%s\n%s", path, begin, end)
	}
	out := string(data[:b]) + begin + "\n\n" + content + string(data[e:])
	return os.WriteFile(path, []byte(out), 0644) //nolint:gosec // A documentation file in the repository.
}
