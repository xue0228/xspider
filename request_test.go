// request_test.go
package xspider

import (
	"bytes"
	"net/http"
	"net/url"
	"testing"
	"xspider/container"
)

func TestNewRequest(t *testing.T) {
	// 测试正常创建Request
	req, err := NewRequest("https://example.com")
	if err != nil {
		t.Errorf("NewRequest failed: %v", err)
	}

	if req.Url.String() != "https://example.com" {
		t.Errorf("Expected URL https://example.com, got %s", req.Url.String())
	}

	if req.Method != "GET" {
		t.Errorf("Expected method GET, got %s", req.Method)
	}

	// 测试无效URL
	_, err = NewRequest("://invalid")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

func TestRequestWithOptions(t *testing.T) {
	headers := http.Header{}
	headers.Set("User-Agent", "test-agent")

	cookies := []*http.Cookie{
		{Name: "session", Value: "12345"},
	}

	body := bytes.NewBufferString("test body")

	req, err := NewRequest(
		"https://example.com",
		WithMethod("POST"),
		WithHeaders(headers),
		WithCookies(cookies),
		WithBody(body),
		WithEncoding("gbk"),
		WithPriority(10),
		WithDontFilter(true),
		WithErrback("errorHandler"),
		WithCallback("responseHandler"),
	)

	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	if req.Method != "POST" {
		t.Errorf("Expected method POST, got %s", req.Method)
	}

	if req.Encoding != "gbk" {
		t.Errorf("Expected encoding gbk, got %s", req.Encoding)
	}

	if req.Priority != 10 {
		t.Errorf("Expected priority 10, got %d", req.Priority)
	}

	if req.DontFilter != true {
		t.Errorf("Expected DontFilter true, got %v", req.DontFilter)
	}

	if req.Errback != "errorHandler" {
		t.Errorf("Expected errback errorHandler, got %s", req.Errback)
	}

	if req.Callback != "responseHandler" {
		t.Errorf("Expected callback responseHandler, got %s", req.Callback)
	}
}

func TestDomain(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://www.example.com", "example.com"},
		{"https://subdomain.example.org", "example.org"},
		{"https://example.net", "example.net"},
	}

	for _, test := range tests {
		u, _ := url.Parse(test.url)
		req := &Request{
			Url: u,
			Ctx: container.NewSyncDict(),
		}

		domain := req.Domain()
		if domain != test.expected {
			t.Errorf("For URL %s, expected getDomain %s, got %s", test.url, test.expected, domain)
		}
	}
}

func TestToDictAndNewRequestFromDict(t *testing.T) {
	// 创建一个原始Request
	originalReq, err := NewRequest("https://example.com/test")
	if err != nil {
		t.Fatalf("Failed to create original request: %v", err)
	}

	originalReq.Method = "POST"
	originalReq.Encoding = "utf-8"
	originalReq.Priority = 5
	originalReq.DontFilter = true
	originalReq.Errback = "handleError"
	originalReq.Callback = "handleResponse"

	// 转换为Dict
	dict := originalReq.ToDict()

	// 从Dict重建Request
	reconstructedReq := NewRequestFromDict(dict)

	// 验证属性是否一致
	if originalReq.Url.String() != reconstructedReq.Url.String() {
		t.Errorf("URL mismatch: expected %s, got %s", originalReq.Url.String(), reconstructedReq.Url.String())
	}

	if originalReq.Method != reconstructedReq.Method {
		t.Errorf("Method mismatch: expected %s, got %s", originalReq.Method, reconstructedReq.Method)
	}

	if originalReq.Encoding != reconstructedReq.Encoding {
		t.Errorf("Encoding mismatch: expected %s, got %s", originalReq.Encoding, reconstructedReq.Encoding)
	}

	if originalReq.Priority != reconstructedReq.Priority {
		t.Errorf("Priority mismatch: expected %d, got %d", originalReq.Priority, reconstructedReq.Priority)
	}

	if originalReq.DontFilter != reconstructedReq.DontFilter {
		t.Errorf("DontFilter mismatch: expected %v, got %v", originalReq.DontFilter, reconstructedReq.DontFilter)
	}

	if originalReq.Errback != reconstructedReq.Errback {
		t.Errorf("Errback mismatch: expected %s, got %s", originalReq.Errback, reconstructedReq.Errback)
	}

	if originalReq.Callback != reconstructedReq.Callback {
		t.Errorf("Callback mismatch: expected %s, got %s", originalReq.Callback, reconstructedReq.Callback)
	}
}
