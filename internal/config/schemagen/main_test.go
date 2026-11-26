package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGeneratedFilesUpToDate fails when the committed schema or reference no
// longer match the config package. Run `go generate ./...` to update them.
func TestGeneratedFilesUpToDate(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	committedSchema := filepath.Join(root, "docs", "static", "config.schema.json")
	committedDocs := filepath.Join(root, "docs", "content", "docs", "configuration.md")
	dir := t.TempDir()
	schema, docs := filepath.Join(dir, "config.schema.json"), filepath.Join(dir, "config.md")
	original, err := os.ReadFile(committedDocs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(docs, original, 0600); err != nil {
		t.Fatal(err)
	}
	// Generate exactly as go generate does: from the config package directory.
	cmd := exec.Command("go", "run", "./schemagen", "-schema", schema, "-docs", docs)
	cmd.Dir = filepath.Join(root, "internal", "config")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v\n%s", err, out)
	}
	for _, pair := range [][2]string{{committedSchema, schema}, {committedDocs, docs}} {
		want, err := os.ReadFile(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is out of date; run `go generate ./...`", pair[0])
		}
	}
}
