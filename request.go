package xspider

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/xue0228/xspider/container"
	"github.com/xue0228/xspider/encoder"
)

type Request struct {
	Url        *url.URL
	Method     string
	Headers    *http.Header
	Body       io.Reader
	Cookies    []*http.Cookie
	Encoding   string
	Priority   int
	DontFilter bool
	Ctx        container.JsonMap
	// 请求抛出错误时的回调函数，错误包括404、请求超时等
	Errback string
	// request请求下载完成后处理其response的回调函数
	Callback string
}

//type CallbackFunc func(*Response, *Spider) Results
//type ErrbackFunc func(*Request, *Response, error, *Spider) Results

func HeaderToString(header http.Header) string {
	var sb strings.Builder
	for key, values := range header {
		key = http.CanonicalHeaderKey(key) // 可选：转为标准大小写
		for _, value := range values {
			sb.WriteString(key)
			sb.WriteString(": ")
			sb.WriteString(value)
			sb.WriteString("\r\n")
		}
	}
	return strings.TrimRight(sb.String(), "\r\n")
}

func (r *Request) HttpRepr() []byte {
	var buf strings.Builder
	buf.WriteString(r.Method)
	buf.WriteString(" ")
	buf.WriteString(r.Url.String())
	buf.WriteString(" HTTP/1.1\r\n")

	buf.WriteString("Host: ")
	buf.WriteString(r.Domain())
	buf.WriteString("\r\n")

	buf.WriteString(HeaderToString(*r.Headers))
	buf.WriteString("\r\n")

	if len(r.Cookies) > 0 {
		buf.WriteString(http.CanonicalHeaderKey("Cookie"))
		buf.WriteString(": ")
		buf.WriteString(CookiesToString(r.Cookies))
		buf.WriteString("\r\n")
	}

	if r.Body != nil {
		body := ReadRequestBody(r)
		buf.Write(body)
	}

	s := buf.String()
	s = strings.TrimRight(s, "\r\n")
	bs, err := encoder.ToBytes(s, "")
	if err != nil {
		panic(err)
	}
	return bs
}

func (r *Request) Fingerprint(includeHeaders []string, keepFragments bool) string {
	header := &http.Header{}
	if includeHeaders != nil && r.Headers != nil {
		for _, v := range includeHeaders {
			header.Add(v, r.Headers.Get(v))
		}
	}
	u, err := url.Parse(r.Url.String())
	if err != nil {
		panic(err)
	}
	if !!keepFragments {
		u.Fragment = ""
	}
	body := ReadRequestBody(r)
	request := NewRequest(
		u.String(),
		WithHeaders(*header),
		WithBody(bytes.NewBuffer(body)),
		WithMethod(r.Method))
	req, err := request.ToJsonMap().Dumps()
	if err != nil {
		panic(err)
	}
	o := sha1.New()
	o.Write(req)
	return hex.EncodeToString(o.Sum(nil))
}

func getDomain(u *url.URL) string {
	host := u.Hostname()
	list := strings.Split(host, ".")
	length := len(list)
	if length >= 2 {
		return list[length-2] + "." + list[length-1]
	} else if length == 1 {
		return list[0]
	} else {
		return "unknown"
	}
}

func (r *Request) Domain() string {
	if d, err := container.Get[string](r.Ctx, "domain"); err != nil {
		return d
	} else {
		d := getDomain(r.Url)
		container.Set(r.Ctx, "domain", d)
		return d
	}
}

func (r *Request) ToRequestTable() *RequestTable {
	var u, method, body string
	if r.Url != nil {
		u = r.Url.String()
	} else {
		u = ""
	}
	method = r.Method
	if r.Body != nil {
		data := ReadRequestBody(r)
		body = base64.StdEncoding.EncodeToString(data)
	} else {
		body = ""
	}
	return &RequestTable{
		Body:   body,
		Method: method,
		Url:    u,
		Fp:     r.Fingerprint(nil, false),
	}
}

func (r *Request) ToJsonMap() container.JsonMap {
	d := container.NewSyncJsonMap()

	if r.Url != nil {
		container.Set(d, "url", r.Url.String())
	} else {
		container.Set(d, "url", "")
	}

	headerDict := make(map[string][]string)
	if r.Headers != nil {
		for k, v := range *r.Headers {
			headerDict[k] = v
		}
	}
	container.Set(d, "headers", headerDict)

	if r.Body != nil {
		data := ReadRequestBody(r)
		body := base64.StdEncoding.EncodeToString(data)
		container.Set(d, "body", body)
	} else {
		container.Set(d, "body", "")
	}

	if r.Cookies != nil {
		cookies := CookiesToString(r.Cookies)
		container.Set(d, "cookies", cookies)
	} else {
		container.Set(d, "cookies", "")
	}

	var ctx map[string]any
	if r.Ctx != nil {
		ctx = r.Ctx.GetMap()
	}
	container.Set(d, "ctx", ctx)

	container.Set(d, "method", r.Method)
	container.Set(d, "encoding", r.Encoding)
	container.Set(d, "priority", r.Priority)
	container.Set(d, "dont_filter", r.DontFilter)
	container.Set(d, "errback", r.Errback)
	container.Set(d, "callback", r.Callback)

	return d
}

func NewRequestFromJsonMap(d container.JsonMap) *Request {
	method, err := container.Get[string](d, "method")
	if err != nil {
		panic(err)
	}
	encoding, err := container.Get[string](d, "encoding")
	if err != nil {
		panic(err)
	}
	priority, err := container.Get[int](d, "priority")
	if err != nil {
		panic(err)
	}
	dontFilter, err := container.Get[bool](d, "dont_filter")
	if err != nil {
		panic(err)
	}
	errback, err := container.Get[string](d, "errback")
	if err != nil {
		panic(err)
	}
	callback, err := container.Get[string](d, "callback")
	if err != nil {
		panic(err)
	}

	var ur *url.URL
	urlStr, err := container.Get[string](d, "url")
	if err != nil {
		panic(err)
	} else {
		u, err := url.Parse(urlStr)
		if err != nil {
			panic(err)
		}
		ur = u
	}

	headerDict, err := container.Get[map[string][]string](d, "headers")
	if err != nil {
		panic(err)
	}
	header := &http.Header{}
	for key, value := range headerDict {
		for _, s := range value {
			header.Add(key, s)
		}
	}

	body, err := container.Get[string](d, "body")
	if err != nil {
		panic(err)
	}
	data, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		panic(err)
	}
	reader := bytes.NewBuffer(data)

	cookiesStr, err := container.Get[string](d, "cookies")
	if err != nil {
		panic(err)
	}
	var cookies []*http.Cookie
	if cookiesStr == "" {
		cookies = nil
	} else {
		cookies, err = http.ParseCookie(cookiesStr)
		if err != nil {
			panic(err)
		}
	}

	context, err := container.Get[map[string]any](d, "ctx")
	if err != nil {
		panic(err)
	}
	ctx := container.NewSyncJsonMap()
	err = ctx.SetMap(context)
	if err != nil {
		panic(err)
	}

	return &Request{
		Method:     method,
		Encoding:   encoding,
		Priority:   priority,
		DontFilter: dontFilter,
		Errback:    errback,
		Callback:   callback,
		Url:        ur,
		Headers:    header,
		Body:       reader,
		Cookies:    cookies,
		Ctx:        ctx,
	}
}

// WithMethod 设置请求方法
func WithMethod(method string) RequestOption {
	return func(r *Request) {
		r.Method = strings.ToUpper(method)
	}
}

// WithBody 设置请求体
func WithBody(body io.Reader) RequestOption {
	return func(r *Request) {
		r.Body = body
	}
}

// WithHeaders 设置请求头
func WithHeaders(headers http.Header) RequestOption {
	return func(r *Request) {
		r.Headers = &headers
	}
}

// WithCookies 设置Cookie
func WithCookies(cookies []*http.Cookie) RequestOption {
	return func(r *Request) {
		r.Cookies = cookies
	}
}

// WithEncoding 设置编码
func WithEncoding(encoding string) RequestOption {
	return func(r *Request) {
		r.Encoding = encoding
	}
}

// WithPriority 设置优先级
func WithPriority(priority int) RequestOption {
	return func(r *Request) {
		r.Priority = priority
	}
}

// WithDontFilter 设置是否过滤
func WithDontFilter(dontFilter bool) RequestOption {
	return func(r *Request) {
		r.DontFilter = dontFilter
	}
}

// WithErrback 设置错误回调
func WithErrback(errback string) RequestOption {
	return func(r *Request) {
		r.Errback = errback
	}
}

// WithCallback 设置响应处理回调
func WithCallback(callback string) RequestOption {
	return func(r *Request) {
		r.Callback = callback
	}
}

func WithCtx(ctx container.JsonMap) RequestOption {
	return func(r *Request) {
		r.Ctx = ctx
	}
}

// RequestOption 是用于配置Request的函数类型
type RequestOption func(*Request)

// NewRequest 创建一个新的Request，只需要URL，其他参数通过可变参数设置
func NewRequest(URL string, opts ...RequestOption) *Request {
	u, err := url.Parse(URL)
	if err != nil {
		panic(err)
	}

	// 创建默认Request
	request := &Request{
		Url:        u,
		Method:     "GET", // 默认为GET方法
		Headers:    &http.Header{},
		Body:       nil,
		Cookies:    nil,
		Encoding:   "utf-8",
		Priority:   0,
		DontFilter: false,
		Ctx:        container.NewSyncJsonMap(),
		Errback:    "",
		Callback:   "",
	}

	// 应用可选配置
	for _, opt := range opts {
		opt(request)
	}

	return request
}
