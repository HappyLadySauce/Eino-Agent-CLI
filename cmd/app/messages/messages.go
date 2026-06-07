package messages

import (
	"github.com/cloudwego/eino/schema"
)

type Chat struct {
	Messages []*schema.Message
	Count	int
}
