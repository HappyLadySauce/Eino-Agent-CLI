package options

import (
	"strings"
	"testing"
)

// TestModelOptionsValidateRequiredFields ensures missing base_url and model are reported.
// TestModelOptionsValidateRequiredFields 确认缺失 base_url 与 model 时会报错。
func TestModelOptionsValidateRequiredFields(t *testing.T) {
	err := (&ModelOptions{}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want non-nil")
	}
	msg := err.Error()
	for _, want := range []string{"base_url is required", "model is required"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q:\n%s", want, msg)
		}
	}
}

// TestModelOptionsValidatePassesWithRequiredFields ensures valid options pass validation.
// TestModelOptionsValidatePassesWithRequiredFields 确认必填项齐全时校验通过。
func TestModelOptionsValidatePassesWithRequiredFields(t *testing.T) {
	o := &ModelOptions{
		BaseURL: "https://api.example.com",
		Model:   "gpt-4",
	}
	if err := o.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}
