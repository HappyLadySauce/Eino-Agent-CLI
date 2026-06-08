package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewSessionID creates a random session id suitable for audit correlation.
// NewSessionID 创建适合审计关联的随机会话 ID。
func NewSessionID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return "sess_" + hex.EncodeToString(bytes[:]), nil
}
