package agent

import "errors"

// ErrServiceMissing is returned when agent tools are built without a service.
// ErrServiceMissing 表示 Agent 工具缺少运行时服务。
var ErrServiceMissing = errors.New("agent tool service is not configured")
