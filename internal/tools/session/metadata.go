package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/HappyLadySauce/Eino-Agent-CLI/internal/security"
)

// SaveMetadata writes a minimal session metadata file.
// SaveMetadata 写入最小会话元数据文件。
func SaveMetadata(secCtx security.Context) error {
	dir := filepath.Join(secCtx.DataDir, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create sessions directory: %w", err)
	}
	payload := Metadata{
		SessionID: secCtx.SessionID,
		Mode:      string(secCtx.SessionMode),
		Sandbox:   string(secCtx.SandboxMode),
		Approval:  string(secCtx.ApprovalMode),
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session metadata: %w", err)
	}
	path := filepath.Join(dir, secCtx.SessionID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write session metadata: %w", err)
	}
	return nil
}

// LoadLatestMetadata reads the newest persisted session metadata.
// LoadLatestMetadata 读取最新持久化会话元数据。
func LoadLatestMetadata(dataDir string) (*Metadata, error) {
	dir := filepath.Join(dataDir, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list session metadata: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		infoI, errI := entries[i].Info()
		infoJ, errJ := entries[j].Info()
		if errI != nil || errJ != nil {
			return entries[i].Name() > entries[j].Name()
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read session metadata: %w", err)
		}
		var metadata Metadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("decode session metadata: %w", err)
		}
		return &metadata, nil
	}
	return nil, nil
}
