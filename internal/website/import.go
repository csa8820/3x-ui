package website

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const MaxArchiveBytes = 25 << 20

func ImportTemplate(data []byte) (string, error) {
	id := "custom-" + uuid.NewString()[:8]
	return id, ExtractTemplate(data, id)
}

func ExtractTemplate(data []byte, id string) error {
	if len(data) > MaxArchiveBytes {
		return errors.New("template ZIP exceeds 25 MiB")
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	if len(r.File) > 2000 {
		return errors.New("template ZIP contains too many files")
	}
	if err := os.MkdirAll(TemplatesDir(), 0o700); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(TemplatesDir(), ".import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	var total uint64
	var indexes []string
	for _, file := range r.File {
		name := file.Name
		if strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.Contains(name, ":") || path.Clean(name) != strings.TrimSuffix(name, "/") || strings.HasPrefix(name, "../") || file.Mode()&os.ModeSymlink != 0 {
			return errors.New("template ZIP contains an unsafe path or symbolic link")
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > MaxArchiveBytes || total+file.UncompressedSize64 > 100<<20 {
			return errors.New("expanded template exceeds size limit")
		}
		total += file.UncompressedSize64
		target := filepath.Join(temp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = in.Close()
			return err
		}
		written, copyErr := io.Copy(out, io.LimitReader(in, MaxArchiveBytes+1))
		_ = in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written > MaxArchiveBytes {
			return errors.New("template file exceeds size limit")
		}
		if path.Base(name) == "index.html" {
			indexes = append(indexes, name)
		}
	}
	if len(indexes) == 0 {
		return errors.New("template ZIP must contain a built index.html")
	}
	sort.Slice(indexes, func(i, j int) bool { return strings.Count(indexes[i], "/") < strings.Count(indexes[j], "/") })
	index := indexes[0]
	for _, candidate := range indexes {
		if strings.HasSuffix(candidate, "/dist/index.html") {
			index = candidate
			break
		}
	}
	root := filepath.Dir(filepath.Join(temp, filepath.FromSlash(index)))
	// Retain upstream license notices when only the built dist subtree is deployed.
	for _, file := range r.File {
		if strings.EqualFold(path.Base(file.Name), "LICENSE") && filepath.Dir(filepath.Join(temp, filepath.FromSlash(file.Name))) != root {
			license, _ := os.ReadFile(filepath.Join(temp, filepath.FromSlash(file.Name)))
			if len(license) > 0 {
				_ = os.WriteFile(filepath.Join(root, "LICENSE"), license, 0o600)
			}
			break
		}
	}
	if _, err := os.Stat(TemplateRoot(id)); err == nil {
		return errors.New("template is already installed")
	}
	return os.Rename(root, TemplateRoot(id))
}
