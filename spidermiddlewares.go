package xspider

import (
	"errors"
	"fmt"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/xue0228/xspider/container"
)

func init() {
	RegisterSpiderModuler(&StartSpiderMiddleware{})
	RegisterSpiderModuler(&UrlLengthSpiderMiddleware{})
	RegisterSpiderModuler(&AllowedDomainSpiderMiddleware{})
	RegisterSpiderModuler(&DepthSpiderMiddleware{})
	RegisterSpiderModuler(&HttpErrorSpiderMiddleware{})
}

type StartSpiderMiddleware struct {
	BaseSpiderMiddleware
}

func (ssm *StartSpiderMiddleware) Name() string {
	return "StartSpiderMiddleware"
}

func (ssm *StartSpiderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&ssm.BaseSpiderModule, spider, ssm.Name())
}

func (ssm *StartSpiderMiddleware) ProcessStartRequests(starts Results, spider *Spider) Results {
	return Generator(func(c chan<- any) {
		for start := range starts {
			if req, ok := start.(*Request); ok {
				//req.Ctx.Set("is_start_request", true)
				container.Set(req.Ctx, "is_start_request", true)
			}
			c <- start
		}
	})
}

type UrlLengthSpiderMiddleware struct {
	BaseSpiderMiddleware
	maxLength int
}

func (sm *UrlLengthSpiderMiddleware) Name() string {
	return "UrlLengthSpiderMiddleware"
}

func (sm *UrlLengthSpiderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&sm.BaseSpiderModule, spider, sm.Name())
	//sm.maxLength = spider.Settings.GetIntWithDefault("URL_LENGTH_LIMIT", 2083)
	sm.maxLength = container.GetWithDefault[int](spider.Settings, "URL_LENGTH_LIMIT", 2083)
}

func (sm *UrlLengthSpiderMiddleware) ProcessSpiderOutput(response *Response, results Results, spider *Spider) Results {
	return Generator(func(c chan<- any) {
		for result := range results {
			if req, ok := result.(*Request); ok {
				if sm.maxLength > 0 && len(req.Url.String()) > sm.maxLength {
					RequestLogger(sm.Logger, req).Infow("URL长度超出限制", "max_url_length", sm.maxLength)
					sm.Stats.IncValue("urllength/request_ignored_count", 1, 0)
					continue
				}
			}
			c <- result
		}
	})
}

type AllowedDomainSpiderMiddleware struct {
	BaseSpiderMiddleware
	allowedDomains []string
	seenDomains    *hashset.Set
}

func (sm *AllowedDomainSpiderMiddleware) Name() string {
	return "AllowedDomainSpiderMiddleware"
}

func (sm *AllowedDomainSpiderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&sm.BaseSpiderModule, spider, sm.Name())
	//ad := spider.Settings.GetWithDefault("ALLOWED_DOMAINS", []string{})
	//ad := spider.Settings.GetWithDefault("ALLOWED_DOMAINS", []string{})
	//allowDomain, ok := ad.([]string)
	//if !ok {
	//	panic("ALLOWED_DOMAIN is not a []string")
	//}
	//sm.allowedDomains = allowDomain
	sm.allowedDomains = container.GetWithDefault[[]string](spider.Settings, "ALLOWED_DOMAINS", []string{})
	sm.seenDomains = hashset.New()
}

func (sm *AllowedDomainSpiderMiddleware) ProcessSpiderOutput(response *Response, results Results, spider *Spider) Results {
	return Generator(func(c chan<- any) {
		for result := range results {
			if req, ok := result.(*Request); ok {
				d := req.Domain()
				if !sm.seenDomains.Contains(d) {
					sm.seenDomains.Add(d)
					sm.Stats.IncValue("allowed_domain/domains", 1, 0)
				}
				if len(sm.allowedDomains) > 0 {
					flag := false
					for _, domain := range sm.allowedDomains {
						if d == domain {
							flag = true
							break
						}
					}
					if !flag {
						RequestLogger(sm.Logger, req).Infow("请求的域名不在允许的域名列表中", "allowed_domains", sm.allowedDomains)
						sm.Stats.IncValue("allowed_domain/filtered", 1, 0)
						continue
					}
				}
			}
			c <- result
		}
	})
}

type DepthSpiderMiddleware struct {
	BaseSpiderMiddleware
	maxDepth     int
	verboseStats bool
	priority     int
}

func (sm *DepthSpiderMiddleware) Name() string {
	return "DepthSpiderMiddleware"
}

func (sm *DepthSpiderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&sm.BaseSpiderModule, spider, sm.Name())
	//sm.maxDepth = spider.Settings.GetIntWithDefault("DEPTH_LIMIT", 0)
	sm.maxDepth = container.GetWithDefault[int](spider.Settings, "DEPTH_LIMIT", 0)
	//sm.verboseStats = spider.Settings.GetBoolWithDefault("DEPTH_STATS_VERBOSE", false)
	sm.verboseStats = container.GetWithDefault[bool](spider.Settings, "DEPTH_STATS_VERBOSE", false)
	// request.priority = request.priority - (depth * DEPTH_PRIORITY)
	//sm.priority = spider.Settings.GetIntWithDefault("DEPTH_PRIORITY", 0)
	sm.priority = container.GetWithDefault[int](spider.Settings, "DEPTH_PRIORITY", 0)
}

func (sm *DepthSpiderMiddleware) ProcessStartRequests(starts Results, spider *Spider) Results {
	return Generator(func(c chan<- any) {
		for start := range starts {
			if request, ok := start.(*Request); ok {
				//request.Ctx.Set("depth", 0)
				container.Set(request.Ctx, "depth", 0)
				sm.Stats.IncValue("request_depth_count/0", 1, 0)
				sm.Stats.MaxValue("request_depth_max", 0)
			}
			c <- start
		}
	})
}

func (sm *DepthSpiderMiddleware) ProcessSpiderOutput(response *Response, results Results, spider *Spider) Results {
	return Generator(func(c chan<- any) {
		for result := range results {
			if request, ok := result.(*Request); ok {
				//depth := response.Ctx.GetIntWithDefault("depth", 0) + 1
				depth := container.GetWithDefault[int](response.Ctx, "depth", 0) + 1
				//request.Ctx.Set("depth", depth)
				container.Set(request.Ctx, "depth", depth)
				if sm.priority != 0 {
					request.Priority -= depth * sm.priority
				}
				if sm.maxDepth > 0 && depth > sm.maxDepth {
					RequestLogger(sm.Logger, request).Infow("请求深度超出限制", "max_depth", sm.maxDepth)
					continue
				} else {
					if sm.verboseStats {
						sm.Stats.IncValue(fmt.Sprintf("request_depth_count/%d", depth), 1, 0)
						sm.Stats.MaxValue("request_depth_max", depth)
					}
				}
			}
			c <- result
		}
	})
}

type HttpErrorSpiderMiddleware struct {
	BaseSpiderMiddleware
	handleAll bool
	handList  []int
}

func (sm *HttpErrorSpiderMiddleware) Name() string {
	return "HttpErrorSpiderMiddleware"
}

func (sm *HttpErrorSpiderMiddleware) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&sm.BaseSpiderModule, spider, sm.Name())
	// Pass all responses with non-200 status codes contained in this list
	//sm.handList = spider.Settings.GetWithDefault("HTTPERROR_ALLOWED_CODES", []int{}).([]int)
	sm.handList = container.GetWithDefault[[]int](spider.Settings, "HTTPERROR_ALLOWED_CODES", []int{})
	// Pass all responses, regardless of its status code
	//sm.handleAll = spider.Settings.GetBoolWithDefault("HTTPERROR_ALLOW_ALL", false)
	sm.handleAll = container.GetWithDefault[bool](spider.Settings, "HTTPERROR_ALLOW_ALL", false)
}

func (sm *HttpErrorSpiderMiddleware) ProcessSpiderInput(response *Response, spider *Spider) {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return
	}
	ctx := response.Ctx
	//if ctx.GetBoolWithDefault("handle_httpstatus_all", false) {
	if container.GetWithDefault(ctx, "handle_httpstatus_all", false) {
		return
	}
	allowedStatuses := []int{}
	//temAllowedStatuses, ok := ctx.Get("handle_httpstatus_list")
	temAllowedStatuses, err := container.Get[[]int](ctx, "handle_httpstatus_list")
	if err == nil {
		allowedStatuses = temAllowedStatuses
	} else if sm.handleAll {
		return
	} else {
		allowedStatuses = sm.handList
	}
	for _, s := range allowedStatuses {
		if response.StatusCode == s {
			return
		}
	}
	panic(fmt.Errorf("%d: %w", response.StatusCode, ErrHttpCode))
}

func (sm *HttpErrorSpiderMiddleware) ProcessSpiderError(response *Response, err error, spider *Spider) Results {
	if errors.Is(err, ErrHttpCode) {
		sm.Stats.IncValue("httperror/response_dropped_count", 1, 0)
		sm.Stats.IncValue(fmt.Sprintf("httperror/response_dropped_status_count/%s", ErrorToReason(err)), 1, 0)
		ResponseLogger(sm.Logger, response).Info("丢弃此Response：HTTP状态码没有处理或不被允许")
	}
	return nil
}
