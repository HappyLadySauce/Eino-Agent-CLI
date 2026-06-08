package security

import "testing"

func TestNormalizeHostname(t *testing.T) {
	if got, want := NormalizeHostname("https://Example.COM:443/path"), "example.com"; got != want {
		t.Fatalf("NormalizeHostname() = %q, want %q", got, want)
	}
}

func TestIsPrivateOrLocalHost(t *testing.T) {
	tests := []string{"localhost", "127.0.0.1", "10.0.0.1", "169.254.169.254"}
	for _, host := range tests {
		if !IsPrivateOrLocalHost(host) {
			t.Fatalf("IsPrivateOrLocalHost(%q) = false, want true", host)
		}
	}
	if IsPrivateOrLocalHost("example.com") {
		t.Fatalf("IsPrivateOrLocalHost(example.com) = true, want false")
	}
}
