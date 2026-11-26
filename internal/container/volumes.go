package container

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jakubdyszkiewicz/isodev/internal/config"
	"github.com/jakubdyszkiewicz/isodev/internal/shell"
)

var volumeNameUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// privateVolumeName names the private volume mounted at target in container,
// for example isodev-<hash>-var-lib-docker. The name is stable, so a recreated
// container gets the same volume.
func privateVolumeName(container, target string) string {
	return container + "-" + strings.Trim(volumeNameUnsafe.ReplaceAllString(strings.Trim(target, "/"), "-"), "-")
}

// volumeSource returns the Docker volume mounted for m in r's container.
func volumeSource(r Record, m config.Mount) string {
	if m.Source != "" {
		return m.Source
	}
	return privateVolumeName(r.Container, m.Target)
}

// privateVolumes returns the names of r's private volumes in cfg.
func privateVolumes(r Record, cfg config.Config) []string {
	var names []string
	for _, m := range cfg.Volumes {
		if m.Type == config.MountVolume && m.Source == "" {
			names = append(names, privateVolumeName(r.Container, m.Target))
		}
	}
	return names
}

// ensureVolumes creates r's private volumes that don't exist, labelled with
// the workspace, and checks that existing ones belong to it. It returns the
// volumes it created.
func ensureVolumes(r Record, cfg config.Config) ([]string, error) {
	var created []string
	for _, name := range privateVolumes(r, cfg) {
		out, err := shell.Run("docker", "volume", "inspect", name)
		if err != nil {
			if _, err := shell.Run("docker", append(append([]string{"volume", "create"}, managedLabels(r.Cwd)...), name)...); err != nil {
				return created, err
			}
			created = append(created, name)
			continue
		}
		var volumes []struct{ Labels map[string]string }
		if err := json.Unmarshal([]byte(out), &volumes); err != nil {
			return created, err
		}
		if len(volumes) != 1 || volumes[0].Labels[labelManaged] != "true" || volumes[0].Labels[labelWorkspace] != r.Cwd {
			return created, fmt.Errorf("existing volume %s belongs to another workspace", name)
		}
	}
	return created, nil
}

// removeVolumes removes the named volumes.
func removeVolumes(names []string) error {
	for _, name := range names {
		if _, err := shell.Run("docker", "volume", "rm", name); err != nil {
			return err
		}
	}
	return nil
}

func workspaceVolumeArgs(cwd string) []string {
	return []string{"volume", "ls", "--quiet", "--filter", "label=" + labelManaged + "=true", "--filter", "label=" + labelWorkspace + "=" + cwd}
}

// removeWorkspaceVolumes removes all private volumes of the workspace at cwd,
// including ones no longer in the configuration.
func removeWorkspaceVolumes(cwd string) error {
	out, err := shell.Run("docker", workspaceVolumeArgs(cwd)...)
	if err != nil {
		return err
	}
	return removeVolumes(strings.Fields(out))
}
