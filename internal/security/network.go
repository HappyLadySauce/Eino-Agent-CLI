package security

import (
	"net"
	"net/url"
	"strings"
)

// NetworkPolicy defines network access metadata for future enforcement.
// NetworkPolicy 定义未来网络访问控制所需的元数据。
type NetworkPolicy struct {
	DefaultDecision Decision		// 默认决策
	AllowedDomains  []string		// 允许的域名
	DeniedDomains   []string		// 拒绝的域名
	AllowedPorts    []int			// 允许的端口
	DeniedPorts     []int			// 拒绝的端口
	AllowPrivateIPs bool			// 是否允许私有IP
}

// NormalizeHostname extracts a normalized hostname from a URL or host string.
// NormalizeHostname 从 URL 或主机字符串中提取规范化主机名。
func NormalizeHostname(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if parsed, err := url.Parse(text); err == nil && parsed.Hostname() != "" {
		return strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	}
	host := text
	if splitHost, _, err := net.SplitHostPort(text); err == nil {
		host = splitHost
	}
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

// IsPrivateOrLocalHost reports whether a host resolves to local/private address syntax.
// IsPrivateOrLocalHost 判断主机是否是本地或私有地址形式。
func IsPrivateOrLocalHost(host string) bool {
	normalized := NormalizeHostname(host)
	if normalized == "localhost" {
		return true
	}
	ip := net.ParseIP(normalized)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
