package securitytests

import (
	"net"
	"testing"
	"x-ui/web/entity"
)

// TestResolveClientIPNoTrustedProxies 验证：未配置受信代理时，
// 任何 X-Forwarded-For 头都被忽略，只返回 TCP 直连对端 IP。
func TestResolveClientIPNoTrustedProxies(t *testing.T) {
	got := entity.ResolveClientIP("203.0.113.5:12345", nil, "198.51.100.1")
	if got != "203.0.113.5" {
		t.Fatalf("expected direct peer IP, got %q", got)
	}
}

// TestResolveClientIPUntrustedPeer 验证：直连对端不在受信代理列表内时，
// 即使配置了受信代理，XFF 也不被信任，返回直连对端 IP。
func TestResolveClientIPUntrustedPeer(t *testing.T) {
	trusted := []*net.IPNet{parseCIDR(t, "10.0.0.0/8")}
	got := entity.ResolveClientIP("203.0.113.5:12345", trusted, "198.51.100.1")
	if got != "203.0.113.5" {
		t.Fatalf("expected direct peer IP for untrusted proxy, got %q", got)
	}
}

// TestResolveClientIPTrustedProxyChain 验证：直连对端是受信代理时，
// 从 XFF 链右侧跳过受信代理，返回第一个非可信地址作为真实客户端。
func TestResolveClientIPTrustedProxyChain(t *testing.T) {
	trusted := []*net.IPNet{parseCIDR(t, "10.0.0.0/8")}
	xff := "198.51.100.7, 10.0.0.1, 10.0.0.2"
	got := entity.ResolveClientIP("10.0.0.2:54321", trusted, xff)
	if got != "198.51.100.7" {
		t.Fatalf("expected real client 198.51.100.7, got %q", got)
	}
}

// TestResolveClientIPAllProxiesTrusted 验证：XFF 全是受信代理时
// 回退到直连对端 IP，避免返回空值。
func TestResolveClientIPAllProxiesTrusted(t *testing.T) {
	trusted := []*net.IPNet{parseCIDR(t, "10.0.0.0/8")}
	xff := "10.0.0.1, 10.0.0.2"
	got := entity.ResolveClientIP("10.0.0.2:54321", trusted, xff)
	if got != "10.0.0.2" {
		t.Fatalf("expected fallback to peer IP, got %q", got)
	}
}

// TestParseTrustedProxies 验证逗号分隔的 IP/CIDR 列表解析，
// 非法条目被静默跳过（解析器容错，校验在 CheckValid 中完成）。
func TestParseTrustedProxies(t *testing.T) {
	got := entity.ParseTrustedProxies("10.0.0.0/8, 172.16.0.1, garbage, 192.168.0.0/16")
	if len(got) != 3 {
		t.Fatalf("expected 3 valid entries, got %d (%v)", len(got), got)
	}
	if !got[0].Contains(net.ParseIP("10.5.5.5")) {
		t.Fatalf("first entry should cover 10.0.0.0/8")
	}
}

func parseCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("parse cidr %s: %v", s, err)
	}
	return n
}
