package xspider

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Request struct {
	Url        *url.URL
	Method     string
	Headers    *http.Header
	Body       io.Reader
	Cookies    []http.Cookie
	Encoding   string
	Priority   int
	DontFilter bool
	Ctx        *Context
	//请求抛出错误时的回调函数，错误包括404、请求超时等
	Errback ErrorbackFunc
	//request请求下载完成后处理其response的回调函数
	//默认调用Parse()
	Callback ParseFunc
}

func domain(u *url.URL) string {
	host := u.Hostname()
	list := strings.Split(host, ".")
	length := len(list)
	if length >= 2 {
		return list[length-2] + "." + list[length-1]
	} else {
		return "unknown"
	}
}

func (r *Request) Domain() string {
	if r.Ctx.Has("domain") {
		return r.Ctx.GetString("domain")
	} else {
		domain := domain(r.Url)
		r.Ctx.Put("domain", domain)
		return domain
	}
}

func NewRequest(method, URL string, body io.Reader) (*Request, error) {
	u, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}
	return &Request{
		Url:      u,
		Method:   strings.ToUpper(method),
		Headers:  &http.Header{},
		Body:     body,
		Encoding: "utf-8",
		Ctx:      NewContext(),
	}, nil
}
