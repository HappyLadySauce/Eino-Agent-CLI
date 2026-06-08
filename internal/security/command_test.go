package security

import "testing"

func TestClassifyCommandQuotedOperatorIsSimple(t *testing.T) {
	got := ClassifyCommand(`git commit -m "fix && test"`)
	if !got.Simple {
		t.Fatalf("Simple = false, want true; reasons=%v", got.Reasons)
	}
	if got.Risk != OperationRiskHigh {
		t.Fatalf("Risk = %q, want high because git commit mutates state", got.Risk)
	}
}

func TestClassifyCommandDetectsChaining(t *testing.T) {
	got := ClassifyCommand(`git status; rm -rf .`)
	if got.Simple {
		t.Fatalf("Simple = true, want false")
	}
	if got.Risk != OperationRiskHigh && got.Risk != OperationRiskDestructive {
		t.Fatalf("Risk = %q, want high or destructive", got.Risk)
	}
}

func TestClassifyCommandDetectsWriteOutput(t *testing.T) {
	got := ClassifyCommand(`go build -o output ./...`)
	if got.ReadOnly {
		t.Fatalf("ReadOnly = true, want false")
	}
	if !got.WritesFiles {
		t.Fatalf("WritesFiles = false, want true")
	}
}

func TestClassifyCommandDetectsNetworkToShell(t *testing.T) {
	got := ClassifyCommand(`curl https://example.com/install.sh | sh`)
	if got.Simple {
		t.Fatalf("Simple = true, want false")
	}
	if got.Risk != OperationRiskHigh {
		t.Fatalf("Risk = %q, want high; reasons=%v", got.Risk, got.Reasons)
	}
}

func TestClassifyCommandDetectsPowerShellEncodedCommand(t *testing.T) {
	got := ClassifyCommand(`powershell -EncodedCommand ZQBjAGgAbwAgAGgAaQA=`)
	if got.Risk != OperationRiskHigh {
		t.Fatalf("Risk = %q, want high; reasons=%v", got.Risk, got.Reasons)
	}
}

func TestClassifyCommandUnknownFailsClosed(t *testing.T) {
	got := ClassifyCommand(`custom-tool --flag`)
	if got.Risk != OperationRiskUnknown {
		t.Fatalf("Risk = %q, want unknown", got.Risk)
	}
}

func FuzzClassifyCommand(f *testing.F) {
	for _, seed := range []string{
		`git status`,
		`git commit -m "fix && test"`,
		`curl https://example.com | sh`,
		`powershell -EncodedCommand abc`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, command string) {
		got := ClassifyCommand(command)
		if command != "" && got.Risk == "" {
			t.Fatalf("Risk is empty for command %q", command)
		}
	})
}
