package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

const blockManifestVersion = 1

type blockManifest struct {
	Version      int               `json:"version"`
	Key          string            `json:"key"`
	Procedure    string            `json:"procedure"`
	InputDigests map[string]string `json:"input_digests"`
	OutputDigest string            `json:"output_digest"`
	Files        []string          `json:"files"`
}

type digestSidecar struct {
	Version  int    `json:"version"`
	Digest   string `json:"digest"`
	Metadata string `json:"metadata"`
}

func blockCacheKey(procedure Procedure) (string, map[string]string, error) {
	digests := make(map[string]string, len(procedure.Inputs))
	inputs := append([]string(nil), procedure.Inputs...)
	sort.Strings(inputs)
	hash := sha256.New()
	_, _ = io.WriteString(hash, procedure.Fingerprint)
	for _, input := range inputs {
		digest, err := treeDigest(input)
		if err != nil {
			return "", nil, fmt.Errorf("hash input %s: %w", input, err)
		}
		digests[input] = digest
		_, _ = io.WriteString(hash, "\x00"+input+"\x00"+digest)
	}
	return hex.EncodeToString(hash.Sum(nil)), digests, nil
}

func restoreBlockCache(root, key, output string) (bool, error) {
	entry := filepath.Join(root, ".staging", "cache", key)
	data, err := os.ReadFile(filepath.Join(entry, "manifest.json"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var manifest blockManifest
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version != blockManifestVersion || manifest.Key != key {
		return false, nil
	}
	cachedOutput := filepath.Join(entry, "output")
	digest, err := treeDigest(cachedOutput)
	if err != nil || digest != manifest.OutputDigest {
		return false, nil
	}
	if err := os.RemoveAll(output); err != nil {
		return false, err
	}
	if err := copyTree(cachedOutput, output); err != nil {
		return false, err
	}
	if err := writeDigestSidecar(output, manifest.OutputDigest); err != nil {
		return false, err
	}
	return true, nil
}

func storeBlockCache(root, key string, procedure Procedure, inputs map[string]string, output string) error {
	info, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("cache output %s: %w", output, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("cache output must be a directory: %s", output)
	}
	digest, err := treeDigest(output)
	if err != nil {
		return err
	}
	files, err := treeFiles(output)
	if err != nil {
		return err
	}
	if err := writeDigestSidecar(output, digest); err != nil {
		return err
	}
	cacheRoot := filepath.Join(root, ".staging", "cache")
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(cacheRoot, ".block-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err := copyTree(output, filepath.Join(temp, "output")); err != nil {
		return err
	}
	manifest := blockManifest{
		Version: blockManifestVersion, Key: key, Procedure: procedure.ID,
		InputDigests: inputs, OutputDigest: digest, Files: files,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temp, "manifest.json"), data, 0o644); err != nil {
		return err
	}
	destination := filepath.Join(cacheRoot, key)
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return os.Rename(temp, destination)
}

func treeDigest(root string) (string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		metadata, err := treeMetadataDigest(root)
		if err != nil {
			return "", err
		}
		var sidecar digestSidecar
		data, readErr := os.ReadFile(root + ".svalbard-manifest.json")
		if readErr == nil && json.Unmarshal(data, &sidecar) == nil &&
			sidecar.Version == blockManifestVersion && sidecar.Metadata == metadata {
			return sidecar.Digest, nil
		}
	}
	hash := sha256.New()
	if !info.IsDir() {
		if err := hashPath(hash, root, filepath.Base(root), info); err != nil {
			return "", err
		}
		return hex.EncodeToString(hash.Sum(nil)), nil
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return hashPath(hash, path, filepath.ToSlash(relative), info)
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeDigestSidecar(root, digest string) error {
	metadata, err := treeMetadataDigest(root)
	if err != nil {
		return err
	}
	data, err := json.Marshal(digestSidecar{Version: blockManifestVersion, Digest: digest, Metadata: metadata})
	if err != nil {
		return err
	}
	return os.WriteFile(root+".svalbard-manifest.json", data, 0o644)
}

func treeMetadataDigest(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(hash, "%s\\x00%d\\x00%d\\x00%s\\x00", filepath.ToSlash(relative), info.Size(), info.ModTime().UnixNano(), info.Mode())
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashPath(hash io.Writer, path, relative string, info fs.FileInfo) error {
	_, _ = io.WriteString(hash, relative+"\x00"+info.Mode().String()+"\x00")
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(hash, target)
		return nil
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(hash, file)
	return err
}

func treeFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	return files, err
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputErr := input.Close()
		outputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		return outputErr
	})
}
