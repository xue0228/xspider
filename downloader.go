package xspider

import (
	"net/http"
	"net/url"
	"time"
)

func init() {
	RegisterSpiderModuler(&DownloaderImpl{})
}

type DownloaderImpl struct {
	BaseSpiderModule
}

func (d *DownloaderImpl) Name() string {
	return "DownloaderImpl"
}

func (d *DownloaderImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&d.BaseSpiderModule, spider, d.Name())
	d.Logger.Info("下载器已初始化")
}

func (d *DownloaderImpl) Fetch(request *Request, spider *Spider) (*Response, error) {
	//创建网络请求客户端
	client := &http.Client{
		//超时
		Timeout: time.Duration(request.Ctx.GetIntWithDefault("download_timeout", 180)) * time.Second,
		//重定向
		//CheckRedirect: NewCheckRedirect(r),
	}
	//添加代理
	proxy := request.Ctx.GetStringWithDefault("proxy", "")
	if proxy != "" {
		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		trans := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		client.Transport = trans
	}

	//创建http.Request请求
	req, err := http.NewRequest(request.Method, request.Url.String(), request.Body)
	if err != nil {
		return nil, err
	}
	//向请求头中添加Cookies
	req.Header = *request.Headers
	for _, v := range request.Cookies {
		req.AddCookie(v)
	}

	//发送网络请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	res, err := NewResponseWithRequest(resp, request)
	if err != nil {
		return nil, err
	}

	return res, nil
}
