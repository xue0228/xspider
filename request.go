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
	"xspider/container"
	"xspider/encoder"
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
	Ctx        container.Dict
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
	request, err := NewRequest(
		u.String(),
		WithHeaders(*header),
		WithBody(bytes.NewBuffer(body)),
		WithMethod(r.Method))
	if err != nil {
		panic(err)
	}
	req, err := request.ToDict().Dumps()
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
	if d, ok := r.Ctx.GetString("domain"); ok {
		return d
	} else {
		d := getDomain(r.Url)
		r.Ctx.Set("domain", d)
		return d
	}
}

func (r *Request) ToDict() container.Dict {
	d := container.NewSyncDict()

	if r.Url != nil {
		d.Set("url", r.Url.String())
	} else {
		d.Set("url", "")
	}

	headerDict := container.NewSyncDict()
	if r.Headers != nil {
		for k, v := range *r.Headers {
			headerDict.Set(k, v)
		}
	}
	d.Set("headers", headerDict)

	if r.Body != nil {
		data := ReadRequestBody(r)
		body := base64.StdEncoding.EncodeToString(data)
		d.Set("body", body)
	} else {
		d.Set("body", "")
	}

	if r.Cookies != nil {
		cookies := CookiesToString(r.Cookies)
		d.Set("cookies", cookies)
	} else {
		d.Set("cookies", "")
	}

	ctx := container.NewSyncDict()
	if r.Ctx != nil {
		ctx = r.Ctx
	}
	d.Set("ctx", ctx)

	d.Set("method", r.Method)
	d.Set("encoding", r.Encoding)
	d.Set("priority", r.Priority)
	d.Set("dont_filter", r.DontFilter)
	d.Set("errback", r.Errback)
	d.Set("callback", r.Callback)

	return d
}

func NewRequestFromDict(d container.Dict) *Request {
	method, ok := d.GetString("method")
	if !ok {
		panic("method is required")
	}
	encoding, ok := d.GetString("encoding")
	if !ok {
		panic("encoding is required")
	}
	priority, ok := d.GetInt("priority")
	if !ok {
		panic("priority is required")
	}
	dontFilter, ok := d.GetBool("dont_filter")
	if !ok {
		panic("dont_filter is required")
	}
	errback, ok := d.GetString("errback")
	if !ok {
		panic("errback is required")
	}
	callback, ok := d.GetString("callback")
	if !ok {
		panic("callback is required")
	}

	var ur *url.URL
	urlStr, ok := d.GetString("url")
	if !ok {
		panic("url is required")
	} else {
		u, err := url.Parse(urlStr)
		if err != nil {
			panic(err)
		}
		ur = u
	}

	h, ok := d.Get("headers")
	if !ok {
		panic("headers is required")
	}
	headerDict, ok := h.(container.Dict)
	if !ok {
		panic("headers is not a Dict")
	}
	header := &http.Header{}
	for _, key := range headerDict.Keys() {
		v, _ := headerDict.Get(key)
		value, ok := v.([]string)
		if !ok {
			panic("header's value is not a []string")
		}
		for _, s := range value {
			header.Add(key, s)
		}
	}

	body, ok := d.GetString("body")
	if !ok {
		panic("body is required")
	}
	data, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		panic(err)
	}
	reader := bytes.NewBuffer(data)

	cookiesStr, ok := d.GetString("cookies")
	if !ok {
		panic("cookies is required")
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

	context, ok := d.Get("ctx")
	if !ok {
		panic("ctx is required")
	}
	ctx, ok := context.(container.Dict)
	if !ok {
		panic("ctx is not a Dict")
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

// RequestOption 是用于配置Request的函数类型
type RequestOption func(*Request)

// NewRequest 创建一个新的Request，只需要URL，其他参数通过可变参数设置
func NewRequest(URL string, opts ...RequestOption) (*Request, error) {
	u, err := url.Parse(URL)
	if err != nil {
		return nil, err
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
		Ctx:        container.NewSyncDict(),
		Errback:    "",
		Callback:   "",
	}

	// 应用可选配置
	for _, opt := range opts {
		opt(request)
	}

	return request, nil
}
