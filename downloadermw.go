package xspider

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// UserAgent中间件，对没有在Headers中指定"User-Agent"参数的Request添加默认值
type UserAgentMiddleware struct {
	BaseMiddleware
	userAgent string
}

func (mw *UserAgentMiddleware) FromSpider(spider *Spider) {
	mw.ModuleName = "downloadermw/user_agent"
	mw.userAgent = spider.Settings.UserAgent
}

func (mw *UserAgentMiddleware) ProcessRequest(request *Request, spider *Spider) RequestResponse {
	if mw.userAgent != "" {
		request.Headers.Add("User-Agent", mw.userAgent)
	}
	return nil
}

// 下载超时中间件，对没有在Meta中指定"download_timeout"的Request添加默认超时时间
type DownloadTimeoutMiddleware struct {
	BaseMiddleware
	timeout time.Duration
}

func (mw *DownloadTimeoutMiddleware) FromSpider(spider *Spider) {
	mw.ModuleName = "downloadermw/download_timeout"
	mw.timeout = spider.Settings.DownloadTimeout
}

func (mw *DownloadTimeoutMiddleware) ProcessRequest(request *Request, spider *Spider) RequestResponse {
	if !request.Ctx.Has("download_timeout") {
		request.Ctx.Put("download_timeout", mw.timeout)
	}
	return nil
}

// Http认证中间件，如果爬虫的Setting中指定了"HttpUser"或"HttpPass"参数，
// 且Request的Header中未指定Authorization，则为该Request添加指定认证信息
type HttpAuthMiddleware struct {
	BaseMiddleware
	auth           string
	domainUnset    bool
	httpAuthDomain []string
}

func (mw *HttpAuthMiddleware) FromSpider(spider *Spider) {
	mw.ModuleName = "downloadermw/http_auth"
	mw.BaseMiddleware.FromSpider(spider)

	usr := spider.Settings.HttpUser
	pwd := spider.Settings.HttpPass
	if usr != "" || pwd != "" {
		mw.auth = BasicAuthHeader(usr, pwd)
		if spider.Settings.HttpAuthDomain == nil {
			mw.Logger.Warn("HttpAuthMiddleware必须搭配设置项HttpAuthDomain一起使用，" +
				"否则Request会携带认证信息访问多个域名带来安全问题。目前HttpAuthDomain参数将被设置为第一个Request的域名，" +
				"请尽快配置正确的HttpAuthDomain参数。")
			mw.domainUnset = true
		} else {
			mw.httpAuthDomain = spider.Settings.HttpAuthDomain
			mw.domainUnset = false
		}
	}
}

func (mw *HttpAuthMiddleware) ProcessRequest(request *Request, spider *Spider) RequestResponse {
	if mw.auth != "" {
		domain := request.Domain()
		if mw.domainUnset {
			mw.httpAuthDomain = []string{domain}
			mw.domainUnset = false
		}
		for _, v := range mw.httpAuthDomain {
			if strings.ToLower(v) == domain {
				request.Headers.Add("Authorization", mw.auth)
			}
		}
	}
	return nil
}

// 重试中间件，对符合要求的Response或者错误对应的Request进行重试
type RetryMiddleware struct {
	BaseMiddleware
	retryEnabled   bool
	maxRetryTimes  int
	retryHttpCodes Setter
	priorityAdjust int
}

func (mw *RetryMiddleware) FromSpider(spider *Spider) {
	mw.ModuleName = "downloadermw/retry"
	mw.BaseMiddleware.FromSpider(spider)

	mw.retryEnabled = spider.Settings.RetryEnabled
	mw.maxRetryTimes = spider.Settings.MaxRetryTimes
	mw.priorityAdjust = spider.Settings.RetryPriorityAdjust
	mw.retryHttpCodes = NewSet(0)
	for _, v := range spider.Settings.RetryHttpCodes {
		mw.retryHttpCodes.Add(v)
	}
}

func (mw *RetryMiddleware) ProcessResponse(request *Request, response *Response, spider *Spider) RequestResponse {
	if request.Ctx.GetBoolWithDefault("dont_retry", false) {
		return response
	}
	if mw.retryHttpCodes.Has(response.StatusCode) {
		reason := response.StatusCode
		req := mw.retry(request, fmt.Sprintf("%d", reason))
		if req == nil {
			return response
		} else {
			return req
		}
	}
	return response
}

func (mw *RetryMiddleware) ProcessException(request *Request, err error, spider *Spider) RequestResponse {
	ok, reason := errorToReason(err)
	if !ok {
		return nil
	}
	r := mw.retry(request, reason)
	if r != nil {
		return r
	} else {
		return nil
	}
}

func (mw *RetryMiddleware) retry(request *Request, reason string) *Request {
	maxRetryTimes := request.Ctx.GetIntWithDefault("max_retry_times", mw.maxRetryTimes)
	priorityAdjust := request.Ctx.GetIntWithDefault("priority_adjust", mw.priorityAdjust)
	return getRetryRequest(request, reason, maxRetryTimes, priorityAdjust, mw.Logger, mw.Stats, "retry")
}

func getRetryRequest(
	request *Request,
	reason string,
	maxRetryTimes int,
	priorityAdjust int,
	logger *zap.SugaredLogger,
	stats StatsCollector,
	statsBaseKey string,
) *Request {
	retryTimes := request.Ctx.GetIntWithDefault("retry_times", 0) + 1
	if retryTimes <= maxRetryTimes {
		NewRequestLogger(logger, request).With(
			"retry_times", retryTimes,
			"reason", reason,
		).Debug("重试Request")

		request.Ctx.Put("retry_times", retryTimes)
		request.DontFilter = true
		request.Priority += priorityAdjust

		stats.IncValue(fmt.Sprintf("%s/count", statsBaseKey), 1, 0)
		stats.IncValue(fmt.Sprintf("%s/count/%s", statsBaseKey, reason), 1, 0)
		return request
	} else {
		stats.IncValue(fmt.Sprintf("%s/max_reached", statsBaseKey), 1, 0)
		NewRequestLogger(logger, request).With(
			"retry_times", retryTimes,
			"reason", reason,
		).Error("Request重试次数过多")
		return nil
	}
}

func errorToReason(err error) (bool, string) {
	// if retryErr, ok := err.(*NeedRetryError); ok {
	// 	return true, retryErr.Reason
	// }
	// fmt.Println(reflect.TypeOf(err))

	if _, ok := err.(*url.Error); ok {
		return true, "url_error"
	}

	netErr, ok := err.(net.Error)
	if !ok {
		return false, ""
	}

	if netErr.Timeout() {
		return true, "timeout_error"
	}

	opErr, ok := netErr.(*net.OpError)
	if !ok {
		return true, "other_net_error"
	}

	switch t := opErr.Err.(type) {
	case *net.DNSError:
		return true, "dns_error"
	case *os.SyscallError:
		if errno, ok := t.Err.(syscall.Errno); ok {
			switch errno {
			case syscall.ECONNREFUSED:
				return true, "connect_refused_error"
			case syscall.ETIMEDOUT:
				return true, "timeout_error"
			}
		}
	case *net.InvalidAddrError:
		return true, "invalid_addr_error"
	case *net.UnknownNetworkError:
		return true, "unknown_netwrok_error"
	case *net.AddrError:
		return true, "addr_error"
	case *net.DNSConfigError:
		return true, "dns_config_error"
	}
	return true, "other_net_error"
}

type DownloadStatsMiddleware struct {
	BaseMiddleware
	downloadStats bool
}

func (mw *DownloadStatsMiddleware) FromSpider(spider *Spider) {
	mw.ModuleName = "downloadermw/download_stats"
	mw.BaseMiddleware.FromSpider(spider)

	mw.downloadStats = spider.Settings.DownloadStats
}

func (mw *DownloadStatsMiddleware) ProcessRequest(request *Request, spider *Spider) RequestResponse {
	if mw.downloadStats {
		mw.Stats.IncValue("downloader/request_count", 1, 0)
		mw.Stats.IncValue(fmt.Sprintf("downloader/request_method_count/%s", strings.ToLower(request.Method)), 1, 0)
		mw.Stats.IncValue("downloader/request_bytes", GetRequestSize(request), 0)
	}
	return nil
}

func (mw *DownloadStatsMiddleware) ProcessResponse(request *Request, response *Response, spider *Spider) RequestResponse {
	if mw.downloadStats {
		mw.Stats.IncValue("downloader/response_count", 1, 0)
		mw.Stats.IncValue(fmt.Sprintf("downloader/response_status_count/%d", response.StatusCode), 1, 0)
		mw.Stats.IncValue("downloader/response_bytes", GetResponseSize(response), 0)
	}
	return response
}

func (mw *DownloadStatsMiddleware) ProcessException(request *Request, err error, spider *Spider) RequestResponse {
	if mw.downloadStats {
		mw.Stats.IncValue("downloader/exception_count", 1, 0)
		reason := reflect.TypeOf(err).String()
		mw.Stats.IncValue(fmt.Sprintf("downloader/exception_type_count/%s", reason), 1, 0)
	}
	return nil
}
