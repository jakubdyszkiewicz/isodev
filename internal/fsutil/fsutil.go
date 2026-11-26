// Package fsutil provides small filesystem helpers.
package fsutil

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Real returns the absolute path with symbolic links resolved.
func Real(p string) (string, error) {
	a, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(a)
}

// RealOrAbs is Real, or the absolute path if p no longer exists, so that
// resources of a deleted directory can still be found.
func RealOrAbs(p string) (string, error) {
	r, err := Real(p)
	if os.IsNotExist(err) {
		return filepath.Abs(p)
	}
	return r, err
}

// ReadJSON decodes the JSON file at p into v.
func ReadJSON(p string, v any) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// WriteJSON writes v as indented JSON readable only by the owner.
func WriteJSON(p string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0600)
}

// WriteJSONAtomic writes v through a temporary file and renames it into place.
func WriteJSONAtomic(p string, v any) error {
	tmp := p + ".tmp"
	if err := WriteJSON(tmp, v); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
