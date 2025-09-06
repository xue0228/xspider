package xspider

import (
	"go.uber.org/zap"
)

type BaseSpiderModule struct {
	Logger *zap.SugaredLogger
	Stats  Statser
}

func InitBaseSpiderModule(b *BaseSpiderModule, spider *Spider, name string) {
	b.Logger = spider.Logger.With("module", name)
	b.Logger.Info("开始初始化模块")
	b.Stats = spider.Stats
}

func (bsm *BaseSpiderModule) Close(spider *Spider) {
	bsm.Logger.Info("模块已关闭")
}

type BaseSpiderMiddleware struct {
	BaseSpiderModule
}

func (sm *BaseSpiderMiddleware) ProcessStartRequests(starts Results, spider *Spider) Results {
	return starts
}

func (sm *BaseSpiderMiddleware) ProcessSpiderInput(response *Response, spider *Spider) {
	// 基础实现，可以被子类覆盖
	// 默认不处理输入，直接返回
}

func (sm *BaseSpiderMiddleware) ProcessSpiderOutput(response *Response, results Results, spider *Spider) Results {
	return results
}

func (sm *BaseSpiderMiddleware) ProcessSpiderError(response *Response, err error, spider *Spider) Results {
	return nil
}

type BaseDownloaderMiddleware struct {
	BaseSpiderModule
}

func (dm *BaseDownloaderMiddleware) ProcessRequest(request *Request, spider *Spider) Result {
	// 基础实现，直接返回传入的请求，不进行任何处理
	return nil
}

func (dm *BaseDownloaderMiddleware) ProcessResponse(request *Request, response *Response, spider *Spider) Result {
	return response
}

func (dm *BaseDownloaderMiddleware) ProcessError(request *Request, err error, spider *Spider) Result {
	return nil
}
