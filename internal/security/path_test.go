package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveWorkspacePathAllowsInsidePath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "dir", "file.txt")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(file, []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	resolved, err := ResolveWorkspacePath(root, "dir/file.txt", OperationRead)
	if err != nil {
		t.Fatalf("ResolveWorkspacePath() error = %v", err)
	}
	if !resolved.Existing {
		t.Fatalf("Existing = false, want true")
	}
	if !samePath(resolved.Absolute, file) {
		t.Fatalf("Absolute = %q, want %q", resolved.Absolute, file)
	}
}

func TestResolveWorkspacePathRejectsDotDotEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	_, err := ResolveWorkspacePath(root, filepath.Join("..", filepath.Base(outside)), OperationRead)
	if err == nil || !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("ResolveWorkspacePath(.. escape) error = %v, want workspace escape", err)
	}
}

func TestResolveWorkspacePathRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require Windows developer mode or privileges")
	}
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("Symlink() unavailable: %v", err)
	}
	_, err := ResolveWorkspacePath(root, filepath.Join("link", "secret.txt"), OperationRead)
	if err == nil || !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("ResolveWorkspacePath(symlink escape) error = %v, want workspace escape", err)
	}
}

func TestResolveWorkspacePathRejectsWorkspaceRootDelete(t *testing.T) {
	root := t.TempDir()
	_, err := ResolveWorkspacePath(root, root, OperationDelete)
	if err == nil || !strings.Contains(err.Error(), "workspace root") {
		t.Fatalf("ResolveWorkspacePath(root delete) error = %v, want workspace root denial", err)
	}
}

func TestWindowsPathRejections(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path validation")
	}
	root := t.TempDir()
	tests := []string{
		`\\?\C:\temp\file.txt`,
		`\\.\C:\temp\file.txt`,
		`COM1.txt`,
		`safe.txt:secret`,
	}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			_, err := ResolveWorkspacePath(root, path, OperationRead)
			if err == nil {
				t.Fatalf("ResolveWorkspacePath(%q) error = nil, want rejection", path)
			}
		})
	}
}

func TestValidateDestructiveTargetRejectsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("home directory unavailable")
	}
	err = ValidateDestructiveTarget(home)
	if err == nil || !strings.Contains(err.Error(), "home directory") {
		t.Fatalf("ValidateDestructiveTarget(home) error = %v, want home denial", err)
	}
}
