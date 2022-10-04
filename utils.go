package xspider

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unsafe"

	"go.uber.org/zap"
)

func BasicAuthHeader(username string, password string) string {
	auth := []byte(fmt.Sprintf("%s:%s", username, password))
	return "Basic " + UrlSafeBase64Encode(auth)
}

func UrlSafeBase64Encode(source []byte) string {
	byteArr := base64.StdEncoding.EncodeToString(source)
	safeUrl := strings.Replace(byteArr, "/", "_", -1)
	safeUrl = strings.Replace(safeUrl, "+", "-", -1)
	//safeUrl = strings.Replace(safeUrl, "=", "", -1)
	return safeUrl
}

func UrlSafeBase64Decode(source string) []byte {
	missing := (4 - len(source)%4) % 4
	source += strings.Repeat("=", missing)
	safeUrl := strings.Replace(source, "-", "+", -1)
	safeUrl = strings.Replace(safeUrl, "_", "/", -1)
	res, _ := base64.StdEncoding.DecodeString(safeUrl)
	return res
}

func GetHeaderSize(header http.Header) int {
	var size int = 0
	for k, v := range header {
		size += len(k)
		for _, v := range v {
			size += len(v)
		}
	}
	return size
}

func ReadRequestBody(request *Request) []byte {
	var p []byte
	if request.Body != nil {
		if b, err := io.ReadAll(request.Body); err == nil {
			p = b
		}
		request.Body = bytes.NewBuffer(p)
	}
	return p
}

func GetRequestSize(request *Request) int {
	return GetHeaderSize(*request.Headers) + len(ReadRequestBody(request)) +
		len(request.Url.String()+request.Method)
}

func GetResponseSize(response *Response) int {
	return len(response.Body) + GetHeaderSize(*response.Headers) + len(fmt.Sprintf("%d", response.StatusCode))
}

func NewRequestLogger(log *zap.SugaredLogger, req *Request) *zap.SugaredLogger {
	return log.With("url", req.Url.String(),
		"method", req.Method,
		"priority", req.Priority,
	)
}

func NewResponseLogger(log *zap.SugaredLogger, response *Response) *zap.SugaredLogger {
	return NewRequestLogger(log, response.Request).With(
		"status_code", response.StatusCode,
		"content_length", len(response.Body),
	)
}

func StringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}

func BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func FindScriptVar(b []byte, v string) string {
	reg := regexp.MustCompile(fmt.Sprintf(`var[ ]+%s[ ]*=[ ]*(.+?)[ ]*;`, v))
	res := BytesToString(reg.FindSubmatch(b)[1])
	res = strings.TrimPrefix(res, "\"")
	res = strings.TrimSuffix(res, "\"")
	return res
}
