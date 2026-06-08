package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolvedPath is the canonical path view used for boundary checks.
// ResolvedPath 是用于边界检查的规范化路径视图。
type ResolvedPath struct {
	WorkspaceRoot string // 工作区根目录
	Requested     string // 请求路径
	Absolute      string // 绝对路径
	Existing      bool   // 是否存在
}

// ResolveWorkspacePath resolves a requested path and verifies workspace containment.
// ResolveWorkspacePath 解析请求路径并校验其是否位于工作区内。
func ResolveWorkspacePath(workspaceRoot, requested string, operation OperationKind) (ResolvedPath, error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return ResolvedPath{}, errors.New("workspace root is required")
	}
	if strings.TrimSpace(requested) == "" {
		return ResolvedPath{}, errors.New("path is required")
	}
	root, err := canonicalExistingPath(workspaceRoot)
	if err != nil {
		return ResolvedPath{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	if err := rejectUnsupportedWindowsPath(requested); err != nil {
		return ResolvedPath{}, err
	}
	if err := rejectWindowsReservedPath(requested); err != nil {
		return ResolvedPath{}, err
	}

	absolute := requested
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(root, absolute)
	}
	absolute, err = filepath.Abs(absolute)
	if err != nil {
		return ResolvedPath{}, fmt.Errorf("make path absolute: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if err := rejectUnsupportedWindowsPath(absolute); err != nil {
		return ResolvedPath{}, err
	}
	if err := rejectWindowsReservedPath(absolute); err != nil {
		return ResolvedPath{}, err
	}

	resolved, existing, err := canonicalTargetPath(absolute)
	if err != nil {
		return ResolvedPath{}, err
	}
	if err := ensureContained(root, resolved); err != nil {
		return ResolvedPath{}, err
	}
	if isDestructiveOperation(operation) && samePath(root, resolved) {
		return ResolvedPath{}, errors.New("destructive operation against workspace root is denied")
	}
	return ResolvedPath{
		WorkspaceRoot: root,
		Requested:     requested,
		Absolute:      resolved,
		Existing:      existing,
	}, nil
}

// ValidateDestructiveTarget rejects broad destructive targets.
// ValidateDestructiveTarget 拒绝宽泛的破坏性目标。
func ValidateDestructiveTarget(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("target path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("make target absolute: %w", err)
	}
	abs = filepath.Clean(abs)
	if samePath(abs, filepath.VolumeName(abs)+string(filepath.Separator)) || samePath(abs, string(filepath.Separator)) {
		return errors.New("destructive operation against filesystem root is denied")
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" && samePath(abs, filepath.Clean(home)) {
		return errors.New("destructive operation against home directory is denied")
	}
	return nil
}

func canonicalExistingPath(path string) (string, error) {
	if err := rejectUnsupportedWindowsPath(path); err != nil {
		return "", err
	}
	if err := rejectWindowsReservedPath(path); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	evaluated, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(evaluated), nil
}

func canonicalTargetPath(abs string) (string, bool, error) {
	if evaluated, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(evaluated), true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("evaluate target path: %w", err)
	}

	parent := filepath.Dir(abs)
	base := filepath.Base(abs)
	for {
		evaluatedParent, err := filepath.EvalSymlinks(parent)
		if err == nil {
			return filepath.Clean(filepath.Join(evaluatedParent, base)), false, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", false, fmt.Errorf("evaluate parent path: %w", err)
		}
		nextParent := filepath.Dir(parent)
		base = filepath.Join(filepath.Base(parent), base)
		if nextParent == parent {
			return "", false, errors.New("no existing parent path found")
		}
		parent = nextParent
	}
}

func ensureContained(root, target string) error {
	if samePath(root, target) {
		return nil
	}
	if !sameVolume(root, target) {
		return fmt.Errorf("path %q is outside workspace volume", target)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("calculate relative path: %w", err)
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path %q escapes workspace root %q", target, root)
	}
	return nil
}

func sameVolume(a, b string) bool {
	volumeA := normalizePathForCompare(filepath.VolumeName(a))
	volumeB := normalizePathForCompare(filepath.VolumeName(b))
	return volumeA == volumeB
}

func samePath(a, b string) bool {
	return normalizePathForCompare(filepath.Clean(a)) == normalizePathForCompare(filepath.Clean(b))
}

func normalizePathForCompare(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
		path = strings.ReplaceAll(path, "/", `\`)
	}
	return path
}

func isDestructiveOperation(operation OperationKind) bool {
	return operation == OperationDelete || operation == OperationWrite
}

func rejectUnsupportedWindowsPath(path string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	trimmed := strings.TrimSpace(path)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, `\\?\`) || strings.HasPrefix(lower, `\\.\`) {
		return fmt.Errorf("windows device path %q is not supported", path)
	}
	withoutVolume := strings.TrimPrefix(trimmed, filepath.VolumeName(trimmed))
	if strings.Contains(withoutVolume, ":") {
		return fmt.Errorf("windows alternate data stream path %q is not supported", path)
	}
	return nil
}

func rejectWindowsReservedPath(path string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	cleaned := filepath.Clean(path)
	for _, segment := range strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == '\\' || r == '/'
	}) {
		if segment == "" || strings.HasSuffix(segment, ":") {
			continue
		}
		if isWindowsReservedName(segment) {
			return fmt.Errorf("windows reserved path segment %q is not supported", segment)
		}
	}
	return nil
}

func isWindowsReservedName(segment string) bool {
	name := strings.TrimRight(segment, " .")
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		name = name[:dot]
	}
	name = strings.ToUpper(name)
	switch name {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(name) == 4 {
		prefix := name[:3]
		suffix := name[3]
		return (prefix == "COM" || prefix == "LPT") && suffix >= '1' && suffix <= '9'
	}
	return false
}
