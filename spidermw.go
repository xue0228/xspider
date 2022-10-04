package xspider

import "fmt"

type DepthMiddleware struct {
	BaseMiddleware
	maxDepth     int
	verboseStats bool
	prio         int
}

func (mw *DepthMiddleware) FromSpider(spider *Spider) {
	mw.ModuleName = "spidermw/depth"
	mw.BaseMiddleware.FromSpider(spider)

	mw.maxDepth = spider.Settings.DepthLimit
	mw.verboseStats = spider.Settings.DepthStatsVerbose
	mw.prio = spider.Settings.DepthPriority
}

func (mw *DepthMiddleware) ProcessSpiderOutput(response *Response, result RequestItems, spider *Spider) RequestItems {
	filter := func(req interface{}) bool {
		switch req := req.(type) {
		case *Request:
			depth := response.Ctx.GetIntWithDefault("depth", 0) + 1
			req.Ctx.Put("depth", depth)
			if mw.prio != 0 {
				req.Priority -= depth * mw.prio
			}
			if mw.maxDepth != 0 && depth > mw.maxDepth {
				NewRequestLogger(mw.Logger, req).Debug("已过滤超出最大深度限制的Request")
				return false
			} else {
				if mw.verboseStats {
					mw.Stats.IncValue(fmt.Sprintf("request_depth_count/%d", depth), 1, 0)
					mw.Stats.MaxValue("request_depth_max", depth)
				}
			}
		}
		return true
	}

	if !response.Ctx.Has("depth") {
		response.Ctx.Put("depth", 0)
		if mw.verboseStats {
			mw.Stats.IncValue("request_depth_count/0", 1, 0)
		}
	}

	var res RequestItems = RequestItems{}
	for _, r := range result {
		if filter(r) {
			res = append(res, r)
		}
	}
	return res
}

type UrlLengthMiddleware struct {
	BaseMiddleware
	maxLength int
}

func (mw *UrlLengthMiddleware) FromSpider(spider *Spider) {
	mw.ModuleName = "spidermw/url_length"
	mw.BaseMiddleware.FromSpider(spider)

	mw.maxLength = spider.Settings.UrlLengthLimit
}

func (mw *UrlLengthMiddleware) ProcessSpiderOutput(response *Response, result RequestItems, spider *Spider) RequestItems {
	filter := func(req interface{}) bool {
		switch req := req.(type) {
		case *Request:
			if mw.maxLength != 0 && len(req.Url.String()) > mw.maxLength {
				NewRequestLogger(mw.Logger, req).Info("已过滤超出最大URL长度的Request")
				mw.Stats.IncValue("urllength/request_ignored_count", 1, 0)
				return false
			}
		}
		return true
	}

	var res RequestItems = RequestItems{}
	for _, r := range result {
		if filter(r) {
			res = append(res, r)
		}
	}
	return res
}

type HttpErrorMiddleware struct {
	BaseMiddleware
	handleHttpStatusAll  bool
	handleHttpStatusList []int
}

func (mw *HttpErrorMiddleware) FromSpider(spider *Spider) {
	mw.ModuleName = "spidermw/http_error"
	mw.BaseMiddleware.FromSpider(spider)

	mw.handleHttpStatusAll = spider.Settings.HttpErrorAllowAll
	mw.handleHttpStatusList = spider.Settings.HttpErrorAllowedCodes
}

func (mw *HttpErrorMiddleware) ProcessSpiderInput(response *Response, spider *Spider) {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return
	}
	meta := response.Ctx
	if meta.GetBoolWithDefault("handle_httpstatus_all", false) {
		return
	}
	var allowedStatuses []int
	if meta.Has("handle_httpstatus_list") {
		allowedStatuses = meta.GetWithDefault("handle_httpstatus_list", []int{}).([]int)
	} else if mw.handleHttpStatusAll {
		return
	}
	for _, v := range allowedStatuses {
		if response.StatusCode == v {
			return
		}
	}
	panic(HttpError(fmt.Sprintf("%d", response.StatusCode)))
}

func (mw *HttpErrorMiddleware) ProcessSpiderException(response *Response, err error, spider *Spider) RequestItems {
	if _, ok := err.(*ErrHttpError); ok {
		mw.Stats.IncValue("httperror/response_ignored_count", 1, 0)
		mw.Stats.IncValue(fmt.Sprintf("httperror/response_ignored_status_count/%d", response.StatusCode), 1, 0)
	}
	NewResponseLogger(mw.Logger, response).Info("忽略此Response：HTTP状态码没有处理或不被允许")
	return RequestItems{}
}
