package encoder

import (
	"testing"
)

func TestToBytes(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		encodingName string
		expectError  bool
	}{
		{"UTF-8 encoding", "hello", "utf-8", false},
		{"UTF-16 encoding", "hello", "utf-16", false},
		{"UTF-16LE encoding", "hello", "utf-16le", false},
		{"UTF-16BE encoding", "hello", "utf-16be", false},
		{"GBK encoding", "你好", "gbk", false},
		{"GB2312 encoding", "你好", "gb2312", false},
		{"BIG5 encoding", "你好", "big5", false},
		{"Unsupported encoding", "hello", "unsupported", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ToBytes(tt.input, tt.encodingName)
			if (err != nil) != tt.expectError {
				t.Errorf("ToBytes() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestToString(t *testing.T) {
	// 准备测试数据
	utf8Bytes := []byte("hello")
	utf16Bytes, _ := ToBytes("hello", "utf-16")
	gbkBytes, _ := ToBytes("你好", "gbk")

	tests := []struct {
		name         string
		input        []byte
		encodingName string
		expected     string
		expectError  bool
	}{
		{"UTF-8 decoding", utf8Bytes, "utf-8", "hello", false},
		{"UTF-16 decoding", utf16Bytes, "utf-16", "hello", false},
		{"GBK decoding", gbkBytes, "gbk", "你好", false},
		{"Unsupported decoding", utf8Bytes, "unsupported", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToString(tt.input, tt.encodingName)
			if (err != nil) != tt.expectError {
				t.Errorf("ToString() error = %v, expectError %v", err, tt.expectError)
			}
			if !tt.expectError && result != tt.expected {
				t.Errorf("ToString() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestB64Encode(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		altchars string
		expected string
	}{
		{"Standard base64", []byte("hello"), "", "aGVsbG8="},
		{"URL-safe base64", []byte("hello"), "-_", "aGVsbG8="},
		{"Standard with altchars", []byte("hello world"), "+/", "aGVsbG8gd29ybGQ="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := B64Encode(tt.input, tt.altchars)
			if result != tt.expected {
				t.Errorf("B64Encode() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestB64Decode(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		altchars  string
		validate  bool
		expected  []byte
		expectErr bool
	}{
		{"Standard decode", "aGVsbG8=", "", false, []byte("hello"), false},
		{"URL-safe decode", "aGVsbG8", "-_", false, []byte("hello"), false},
		{"Invalid base64 with validation", "invalid!", "", true, nil, true},
		{"Invalid base64 without validation", "invalid!", "", false, nil, true},
		{"Wrong altchars length", "aGVsbG8=", "abc", false, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := B64Decode(tt.input, tt.altchars, tt.validate)
			if (err != nil) != tt.expectErr {
				t.Errorf("B64Decode() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if !tt.expectErr && string(result) != string(tt.expected) {
				t.Errorf("B64Decode() = %v, expected %v", string(result), string(tt.expected))
			}
		})
	}
}

func TestStandardB64Functions(t *testing.T) {
	original := []byte("hello world")
	encoded := StandardB64Encode(original)
	decoded := StandardB64Decode(encoded)

	if string(decoded) != string(original) {
		t.Errorf("StandardB64 functions failed: expected %s, got %s", string(original), string(decoded))
	}
}

func TestUrlSafeB64Functions(t *testing.T) {
	original := []byte("hello world?")
	encoded := UrlSafeB64Encode(original)
	decoded := UrlSafeB64Decode(encoded)

	if string(decoded) != string(original) {
		t.Errorf("UrlSafeB64 functions failed: expected %s, got %s", string(original), string(decoded))
	}
}
