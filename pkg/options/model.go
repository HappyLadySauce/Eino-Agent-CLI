package options

import (
	"errors"
	"strings"

	"github.com/spf13/pflag"
)

type ModelOptions struct {
	AuthToken       string `mapstructure:"EINO_AUTH_TOKEN"`
	BaseURL         string `mapstructure:"EINO_BASE_URL"`
	Model           string `mapstructure:"EINO_MODEL"`
	MaxOutputTokens int    `mapstructure:"EINO_MAX_OUTPUT_TOKENS"`
}

func NewModelOptions() *ModelOptions {
	return &ModelOptions{}
}

func (o *ModelOptions) Validate() error {
	var errs error

	o.BaseURL = normalizeBaseURL(o.BaseURL)

	if o.BaseURL == "" {
		errs = errors.Join(errs, errors.New("base_url is required"))
	}
	if o.Model == "" {
		errs = errors.Join(errs, errors.New("model is required"))
	}

	return errs
}

// normalizeBaseURL ensures the model endpoint has an explicit URL scheme.
// Ollama-style host:port values are normalized to http:// by default.
// normalizeBaseURL 确保模型端点带有明确的 URL scheme；类似 Ollama 的 host:port 默认补全为 http://。
func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		return strings.TrimRight(raw, "/")
	}
	return "http://" + strings.TrimRight(raw, "/")
}

func (o *ModelOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.AuthToken, "auth-token", "", "The authentication token for the model")
	fs.StringVar(&o.BaseURL, "base-url", "", "The base URL for the model")
	fs.StringVar(&o.Model, "model", "", "The model to use")
	fs.IntVar(&o.MaxOutputTokens, "max-output-tokens", 32000, "The maximum number of output tokens")
}
