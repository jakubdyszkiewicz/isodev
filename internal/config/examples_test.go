package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jakubdyszkiewicz/isodev/internal/paths"
	"github.com/jakubdyszkiewicz/isodev/internal/testutil"
)

const repoRoot = "../.."

// yamlBlock matches fenced YAML code blocks in Markdown.
var yamlBlock = regexp.MustCompile("(?ms)^```yaml\n(.*?)^```")

// TestDocumentedConfigsLoad keeps documentation honest: every YAML block in the
// README and documentation site, and every example configuration, must pass
// the strict loader.
func TestDocumentedConfigsLoad(t *testing.T) {
	docs := []string{filepath.Join(repoRoot, "README.md")}
	err := filepath.WalkDir(filepath.Join(repoRoot, "docs", "content"), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) == ".md" {
			docs = append(docs, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	blocks := 0
	for _, doc := range docs {
		data, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		for i, m := range yamlBlock.FindAllSubmatch(data, -1) {
			blocks++
			t.Run(filepath.Base(doc)+"#"+strconv.Itoa(i+1), func(t *testing.T) {
				testutil.Config(t, string(m[1]))
				checkLoads(t)
			})
		}
	}
	if blocks == 0 {
		t.Fatal("no YAML blocks found in the documentation")
	}

	for _, example := range exampleConfigs(t) {
		t.Run(filepath.Base(filepath.Dir(example)), func(t *testing.T) {
			testutil.Home(t)
			if err := os.CopyFS(paths.ConfigDir(), os.DirFS(filepath.Dir(example))); err != nil {
				t.Fatal(err)
			}
			checkLoads(t)
		})
	}
}

// exampleConfigs returns every config.yaml under examples/.
func exampleConfigs(t *testing.T) []string {
	t.Helper()
	var configs []string
	err := filepath.WalkDir(filepath.Join(repoRoot, "examples"), func(path string, d os.DirEntry, err error) error {
		if err == nil && d.Name() == "config.yaml" {
			configs = append(configs, path)
		}
		return err
	})
	if err != nil || len(configs) == 0 {
		t.Fatalf("no example configurations found: %v", err)
	}
	return configs
}

// TestExampleImagesBuild builds the image of every example, which catches
// install steps broken by upstream changes. It downloads a lot, so it only
// runs with ISODEV_EXAMPLES_TEST=1.
func TestExampleImagesBuild(t *testing.T) {
	if os.Getenv("ISODEV_EXAMPLES_TEST") != "1" {
		t.Skip("set ISODEV_EXAMPLES_TEST=1 to build the example images")
	}
	for _, example := range exampleConfigs(t) {
		dir := filepath.Dir(example)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(dir, "image", "Dockerfile")); os.IsNotExist(err) {
				t.Skip("no image/Dockerfile")
			}
			tag := "isodev-example-test-" + filepath.Base(dir)
			out, err := exec.Command("docker", "build", "--quiet", "--tag", tag, filepath.Join(dir, "image")).CombinedOutput()
			if err != nil {
				t.Fatalf("build %s: %v\n%s", dir, err, out)
			}
			_ = exec.Command("docker", "image", "rm", tag).Run()
		})
	}
}

// checkLoads creates the host files that the configuration refers to but that
// a documentation snippet cannot provide, then loads and validates it.
func checkLoads(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(paths.ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	var raw Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("invalid configuration: %v\n%s", err, data)
	}
	base := paths.ConfigDir()
	create := func(path, base string, dir bool) {
		p, err := expandPath(path, base)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(p); err == nil {
			return
		}
		if dir {
			err = os.MkdirAll(p, 0700)
		} else if err = os.MkdirAll(filepath.Dir(p), 0700); err == nil {
			err = os.WriteFile(p, nil, 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range raw.Volumes {
		if m.Type == MountBind {
			create(m.Source, base, true)
		}
	}
	for _, h := range append(append([]Script{}, raw.Lifecycle.OnStart.Scripts...), raw.Lifecycle.OnCreate.Scripts...) {
		if h.File != "" {
			create(h.File, base, false)
		}
	}
	if raw.Build != nil && raw.Build.Context != "" {
		create(raw.Build.Context, base, true)
		dockerfile := raw.Build.Dockerfile
		if dockerfile == "" {
			dockerfile = DefaultDockerfile
		}
		context, _ := expandPath(raw.Build.Context, base)
		create(dockerfile, context, false)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	if err := cfg.ValidateBuild(); err != nil {
		t.Fatal(err)
	}
}
