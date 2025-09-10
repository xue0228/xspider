package xspider

import (
	"fmt"
	"strings"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/xue0228/xspider/container"
	"go.uber.org/zap"
)

func init() {
	RegisterSpiderModuler(&HttpAuthDownloaderMiddleware{})
	RegisterSpiderModuler(&UserAgentDownloaderMiddleware{})
	RegisterSpiderModuler(&DownloadTimeoutDownloaderMiddleware{})
	RegisterSpiderModuler(&DefaultHeadersDownloaderMiddleware{})
	RegisterSpiderModuler(&RetryDownloaderMiddleware{})
	RegisterSpiderModuler(&DownloaderStatsDownloaderMiddleware{})
}

type HttpAuthDownloaderMiddleware struct {
	BaseDownloaderMiddleware
	auth        string
	domains     []string
	domainUnset bool
}

func (dm *HttpAuthDownloaderMiddleware) Name() string {
	return "HttpAuthDownloaderMiddleware"
}

func (dm *HttpAuthDownloaderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&dm.BaseSpiderModule, spider, dm.Name())
	//httpUser := spider.Settings.GetStringWithDefault("HTTP_USER", "")
	//httpPass := spider.Settings.GetStringWithDefault("HTTP_PASS", "")
	httpUser := container.GetWithDefault[string](spider.Settings, "HTTP_USER", "")
	httpPass := container.GetWithDefault[string](spider.Settings, "HTTP_PASS", "")
	if httpUser != "" || httpPass != "" {
		dm.auth = BasicAuthHeader(httpUser, httpPass, "")
		//domains, ok := spider.Settings.Get("HTTP_AUTH_DOMAINS")
		//if !ok {
		//	dm.Logger.Warn("HttpAuthMiddleware必须搭配设置项HttpAuthDomains一起使用，" +
		//		"否则Request会携带认证信息访问多个域名带来安全问题。目前HttpAuthDomains参数将被设置为第一个Request的域名，" +
		//		"请尽快配置正确的HttpAuthDomains参数。")
		//	dm.domainUnset = true
		//} else {
		//	dm.domains = domains.([]string)
		//	dm.domainUnset = false
		//}
		if !spider.Settings.Has("HTTP_AUTH_DOMAINS") {
			dm.Logger.Warn("HttpAuthMiddleware必须搭配设置项HttpAuthDomains一起使用，" +
				"否则Request会携带认证信息访问多个域名带来安全问题。目前HttpAuthDomains参数将被设置为第一个Request的域名，" +
				"请尽快配置正确的HttpAuthDomains参数。")
			dm.domainUnset = true
		} else {
			dm.domains = container.GetWithDefault[[]string](spider.Settings, "HTTP_AUTH_DOMAINS", []string{})
			dm.domainUnset = false
		}
	}
}

func (dm *HttpAuthDownloaderMiddleware) ProcessRequest(request *Request, spider *Spider) Result {
	if dm.auth != "" {
		domain := request.Domain()
		if dm.domainUnset {
			dm.domains = []string{domain}
			dm.domainUnset = false
		}
		for _, v := range dm.domains {
			if strings.ToLower(v) == domain {
				request.Headers.Set("Authorization", dm.auth)
			}
		}
	}
	return nil
}

type UserAgentDownloaderMiddleware struct {
	BaseDownloaderMiddleware
	userAgent string
}

func (dm *UserAgentDownloaderMiddleware) Name() string {
	return "UserAgentDownloaderMiddleware"
}

func (dm *UserAgentDownloaderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&dm.BaseSpiderModule, spider, dm.Name())
	//dm.userAgent = spider.Settings.GetStringWithDefault("USER_AGENT", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36 Edg/97.0.1072.76")
	dm.userAgent = container.GetWithDefault[string](spider.Settings, "USER_AGENT", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36 Edg/97.0.1072.76")
}

func (dm *UserAgentDownloaderMiddleware) ProcessRequest(request *Request, spider *Spider) Result {
	if dm.userAgent != "" {
		request.Headers.Set("User-Agent", dm.userAgent)
	}
	return nil
}

type DownloadTimeoutDownloaderMiddleware struct {
	BaseDownloaderMiddleware
	timeout int
}

func (dm *DownloadTimeoutDownloaderMiddleware) Name() string {
	return "DownloadTimeoutDownloaderMiddleware"
}

func (dm *DownloadTimeoutDownloaderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&dm.BaseSpiderModule, spider, dm.Name())
	//dm.timeout = spider.Settings.GetIntWithDefault("DOWNLOAD_TIMEOUT", 180)
	dm.timeout = container.GetWithDefault[int](spider.Settings, "DOWNLOAD_TIMEOUT", 180)
}

func (dm *DownloadTimeoutDownloaderMiddleware) ProcessRequest(request *Request, spider *Spider) Result {
	if !request.Ctx.Has("download_timeout") {
		//request.Ctx.Set("download_timeout", dm.timeout)
		container.Set(request.Ctx, "download_timeout", dm.timeout)
	}
	return nil
}

type DefaultHeadersDownloaderMiddleware struct {
	BaseDownloaderMiddleware
	headers map[string]string
}

func (dm *DefaultHeadersDownloaderMiddleware) Name() string {
	return "DefaultHeadersDownloaderMiddleware"
}

func (dm *DefaultHeadersDownloaderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&dm.BaseSpiderModule, spider, dm.Name())
	dm.headers = container.GetWithDefault[map[string]string](spider.Settings, "DEFAULT_REQUEST_HEADERS", map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"})
	//headers := spider.Settings.GetWithDefault("DEFAULT_REQUEST_HEADERS", map[string]string{
	//	"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
	//	"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"})
	//switch h := headers.(type) {
	//case container.Dict:
	//	m := make(map[string]string, h.Len())
	//	h.ForEach(func(key string, value any) any {
	//		m[key] = value.(string)
	//		return nil
	//	})
	//	dm.headers = m
	//case map[string]string:
	//	dm.headers = h
	//default:
	//	panic("DEFAULT_REQUEST_HEADERS参数类型错误")
	//}
}

func (dm *DefaultHeadersDownloaderMiddleware) ProcessRequest(request *Request, spider *Spider) Result {
	if dm.headers != nil {
		for k, v := range dm.headers {
			request.Headers.Set(k, v)
		}
	}
	return nil
}

type RetryDownloaderMiddleware struct {
	BaseDownloaderMiddleware
	retryEnabled   bool
	maxRetryTimes  int
	priorityAdjust int
	retryHttpCodes *hashset.Set
	retryReasons   *hashset.Set
}

func (dm *RetryDownloaderMiddleware) Name() string {
	return "RetryDownloaderMiddleware"
}

func (dm *RetryDownloaderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&dm.BaseSpiderModule, spider, dm.Name())
	dm.retryHttpCodes = hashset.New()
	dm.retryReasons = hashset.New()

	//dm.retryEnabled = spider.Settings.GetBoolWithDefault("RETRY_ENABLED", true)
	dm.retryEnabled = container.GetWithDefault[bool](spider.Settings, "RETRY_ENABLED", true)
	//dm.maxRetryTimes = spider.Settings.GetIntWithDefault("RETRY_TIMES", 2)
	dm.maxRetryTimes = container.GetWithDefault[int](spider.Settings, "RETRY_TIMES", 2)
	//dm.priorityAdjust = spider.Settings.GetIntWithDefault("RETRY_PRIORITY_ADJUST", -1)
	dm.priorityAdjust = container.GetWithDefault[int](spider.Settings, "RETRY_PRIORITY_ADJUST", -1)
	//codes := spider.Settings.GetWithDefault("RETRY_HTTP_CODES", []any{500, 502, 503, 504, 522, 524, 408, 429})
	codes := container.GetWithDefault[[]int](spider.Settings, "RETRY_HTTP_CODES", []int{500, 502, 503, 504, 522, 524, 408, 429})
	for _, v := range codes {
		dm.retryHttpCodes.Add(v)
	}
	//reasons := spider.Settings.GetWithDefault("RETRY_REASONS", []any{
	//	"url.Error",
	//	"Timeout",
	//	"syscall.Errno",
	//	"syscall.ECONNREFUSED",
	//	"syscall.ETIMEDOUT",
	//	//"net.Error",
	//	"net.OpError",
	//	"net.DNSError",
	//	"os.SyscallError",
	//	"net.InvalidAddrError",
	//	"net.UnknownNetworkError",
	//	"net.AddrError",
	//	"net.DNSConfigError",
	//})
	reasons := container.GetWithDefault[[]string](spider.Settings, "RETRY_REASONS", []string{
		"url.Error",
		"Timeout",
		"syscall.Errno",
		"syscall.ECONNREFUSED",
		"syscall.ETIMEDOUT",
		//"net.Error",
		"net.OpError",
		"net.DNSError",
		"os.SyscallError",
		"net.InvalidAddrError",
		"net.UnknownNetworkError",
	})
	for _, v := range reasons {
		dm.retryReasons.Add(v)
	}
}

func (dm *RetryDownloaderMiddleware) ProcessResponse(request *Request, response *Response, spider *Spider) Result {
	//if request.Ctx.GetBoolWithDefault("dont_retry", false) {
	if container.GetWithDefault[bool](request.Ctx, "dont_retry", false) {
		return response
	}
	if dm.retryHttpCodes.Contains(response.StatusCode) {
		reason := response.StatusCode
		req := dm.retry(request, fmt.Sprintf("%d", reason))
		if req == nil {
			return response
		} else {
			return req
		}
	}
	return response
}

func (dm *RetryDownloaderMiddleware) ProcessError(request *Request, err error, spider *Spider) Result {
	reason := ErrorToReason(err)
	if !dm.retryReasons.Contains(reason) {
		return nil
	}
	req := dm.retry(request, reason)
	if req == nil {
		return nil
	}
	return req
}

func (dm *RetryDownloaderMiddleware) retry(request *Request, reason string) *Request {
	//maxRetryTimes := request.Ctx.GetIntWithDefault("max_retry_times", dm.maxRetryTimes)
	//priorityAdjust := request.Ctx.GetIntWithDefault("priority_adjust", dm.priorityAdjust)
	maxRetryTimes := container.GetWithDefault[int](request.Ctx, "max_retry_times", dm.maxRetryTimes)
	priorityAdjust := container.GetWithDefault[int](request.Ctx, "priority_adjust", dm.priorityAdjust)
	return getRetryRequest(request, reason, maxRetryTimes, priorityAdjust, dm.Logger, dm.Stats, "retry")
}

func getRetryRequest(
	request *Request,
	reason string,
	maxRetryTimes int,
	priorityAdjust int,
	logger *zap.SugaredLogger,
	stats Statser,
	statsBaseKey string,
) *Request {
	//retryTimes := request.Ctx.GetIntWithDefault("retry_times", 0) + 1
	retryTimes := container.GetWithDefault[int](request.Ctx, "retry_times", 0) + 1
	if retryTimes <= maxRetryTimes {
		RequestLogger(logger, request).With(
			"retry_times", retryTimes,
			"reason", reason,
		).Debug("重试Request")

		//request.Ctx.Set("retry_times", retryTimes)
		container.Set(request.Ctx, "retry_times", retryTimes)
		request.DontFilter = true
		request.Priority += priorityAdjust

		stats.IncValue(fmt.Sprintf("%s/count", statsBaseKey), 1, 0)
		stats.IncValue(fmt.Sprintf("%s/count/%s", statsBaseKey, reason), 1, 0)
		return request
	} else {
		stats.IncValue(fmt.Sprintf("%s/max_reached", statsBaseKey), 1, 0)
		RequestLogger(logger, request).With(
			"retry_times", retryTimes,
			"reason", reason,
		).Error("Request重试次数过多")
		return nil
	}
}

type DownloaderStatsDownloaderMiddleware struct {
	BaseDownloaderMiddleware
}

func (dm *DownloaderStatsDownloaderMiddleware) Name() string {
	return "DownloaderStatsDownloaderMiddleware"
}

func (dm *DownloaderStatsDownloaderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&dm.BaseSpiderModule, spider, dm.Name())
}

func (dm *DownloaderStatsDownloaderMiddleware) ProcessRequest(request *Request, spider *Spider) Result {
	dm.Stats.IncValue("downloader/request_count", 1, 0)
	dm.Stats.IncValue("downloader/request_method_count/"+strings.ToLower(request.Method), 1, 0)
	dm.Stats.IncValue("downloader/request_bytes", len(request.HttpRepr()), 0)
	return nil
}

func (dm *DownloaderStatsDownloaderMiddleware) ProcessResponse(request *Request, response *Response, spider *Spider) Result {
	dm.Stats.IncValue("downloader/response_count", 1, 0)
	dm.Stats.IncValue(fmt.Sprintf("downloader/response_status_count/%d", response.StatusCode), 1, 0)
	dm.Stats.IncValue("downloader/response_bytes", GetResponseSize(response), 0)
	return response
}

func (dm *DownloaderStatsDownloaderMiddleware) ProcessError(request *Request, err error, spider *Spider) Result {
	dm.Stats.IncValue("downloader/error_count", 1, 0)
	dm.Stats.IncValue(fmt.Sprintf("downloader/error_type_count/%s", ErrorToReason(err)), 1, 0)
	return nil
}
