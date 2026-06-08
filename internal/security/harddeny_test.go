package security

import "testing"

func TestIsSensitivePath(t *testing.T) {
	tests := []string{
		`.env`,
		`.ssh/id_ed25519`,
		`.aws/credentials`,
		`.kube/config`,
		`.docker/config.json`,
	}
	for _, path := range tests {
		if !IsSensitivePath(path) {
			t.Fatalf("IsSensitivePath(%q) = false, want true", path)
		}
	}
	if IsSensitivePath(`src/main.go`) {
		t.Fatalf("IsSensitivePath(src/main.go) = true, want false")
	}
}
