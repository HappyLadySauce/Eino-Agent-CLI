package security

import (
	"path/filepath"
	"runtime"
	"strings"
)

var sensitivePathNames = map[string]bool{
	".env":                                 true,
	".npmrc":                               true,
	".pypirc":                              true,
	".netrc":                               true,
	"credentials":                          true,
	"config":                               true,
	"kubeconfig":                           true,
	"id_rsa":                               true,
	"id_dsa":                               true,
	"id_ecdsa":                             true,
	"id_ed25519":                           true,
	"known_hosts":                          false,
	"authorized_keys":                      true,
	"dockerconfigjson":                     true,
	"service-account":                      true,
	"service_account":                      true,
	"application_default_credentials.json": true,
}

// IsSensitivePath reports whether a path is a known secret-bearing location.
// IsSensitivePath 判断路径是否属于已知敏感凭据位置。
func IsSensitivePath(path string) bool {
	normalized := normalizeSensitivePath(path)
	base := filepath.Base(normalized)
	if sensitivePathNames[base] {
		return true
	}
	segments := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	for i, segment := range segments {
		if segment == ".ssh" && i+1 < len(segments) && strings.HasPrefix(segments[i+1], "id_") {
			return true
		}
		if segment == ".aws" && i+1 < len(segments) && (segments[i+1] == "credentials" || segments[i+1] == "config") {
			return true
		}
		if segment == ".kube" && i+1 < len(segments) && segments[i+1] == "config" {
			return true
		}
		if segment == ".docker" && i+1 < len(segments) && segments[i+1] == "config.json" {
			return true
		}
	}
	return false
}

func normalizeSensitivePath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return cleaned
}
