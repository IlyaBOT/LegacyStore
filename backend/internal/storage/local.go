package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidPath = errors.New("invalid storage path")
	ErrTooLarge    = errors.New("upload too large")
)

type SavedFile struct {
	RelativePath string
	SHA256       string
	SizeBytes    int64
}

type Local struct {
	root           string
	maxUploadBytes int64
}

func NewLocal(root string, maxUploadBytes int64) (*Local, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("local storage path is empty")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if maxUploadBytes <= 0 {
		maxUploadBytes = 8 << 30
	}
	for _, dir := range []string{"quarantine", "objects", "images"} {
		if err := os.MkdirAll(filepath.Join(absolute, dir), 0750); err != nil {
			return nil, err
		}
	}
	return &Local{root: absolute, maxUploadBytes: maxUploadBytes}, nil
}

func (s *Local) SaveQuarantine(reader io.Reader, originalName string) (*SavedFile, error) {
	if reader == nil {
		return nil, errors.New("nil upload reader")
	}
	extension := safeExtension(originalName)
	randomID, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	relative := filepath.ToSlash(filepath.Join("quarantine", randomID+extension))
	absolute, err := s.Resolve(relative)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(absolute)
		}
	}()

	hash := sha256.New()
	limited := &io.LimitedReader{R: reader, N: s.maxUploadBytes + 1}
	written, err := io.Copy(io.MultiWriter(file, hash), limited)
	if err != nil {
		return nil, err
	}
	if written > s.maxUploadBytes {
		return nil, ErrTooLarge
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	keep = true
	return &SavedFile{
		RelativePath: relative,
		SHA256:       hex.EncodeToString(hash.Sum(nil)),
		SizeBytes:    written,
	}, nil
}

func (s *Local) SaveImage(data []byte, extension, sha256Hex string) (*SavedFile, error) {
	if len(data) == 0 {
		return nil, errors.New("empty image")
	}
	extension = strings.ToLower(strings.TrimSpace(extension))
	if extension != ".jpg" && extension != ".png" && extension != ".svg" {
		return nil, ErrInvalidPath
	}
	sha256Hex = strings.ToLower(strings.TrimSpace(sha256Hex))
	if len(sha256Hex) != 64 {
		return nil, errors.New("invalid sha256")
	}
	for _, r := range sha256Hex {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return nil, errors.New("invalid sha256")
		}
	}

	relative := filepath.ToSlash(filepath.Join("images", sha256Hex[:2], sha256Hex+extension))
	absolute, err := s.Resolve(relative)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if errors.Is(err, os.ErrExist) {
		info, statErr := os.Stat(absolute)
		if statErr != nil {
			return nil, statErr
		}
		return &SavedFile{RelativePath: relative, SHA256: sha256Hex, SizeBytes: info.Size()}, nil
	}
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(absolute)
		}
	}()
	written, err := file.Write(data)
	if err != nil {
		return nil, err
	}
	if written != len(data) {
		return nil, io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	keep = true
	return &SavedFile{RelativePath: relative, SHA256: sha256Hex, SizeBytes: int64(written)}, nil
}

func (s *Local) Remove(relative string) error {
	absolute, err := s.Resolve(relative)
	if err != nil {
		return err
	}
	err = os.Remove(absolute)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Local) Open(relative string) (*os.File, error) {
	absolute, err := s.Resolve(relative)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(absolute)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, ErrInvalidPath
	}
	return file, nil
}

func (s *Local) Resolve(relative string) (string, error) {
	relative = strings.TrimSpace(relative)
	if relative == "" || filepath.IsAbs(relative) {
		return "", ErrInvalidPath
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}
	absolute := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, absolute)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}
	return absolute, nil
}

func safeExtension(name string) string {
	ext := strings.ToLower(filepath.Ext(filepath.Base(strings.TrimSpace(name))))
	if len(ext) > 12 {
		return ""
	}
	for _, r := range ext {
		if r == '.' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		return ""
	}
	return ext
}

func randomHex(bytesCount int) (string, error) {
	buffer := make([]byte, bytesCount)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("random storage id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
