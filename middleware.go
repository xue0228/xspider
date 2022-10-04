package xspider

import "go.uber.org/zap"

type DownloaderMiddlewarer interface {
	GetModuleName() string

	FromSpider(spider *Spider)

	ProcessRequest(request *Request, spider *Spider) RequestResponse

	ProcessResponse(request *Request, response *Response, spider *Spider) RequestResponse

	ProcessException(request *Request, err error, spider *Spider) RequestResponse
}

// 爬虫中间件接口
type SpiderMiddlewarer interface {
	GetModuleName() string

	FromSpider(spider *Spider)

	ProcessSpiderInput(response *Response, spider *Spider)

	ProcessSpiderOutput(response *Response, result RequestItems, spider *Spider) RequestItems

	ProcessSpiderException(response *Response, err error, spider *Spider) RequestItems

	ProcessStartRequests(result []*Request, spider *Spider) []*Request
}

// 下载器中间件各功能函数的返回值，Request、Response、nil或者ErrIgnoreRequest
type RequestResponse interface{}

type ProcessRequestFunc func(request *Request, spider *Spider) RequestResponse
type ProcessResponseFunc func(request *Request, response *Response, spider *Spider) RequestResponse
type ProcessExceptionFunc func(request *Request, err error, spider *Spider) RequestResponse

type ProcessSpiderInputFunc func(response *Response, spider *Spider)
type ProcessSpiderOutputFunc func(response *Response, result RequestItems, spider *Spider) RequestItems
type ProcessSpiderExceptionFunc func(response *Response, err error, spider *Spider) RequestItems
type ProcessStartRequestsFunc func(result []*Request, spider *Spider) []*Request

// 爬虫中间件及下载器中间件共用的基础中间件数据结构，实现了两者的全部接口
type BaseMiddleware struct {
	//中间件模块名称
	ModuleName string
	Logger     *zap.SugaredLogger
	Stats      StatsCollector
}

func (mw *BaseMiddleware) GetModuleName() string {
	return mw.ModuleName
}

func (mw *BaseMiddleware) FromSpider(spider *Spider) {
	mw.Logger = spider.Log.With("module_name", mw.ModuleName)
	mw.Stats = spider.Stats
}

func (mw *BaseMiddleware) ProcessSpiderInput(response *Response, spider *Spider) {}

func (mw *BaseMiddleware) ProcessSpiderOutput(response *Response, result RequestItems, spider *Spider) RequestItems {
	return result
}

func (mw *BaseMiddleware) ProcessSpiderException(response *Response, err error, spider *Spider) RequestItems {
	return RequestItems{}
}

func (mw *BaseMiddleware) ProcessStartRequests(result []*Request, spider *Spider) []*Request {
	return result
}

func (mw *BaseMiddleware) ProcessRequest(request *Request, spider *Spider) RequestResponse {
	return nil
}

func (mw *BaseMiddleware) ProcessResponse(request *Request, response *Response, spider *Spider) RequestResponse {
	return response
}

func (mw *BaseMiddleware) ProcessException(request *Request, err error, spider *Spider) RequestResponse {
	return nil
}
