package network

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

type AutoHttpsConn struct {
	net.Conn

	firstBuf []byte
	bufStart int

	readRequestOnce sync.Once
}

func NewAutoHttpsConn(conn net.Conn) net.Conn {
	return &AutoHttpsConn{
		Conn: conn,
	}
}

func (c *AutoHttpsConn) readRequest() bool {
	c.firstBuf = make([]byte, 2048)
	n, err := c.Conn.Read(c.firstBuf)
	c.firstBuf = c.firstBuf[:n]
	if err != nil {
		return false
	}
	reader := bytes.NewReader(c.firstBuf)
	bufReader := bufio.NewReader(reader)
	request, err := http.ReadRequest(bufReader)
	if err != nil {
		return false
	}
	// Host 与 RequestURI 均来自客户端，直接拼入 Location 头会引发
	// HTTP 响应拆分（CRLF 注入）与开放重定向。这里只接受合法的 Host
	// 头部（不含控制字符、空白或路径分隔符），否则丢弃该请求。
	if !isSafeRedirectHost(request.Host) {
		return false
	}
	resp := http.Response{
		Header: http.Header{},
	}
	resp.StatusCode = http.StatusTemporaryRedirect
	location := fmt.Sprintf("https://%v%v", request.Host, request.RequestURI)
	resp.Header.Set("Location", location)
	resp.Write(c.Conn)
	c.Close()
	c.firstBuf = nil
	return true
}

// isSafeRedirectHost 判断 Host 头是否可安全用于重定向 Location。
// 拒绝任何包含 CR/LF、控制字符、空白的值，避免响应拆分；同时要求
// 形如 host 或 host:port，避免把任意 URI 当作 Host 注入。
func isSafeRedirectHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.ContainsAny(host, " \t\r\n\x00") {
		return false
	}
	// Host 不应包含路径/查询分隔符
	if strings.ContainsAny(host, "/?#") {
		return false
	}
	hostPart := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostPart = h
	}
	if hostPart == "" {
		return false
	}
	// 拒绝 IPv6 之外的方括号，以及任何剩余的控制字符
	if strings.ContainsAny(hostPart, "[] \t\r\n\x00") && !strings.HasPrefix(host, "[") {
		return false
	}
	for _, r := range hostPart {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func (c *AutoHttpsConn) Read(buf []byte) (int, error) {
	c.readRequestOnce.Do(func() {
		c.readRequest()
	})

	if c.firstBuf != nil {
		n := copy(buf, c.firstBuf[c.bufStart:])
		c.bufStart += n
		if c.bufStart >= len(c.firstBuf) {
			c.firstBuf = nil
		}
		return n, nil
	}

	return c.Conn.Read(buf)
}
