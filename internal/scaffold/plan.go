// Package scaffold plans safe, deterministic source generation.
package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Change struct {
	Path         string
	Data, Before []byte
	Update       bool
}
type Plan struct {
	Root    string
	Changes []Change
	New     bool
}

func (p *Plan) validate() error {
	root, err := filepath.Abs(p.Root)
	if err != nil {
		return err
	}
	p.Root = root
	if p.New {
		if _, e := os.Lstat(root); !errors.Is(e, os.ErrNotExist) {
			return fmt.Errorf("project destination already exists: %s", root)
		}
	}
	seen := map[string]bool{}
	for _, c := range p.Changes {
		clean := filepath.Clean(c.Path)
		if filepath.VolumeName(clean) != "" || strings.ContainsAny(clean, ":\x00") {
			return fmt.Errorf("unsafe generated path %q", c.Path)
		}
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe generated path %q", c.Path)
		}
		key := strings.ToLower(clean)
		if seen[key] {
			return fmt.Errorf("duplicate generated path %s", c.Path)
		}
		seen[key] = true
		path := filepath.Join(root, clean)
		for parent := path; ; parent = filepath.Dir(parent) {
			info, e := os.Lstat(parent)
			if e == nil && info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing symlink path %s", parent)
			}
			if e != nil && !errors.Is(e, os.ErrNotExist) {
				return e
			}
			if parent == filepath.Dir(parent) {
				break
			}
		}
		before, e := os.ReadFile(path)
		if c.Update {
			if e != nil || !bytes.Equal(before, c.Before) {
				return fmt.Errorf("generated file changed; preserve it and resolve conflict: %s", c.Path)
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return fmt.Errorf("destination exists: %s", c.Path)
		}
	}
	return nil
}

// Apply validates the whole plan before staging any changes. Updates require
// exact expected contents; handwritten files are never update destinations.
func (p *Plan) Apply(dry bool) (err error) {
	if err = p.validate(); err != nil {
		return err
	}
	if dry {
		return nil
	}
	if p.New {
		if err = os.MkdirAll(filepath.Dir(p.Root), 0755); err != nil {
			return err
		}
		stage, e := os.MkdirTemp(filepath.Dir(p.Root), ".gonex-stage-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(stage)
		for _, c := range p.Changes {
			path := filepath.Join(stage, c.Path)
			if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err = os.WriteFile(path, c.Data, 0644); err != nil {
				return err
			}
		}
		return os.Rename(stage, p.Root)
	}
	lock, e := os.OpenFile(filepath.Join(p.Root, ".gonex.lock"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return fmt.Errorf("another generator may be running; inspect .gonex.lock: %w", e)
	}
	lock.Close()
	defer os.Remove(filepath.Join(p.Root, ".gonex.lock"))
	if err = p.validate(); err != nil {
		return err
	}
	stage, e := os.MkdirTemp(p.Root, ".gonex-stage-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	for i, c := range p.Changes {
		if err = os.WriteFile(filepath.Join(stage, fmt.Sprint(i)), c.Data, 0644); err != nil {
			return err
		}
	}
	applied := []Change{}
	dirs := []string{}
	defer func() {
		if err == nil {
			return
		}
		for i := len(applied) - 1; i >= 0; i-- {
			c := applied[i]
			if c.Update {
				err = errors.Join(err, os.WriteFile(filepath.Join(p.Root, c.Path), c.Before, 0644))
			} else {
				err = errors.Join(err, os.Remove(filepath.Join(p.Root, c.Path)))
			}
		}
		for i := len(dirs) - 1; i >= 0; i-- {
			_ = os.Remove(dirs[i])
		}
	}()
	for i, c := range p.Changes {
		path := filepath.Join(p.Root, c.Path)
		missing := []string{}
		for d := filepath.Dir(path); d != p.Root; d = filepath.Dir(d) {
			if _, e = os.Stat(d); errors.Is(e, os.ErrNotExist) {
				missing = append(missing, d)
			}
		}
		for j := len(missing) - 1; j >= 0; j-- {
			if err = os.Mkdir(missing[j], 0755); err != nil {
				return err
			}
			dirs = append(dirs, missing[j])
		}
		if c.Update {
			if err = os.Rename(filepath.Join(stage, fmt.Sprint(i)), path); err != nil {
				return err
			}
		} else {
			var f *os.File
			f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				return err
			}
			applied = append(applied, c)
			_, err = f.Write(c.Data)
			err = errors.Join(err, f.Close())
			if err != nil {
				return err
			}
			continue
		}
		applied = append(applied, c)
	}
	return nil
}
