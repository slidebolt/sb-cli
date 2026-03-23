package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	storage "github.com/slidebolt/sb-storage-sdk"
)

type workspaceKey string

func (k workspaceKey) Key() string { return string(k) }

type workspaceTarget struct {
	target storage.StorageTarget
	suffix string
}

var workspaceTargets = []workspaceTarget{
	{target: storage.Profile, suffix: ".profile.json"},
	{target: storage.Private, suffix: ".private.json"},
	{target: storage.Internal, suffix: ".internal.json"},
	{target: storage.State, suffix: ".json"},
	{target: storage.Source, suffix: ".lua"},
}

func pullWorkspace(store storage.Storage, outDir, pattern string) error {
	if outDir == "" {
		return fmt.Errorf("storage pull: out dir is required")
	}
	if pattern == "" {
		pattern = ">"
	}
	for _, target := range workspaceTargets {
		entries, err := store.SearchFiles(target.target, pattern)
		if err != nil {
			return fmt.Errorf("storage pull: %s: %w", target.target, err)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
		for _, entry := range entries {
			path := workspacePath(outDir, entry.Key, target.suffix)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			body := entry.Data
			if target.target == storage.Source {
				var source string
				if err := json.Unmarshal(entry.Data, &source); err != nil {
					return fmt.Errorf("storage pull: parse source %s: %w", entry.Key, err)
				}
				body = []byte(source)
			}
			if err := os.WriteFile(path, body, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func pushWorkspace(store storage.Storage, srcDir string) error {
	if srcDir == "" {
		return fmt.Errorf("storage push: src dir is required")
	}
	files, err := readWorkspaceFiles(srcDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := store.WriteFile(file.target, workspaceKey(file.key), file.data); err != nil {
			return err
		}
	}
	return nil
}

func deleteWorkspace(store storage.Storage, pattern string) error {
	if pattern == "" {
		return fmt.Errorf("storage delete: pattern is required")
	}
	keys := map[string]struct{}{}
	for _, target := range workspaceTargets {
		entries, err := store.SearchFiles(target.target, pattern)
		if err != nil {
			return fmt.Errorf("storage delete: %s: %w", target.target, err)
		}
		for _, entry := range entries {
			keys[entry.Key] = struct{}{}
		}
	}
	all := make([]string, 0, len(keys))
	for key := range keys {
		all = append(all, key)
	}
	sort.Strings(all)
	for _, key := range all {
		if err := store.Delete(workspaceKey(key)); err != nil {
			return err
		}
	}
	return nil
}

type workspaceFile struct {
	key    string
	target storage.StorageTarget
	data   json.RawMessage
}

func readWorkspaceFiles(root string) ([]workspaceFile, error) {
	var out []workspaceFile
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		key, target, err := workspaceFileTarget(root, path)
		if err != nil {
			return err
		}
		if target == "" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		data := json.RawMessage(body)
		if target == storage.Source {
			encoded, err := json.Marshal(string(body))
			if err != nil {
				return err
			}
			data = encoded
		}
		out = append(out, workspaceFile{key: key, target: target, data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].key == out[j].key {
			return out[i].target < out[j].target
		}
		return out[i].key < out[j].key
	})
	return out, nil
}

func workspacePath(root, key, suffix string) string {
	parts := strings.Split(key, ".")
	elems := append([]string{root}, parts...)
	return filepath.Join(filepath.Join(elems...), parts[len(parts)-1]+suffix)
}

func workspaceFileTarget(root, path string) (string, storage.StorageTarget, error) {
	for _, target := range workspaceTargets {
		if !strings.HasSuffix(path, target.suffix) {
			continue
		}
		key, err := workspaceKeyFromPath(root, path, target.suffix)
		return key, target.target, err
	}
	return "", "", nil
}

func workspaceKeyFromPath(root, path, suffix string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("storage push: invalid path: %s", path)
	}
	trimmed := strings.TrimSuffix(rel, suffix)
	if trimmed == rel || trimmed == "" {
		return "", fmt.Errorf("storage push: invalid workspace path: %s", path)
	}
	parts := strings.Split(trimmed, string(filepath.Separator))
	if len(parts) < 2 || parts[len(parts)-1] != parts[len(parts)-2] {
		return "", fmt.Errorf("storage push: invalid storage layout: %s", path)
	}
	parts = parts[:len(parts)-1]
	return strings.Join(parts, "."), nil
}
