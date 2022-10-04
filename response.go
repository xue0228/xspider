package xspider

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Response struct {
	StatusCode int
	Body       []byte
	Ctx        *Context
	Request    *Request
	Headers    *http.Header
}

func NewResponse(response *http.Response, request *Request) (*Response, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return &Response{
		StatusCode: response.StatusCode,
		Body:       body,
		Ctx:        request.Ctx,
		Request:    request,
		Headers:    &response.Header,
	}, nil
}

// Save writes response body to disk
func (r *Response) Save(fileName string) error {
	dir := filepath.Dir(fileName)
	// fmt.Println(dir)
	// if _, err := os.Stat(dir); os.IsNotExist(err) {
	// 	if err := os.MkdirAll(dir, 0777); err != nil {
	// 		fmt.Println(err)
	// 	}
	// }
	os.MkdirAll(dir, 0777)
	return os.WriteFile(fileName, r.Body, 0777)
}

// FileName returns the sanitized file name parsed from "Content-Disposition"
// header or from URL
func (r *Response) FileName() string {
	_, params, err := mime.ParseMediaType(r.Headers.Get("Content-Disposition"))
	if fName, ok := params["filename"]; ok && err == nil {
		return SanitizeFileName(fName)
	}
	if r.Request.Url.RawQuery != "" {
		return SanitizeFileName(fmt.Sprintf("%s_%s", r.Request.Url.Path, r.Request.Url.RawQuery))
	}
	return SanitizeFileName(strings.TrimPrefix(r.Request.Url.Path, "/"))
}
