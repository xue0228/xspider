package xspider

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func init() {
	RegisterSpiderModuler(&EnginerImpl{})
}

type ItemResponseSignal struct {
	Item     any
	Response *Response
	Spider   *Spider
}

type EnginerImpl struct {
	BaseSpiderModule
	signal SignalManager

	heartBeat time.Duration

	quit          chan struct{}
	itemChan      chan struct{}
	schedulerChan chan struct{}

	osChan chan os.Signal

	itemSlot     ItemSloter
	responseSlot ResponseSloter
	requestSlot  RequestSloter

	wg sync.WaitGroup
}

func (eg *EnginerImpl) Name() string {
	return "EnginerImpl"
}

func (eg *EnginerImpl) FromSpider(spider *Spider) {
	// 初始化基类
	InitBaseSpiderModule(&eg.BaseSpiderModule, spider, eg.Name())
	eg.signal = spider.Signal

	// 初始化通信参数
	eg.heartBeat = time.Millisecond * 100
	eg.quit = make(chan struct{})
	eg.itemChan = make(chan struct{})
	eg.schedulerChan = make(chan struct{})

	eg.osChan = make(chan os.Signal, 1)
	signal.Notify(eg.osChan, os.Interrupt, syscall.SIGTERM)

	// 初始化缓存调度组件
	responseSlotStr := spider.Settings.GetStringWithDefault("RESPONSE_SLOT_STRUCT", "ResponseSlotImpl")
	eg.responseSlot = GetAndAssertComponent[ResponseSloter](responseSlotStr)
	eg.responseSlot.FromSpider(spider)
	itemSLotStr := spider.Settings.GetStringWithDefault("ITEM_SLOT_STRUCT", "ItemSlotImpl")
	eg.itemSlot = GetAndAssertComponent[ItemSloter](itemSLotStr)
	eg.itemSlot.FromSpider(spider)
	requestSlotStr := spider.Settings.GetStringWithDefault("REQUEST_SLOT_STRUCT", "RequestSlotImpl")
	eg.requestSlot = GetAndAssertComponent[RequestSloter](requestSlotStr)
	eg.requestSlot.FromSpider(spider)

	// 注册信号的回调函数
	eg.signal.Connect(eg.spiderOpened, StSpiderOpened, 0)
	eg.signal.Connect(eg.startsLeftSpider, StStartsLeftSpider, 500)
	eg.signal.Connect(eg.startsLeftSpiderMiddleware, StStartsLeftSpiderMiddleware, 500)
	eg.signal.Connect(eg.requestLeftEngine, StRequestLeftEngine, 500)
	eg.signal.Connect(eg.itemLeftEngine, StItemLeftEngine, 500)
	eg.signal.Connect(eg.requestLeftScheduler, StRequestLeftScheduler, 500)
	eg.signal.Connect(eg.requestReachedDownloaderMiddleware, StRequestReachedDownloaderMiddleware, 500)
	eg.signal.Connect(eg.requestReachedDownloader, StRequestReachedDownloader, 500)
	eg.signal.Connect(eg.responseLeftDownloader, StResponseLeftDownloader, 500)
	eg.signal.Connect(eg.responseLeftDownloaderMiddleware, StResponseLeftDownloaderMiddleware, 500)
	eg.signal.Connect(eg.responseReachedSpiderMiddleware, StResponseReachedSpiderMiddleware, 500)
	eg.signal.Connect(eg.responseReachedSpider, StResponseReachedSpider, 500)
	eg.signal.Connect(eg.resultsLeftSpider, StResultsLeftSpider, 500)
	eg.signal.Connect(eg.resultsLeftSpiderMiddleware, StResultsLeftSpiderMiddleware, 500)
	eg.signal.Connect(eg.spiderError, StSpiderError, 500)
	eg.signal.Connect(eg.downloaderError, StDownloaderError, 500)
	eg.signal.Connect(eg.requestErrback, StRequestErrback, 500)
	eg.signal.Connect(eg.spiderIdle, StSpiderIdle, 500)
	eg.signal.Connect(eg.spiderClosed, StSpiderClosed, 1000)

	eg.Logger.Info("引擎已初始化")
}

func (eg *EnginerImpl) Start(spider *Spider) {
	go func() {
		eg.wg.Add(1)
		defer eg.wg.Done()
		eg.processInterrupt(spider)
	}()
	go func() {
		eg.wg.Add(1)
		defer eg.wg.Done()
		eg.processItem(spider)
	}()
	go func() {
		eg.wg.Add(1)
		defer eg.wg.Done()
		eg.processDownloader(spider)
	}()
	eg.signal.Emit(NewSpiderOpenedSignal(SenderEngine, spider))
	eg.processScheduler(spider)
	eg.wg.Wait()
}

func (eg *EnginerImpl) Close(spider *Spider) {
	close(eg.quit)
	eg.wg.Wait()
	eg.itemSlot.Close(spider)
	eg.responseSlot.Close(spider)
	eg.requestSlot.Close(spider)
	eg.BaseSpiderModule.Close(spider)
}

func (eg *EnginerImpl) processInterrupt(spider *Spider) {
	count := 0

	for {
		select {
		case sig := <-eg.osChan:
			count++
			if count == 1 {
				// 第一次接收到信号，停止从scheduler调度新请求
				eg.Logger.Info("接收到中断信号，正在优雅关闭...", "signal", sig)
				close(eg.schedulerChan)
			} else {
				// 第二次接收到信号，强制退出
				eg.Logger.Info("再次接收到中断信号，强制退出", "signal", sig)
				eg.signal.Emit(NewSpiderClosedSignal(SenderEngine, "interrupted", spider))
			}
		case <-eg.quit:
			return
		}
	}
}

// 处理Item循环
func (eg *EnginerImpl) processItem(spider *Spider) {
	for {
		select {
		case <-eg.itemChan:
			itemSignal := eg.itemSlot.Pop()
			if itemSignal == nil {
				continue
			}
			spider.Signal.Emit(NewItemLeftEngineSignal(SenderEngine, itemSignal.Item, itemSignal.Response, spider))
			if eg.itemSlot.IsFree() && !eg.itemSlot.IsEmpty() {
				eg.triggerItem()
			}
		case <-time.After(eg.heartBeat):
			if eg.itemSlot.IsFree() && !eg.itemSlot.IsEmpty() {
				eg.triggerItem()
			}
		case <-eg.quit:
			return
		}
	}
}

// 处理下载器循环
func (eg *EnginerImpl) processDownloader(spider *Spider) {
	for {
		select {
		case <-eg.quit:
			return
		default:
			requests := eg.requestSlot.Pop()
			idx := 0
			for request := range requests {
				eg.signal.Emit(NewRequestReachedDownloaderSignal(SenderDownloader, request, spider))
				idx++
			}
			if idx == 0 {
				time.Sleep(eg.heartBeat)
				eg.requestSlot.Clear(300 * time.Second)
			}
		}
	}
}

// 处理调度器循环
func (eg *EnginerImpl) processScheduler(spider *Spider) {
	for {
		select {
		case <-eg.schedulerChan:
			//fmt.Println("1")
			if (eg.requestSlot.IsFree() || eg.requestSlot.IsEmpty()) &&
				eg.itemSlot.IsFree() && eg.responseSlot.IsFree() {
				if spider.scheduler.HasPendingRequests() {
					request := spider.scheduler.NextRequest()
					spider.Signal.Emit(NewRequestLeftSchedulerSignal(SenderScheduler, request, spider))
					eg.triggerScheduler()
				}
			}
		case <-time.After(eg.heartBeat):
			//fmt.Println(spider.scheduler.HasPendingRequests())
			//fmt.Println(eg.requestSlot.IsEmpty())
			//fmt.Println(eg.itemSlot.IsEmpty())
			//fmt.Println(spider.Signal.IsAllDone())
			if eg.isIdle(spider) {
				spider.Signal.Emit(NewSpiderIdleSignal(SenderEngine, spider))
			} else {
				eg.triggerScheduler()
			}
		case <-eg.quit:
			return
		}
	}
}

// 判断是否全部爬取完成
func (eg *EnginerImpl) isIdle(spider *Spider) bool {
	return !spider.scheduler.HasPendingRequests() &&
		eg.requestSlot.IsEmpty() &&
		eg.itemSlot.IsEmpty() &&
		spider.Signal.IsAllDone()
}

// 触发调度器
func (eg *EnginerImpl) triggerScheduler() {
	go func() {
		eg.wg.Add(1)
		defer eg.wg.Done()
		select {
		case eg.schedulerChan <- struct{}{}:
		default:
		}
	}()
}

func (eg *EnginerImpl) triggerItem() {
	go func() {
		eg.wg.Add(1)
		defer eg.wg.Done()
		select {
		case eg.itemChan <- struct{}{}:
		default:
		}
	}()
}

// 信号处理逻辑

// 爬虫启动后发出Starts
func (eg *EnginerImpl) spiderOpened(spider *Spider) {
	eg.Logger.Info("爬虫引擎启动")
	eg.signal.Emit(NewStartsLeftSpiderSignal(SenderEngine, spider.Starts, spider))
}

// 起始请求送往ProcessStartRequests处理
func (eg *EnginerImpl) startsLeftSpider(results Results, spider *Spider) {
	eg.Logger.Debug("开始处理起始请求ProcessStartRequests")
	defer eg.Logger.Debug("结束处理起始请求ProcessStartRequests")

	if results == nil {
		eg.Logger.Warn("starts is nil")
		return
	}

	starts, idx, err := spider.spiderManager.ProcessStartRequests(results, spider)
	if err != nil {
		LogSpiderModulerError(
			eg.Logger, zap.FatalLevel,
			"ProcessStartRequests方法出错",
			spider.spiderManager.Middlewares()[idx], err)
	}
	if starts == nil {
		LogSpiderModulerError(
			eg.Logger, zap.FatalLevel,
			"ProcessStartRequests方法出错",
			spider.spiderManager.Middlewares()[idx],
			fmt.Errorf("middleware %s returned nil", spider.spiderManager.Middlewares()[idx].Name()))
	}

	eg.signal.Emit(NewStartsLeftSpiderMiddlewareSignal(SenderProcessStartRequests, starts, spider))
}

// 处理好的起始请求在Engine处分发，Request直接发往Scheduler，Item暂存Engine等待处理
func (eg *EnginerImpl) startsLeftSpiderMiddleware(results Results, spider *Spider) {
	eg.Logger.Debug("开始分发起始请求处理结果")
	defer eg.Logger.Debug("结束分发起始请求处理结果")

	for result := range results {
		switch start := result.(type) {
		case *Request:
			eg.signal.Emit(NewRequestLeftEngineSignal(SenderProcessStartRequests, start, spider))
		default:
			item := &ItemResponseSignal{
				Item:     start,
				Response: nil,
				Spider:   spider,
			}
			eg.itemSlot.Push(item)
		}
	}
}

// 新生成的请求送往Scheduler等候调度
func (eg *EnginerImpl) requestLeftEngine(request *Request, spider *Spider) {
	spider.scheduler.EnqueueRequest(request)
}

// 将Item发往ItemPipeline处理
func (eg *EnginerImpl) itemLeftEngine(item any, response *Response, spider *Spider, sender Sender) {
	var logger *zap.SugaredLogger
	if response != nil {
		logger = ResponseLogger(eg.Logger, response)
	} else {
		logger = eg.Logger.With("sender", sender)
	}
	logger.Debug("开始处理Item")

	defer func() {
		eg.itemSlot.Finish(&ItemResponseSignal{
			Item:     item,
			Response: response,
			Spider:   spider,
		})
		logger.Debug("结束处理Item")
	}()

	itemProcessed, idx, err := spider.itemManager.ProcessItem(item, spider)
	if err != nil {
		if errors.Is(err, ErrDropItem) {
			LogSpiderModulerError(
				logger, zap.InfoLevel,
				"ProcessItem方法出错",
				spider.itemManager.ItemPipelines()[idx], err)
			eg.signal.Emit(NewItemDroppedSignal(SenderItemPipeline, itemProcessed, response, err, spider))
		} else {
			LogSpiderModulerError(
				logger, zap.ErrorLevel,
				"ProcessItem方法出错",
				spider.itemManager.ItemPipelines()[idx], err)
			eg.signal.Emit(NewItemErrorSignal(SenderItemPipeline, itemProcessed, response, err, spider))
		}
	} else {
		eg.signal.Emit(NewItemScrapedSignal(SenderItemPipeline, itemProcessed, response, spider))
	}

	if itemProcessed == nil {
		LogSpiderModulerError(
			logger, zap.FatalLevel,
			"ProcessItem方法出错",
			spider.itemManager.ItemPipelines()[idx],
			fmt.Errorf("middleware %s returned nil", spider.itemManager.ItemPipelines()[idx].Name()))
	}
}

// 离开Scheduler的请求Engine不做额外处理直接发往DownloaderMiddleware
func (eg *EnginerImpl) requestLeftScheduler(request *Request, spider *Spider) {
	logger := RequestLogger(eg.Logger, request)
	logger.Debug("开始分发请求到下载器中间件")
	defer logger.Debug("结束分发请求到下载器中间件")

	eg.signal.Emit(NewRequestReachedDownloaderMiddlewareSignal(SenderEngine, request, spider))
}

// 在请求进入下载器之前使用ProcessRequest处理
func (eg *EnginerImpl) requestReachedDownloaderMiddleware(request *Request, spider *Spider) {
	logger := RequestLogger(eg.Logger, request)
	logger.Debug("开始处理请求ProcessRequest")
	defer logger.Debug("结束处理请求ProcessRequest")

	result, idx, err := spider.downloaderManager.ProcessRequest(request, spider)

	if err != nil {
		if errors.Is(err, ErrDropRequest) {
			LogSpiderModulerError(
				logger, zap.InfoLevel,
				"ProcessRequest方法出错",
				spider.downloaderManager.Middlewares()[idx], err)
			eg.signal.Emit(NewRequestDroppedSignal(SenderProcessRequest, request, err, spider))
		} else {
			LogSpiderModulerError(
				logger, zap.ErrorLevel,
				"ProcessRequest方法出错",
				spider.downloaderManager.Middlewares()[idx], err)
		}
		eg.signal.Emit(NewDownloaderErrorSignal(SenderProcessRequest, request, err, spider))
		return
	}

	if result == nil {
		eg.requestSlot.Push(request)
		return
	}

	switch res := result.(type) {
	case *Request:
		eg.signal.Emit(NewRequestLeftEngineSignal(SenderProcessRequest, res, spider))
	case *Response:
		eg.signal.Emit(NewResponseLeftDownloaderSignal(SenderProcessRequest, request, res, spider))
	default:
		LogSpiderModulerError(
			logger, zap.FatalLevel,
			"ProcessRequest方法返回类型错误",
			spider.downloaderManager.Middlewares()[idx],
			fmt.Errorf("invalid type: %s", reflect.TypeOf(result).String()))
	}
}

// 下载请求
func (eg *EnginerImpl) requestReachedDownloader(request *Request, spider *Spider) {
	logger := RequestLogger(eg.Logger, request)
	logger.Debug("开始下载请求")

	defer func() {
		eg.requestSlot.Finish(request)
		logger.Debug("结束下载请求")
	}()

	response, err := spider.downloader.Fetch(request, spider)
	if err != nil {
		LogSpiderModulerError(
			logger, zap.ErrorLevel,
			"Downloader下载出错",
			spider.downloader, err)
		eg.signal.Emit(NewDownloaderErrorSignal(SenderDownloader, request, err, spider))
		return
	}
	eg.signal.Emit(NewResponseLeftDownloaderSignal(SenderDownloader, request, response, spider))
}

// 由ProcessResponse处理Response
func (eg *EnginerImpl) responseLeftDownloader(request *Request, response *Response, spider *Spider) {
	logger := ResponseLogger(eg.Logger, response)
	logger.Debug("开始处理响应ProcessResponse")
	defer logger.Debug("结束处理响应ProcessResponse")

	result, idx, err := spider.downloaderManager.ProcessResponse(request, response, spider)

	if err != nil {
		if errors.Is(err, ErrDropRequest) {
			LogSpiderModulerError(
				logger, zap.InfoLevel,
				"ProcessRequest方法出错",
				spider.downloaderManager.Middlewares()[idx], err)
			eg.signal.Emit(NewRequestDroppedSignal(SenderProcessResponse, request, err, spider))
		} else {
			LogSpiderModulerError(
				logger, zap.ErrorLevel,
				"ProcessRequest方法出错",
				spider.downloaderManager.Middlewares()[idx], err)
		}
		eg.signal.Emit(NewRequestErrbackSignal(SenderProcessResponse, request, response, err, spider))
		return
	}

	switch res := result.(type) {
	case *Request:
		eg.signal.Emit(NewRequestLeftEngineSignal(SenderProcessResponse, res, spider))
	case *Response:
		eg.signal.Emit(NewResponseLeftDownloaderMiddlewareSignal(SenderProcessResponse, res, spider))
	default:
		LogSpiderModulerError(
			logger, zap.FatalLevel,
			"ProcessResponse方法返回类型错误",
			spider.downloaderManager.Middlewares()[idx],
			fmt.Errorf("invalid type: %s", reflect.TypeOf(result).String()))
	}
}

// Engine不对结果做额外处理，直接发往SpiderMiddleware
func (eg *EnginerImpl) responseLeftDownloaderMiddleware(response *Response, spider *Spider) {
	logger := ResponseLogger(eg.Logger, response)
	logger.Debug("开始分发响应到爬虫中间件")
	defer logger.Debug("结束分发响应爬虫中间件")

	eg.signal.Emit(NewResponseReachedSpiderMiddlewareSignal(SenderEngine, response, spider))
}

// 通过ProcessSpiderInput处理Response
func (eg *EnginerImpl) responseReachedSpiderMiddleware(response *Response, spider *Spider) {
	logger := ResponseLogger(eg.Logger, response)
	logger.Debug("开始处理响应ProcessSpiderInput")
	defer logger.Debug("结束处理响应ProcessSpiderInput")

	idx, err := spider.spiderManager.ProcessSpiderInput(response, spider)
	if err != nil {
		LogSpiderModulerError(
			logger, zap.ErrorLevel,
			"ProcessSpiderInput方法出错",
			spider.spiderManager.Middlewares()[idx], err)
		eg.signal.Emit(NewRequestErrbackSignal(SenderProcessSpiderInput, response.Request, response, err, spider))
		return
	}
	eg.signal.Emit(NewResponseReachedSpiderSignal(SenderProcessSpiderInput, response, spider))
}

// 调用Request的解析函数处理Response
func (eg *EnginerImpl) responseReachedSpider(response *Response, spider *Spider) {
	logger := RequestLogger(eg.Logger, response.Request)
	logger.Debug("开始解析响应")

	eg.responseSlot.Add(response)
	defer func() {
		eg.responseSlot.Done(response)
		logger.Debug("结束解析响应")
	}()

	defer func() {
		if err := recover(); err != nil {
			logger.Errorw("解析Response出错", "error", err)
			eg.signal.Emit(NewSpiderErrorSignal(SenderSpider, response, err.(error), spider))
		}
	}()

	if response.Request.Callback == "" {
		response.Request.Callback = spider.DefaultParseFunc
	}
	callback, ok := GetRegisteredByName(response.Request.Callback)
	if !ok {
		logger.Fatal("未注册的CallbackFunc名称")
	}
	//callbackFunc, ok := callback.(CallbackFunc)
	callbackFunc, ok := callback.(func(*Response, *Spider) Results)
	if !ok {
		logger.Fatalw("CallbackFunc类型错误",
			"type", reflect.TypeOf(callback).String())
	}
	results := callbackFunc(response, spider)
	if results == nil {
		logger.Warn("CallbackFunc返回结果为空")
		return
	}
	eg.signal.Emit(NewResultsLeftSpiderSignal(
		SenderSpider, response, results, spider.spiderManager.Len()-1, spider))
}

// 使用ProcessSpiderOutput处理爬虫结果
func (eg *EnginerImpl) resultsLeftSpider(response *Response, results Results, index int, spider *Spider) {
	logger := ResponseLogger(eg.Logger, response)
	logger.Debug("开始处理爬虫结果ProcessSpiderOutput")
	defer logger.Debug("结束处理爬虫结果ProcessSpiderOutput")

	resultsProcessed, idx, err := spider.spiderManager.ProcessSpiderOutput(response, results, index, spider)

	if err != nil {
		LogSpiderModulerError(
			logger, zap.ErrorLevel,
			"ProcessSpiderOutput方法出错",
			spider.spiderManager.Middlewares()[idx], err)
		eg.signal.Emit(NewSpiderErrorSignal(SenderProcessSpiderOutput, response, err, spider))
		return
	}
	if resultsProcessed == nil {
		LogSpiderModulerError(
			logger, zap.FatalLevel,
			"ProcessSpiderOutput方法出错",
			spider.spiderManager.Middlewares()[idx],
			fmt.Errorf("middleware %s returned nil", spider.spiderManager.Middlewares()[idx].Name()))
	}

	eg.signal.Emit(NewResultsLeftSpiderMiddlewareSignal(SenderProcessSpiderOutput, response, resultsProcessed, spider))
}

// 分发从爬虫中间件返回的结果
func (eg *EnginerImpl) resultsLeftSpiderMiddleware(response *Response, results Results, spider *Spider) {
	logger := ResponseLogger(eg.Logger, response)
	logger.Debug("开始分发爬虫解析结果")
	defer logger.Debug("结束分发爬虫解析结果")

	for result := range results {
		switch res := result.(type) {
		case *Request:
			eg.signal.Emit(NewRequestLeftEngineSignal(SenderProcessSpiderOutput, res, spider))
		default:
			item := &ItemResponseSignal{
				Item:     res,
				Response: nil,
				Spider:   spider,
			}
			eg.itemSlot.Push(item)
		}
	}
}

// 使用ProcessSpiderError处理爬虫错误
func (eg *EnginerImpl) spiderError(response *Response, e error, spider *Spider) {
	logger := ResponseLogger(eg.Logger, response)
	logger.Debug("开始处理爬虫错误ProcessSpiderError")
	defer logger.Debug("结束处理爬虫错误ProcessSpiderError")

	results, idx, err := spider.spiderManager.ProcessSpiderError(response, e, spider)

	if err != nil {
		LogSpiderModulerError(
			logger, zap.ErrorLevel,
			"ProcessSpiderError方法出错",
			spider.spiderManager.Middlewares()[idx], err)
		eg.signal.Emit(NewErrorUnhandledSignal(SenderProcessSpiderError, response.Request, response, err, spider))
		return
	}

	if results == nil {
		LogSpiderModulerError(
			logger, zap.WarnLevel,
			"ProcessSpiderError方法无法处理该错误",
			spider.spiderManager.Middlewares()[idx], e)
		eg.signal.Emit(NewErrorUnhandledSignal(
			SenderProcessSpiderError, response.Request, response, e, spider))
		return
	}

	eg.signal.Emit(NewResultsLeftSpiderSignal(
		SenderProcessSpiderError, response, results, idx-1, spider))
}

// 使用ProcessError处理下载错误
func (eg *EnginerImpl) downloaderError(request *Request, e error, spider *Spider, sender Sender) {
	logger := RequestLogger(eg.Logger, request)
	logger.Debug("开始处理下载错误ProcessError")
	defer logger.Debug("结束处理下载错误ProcessError")

	result, idx, err := spider.downloaderManager.ProcessError(request, e, spider)
	if err != nil {
		LogSpiderModulerError(
			logger, zap.ErrorLevel,
			"ProcessError方法出错",
			spider.downloaderManager.Middlewares()[idx], err)
		eg.signal.Emit(NewErrorUnhandledSignal(SenderProcessError, request, nil, err, spider))
		return
	}

	if result == nil {
		logger.Warnw("ProcessError方法无法处理该错误",
			"error", e)

		if sender == SenderProcessRequest {
			eg.signal.Emit(NewRequestErrbackSignal(SenderProcessError, request, nil, e, spider))
			return
		}

		eg.signal.Emit(NewErrorUnhandledSignal(
			SenderProcessError, request, nil, e, spider))
		return
	}

	switch res := result.(type) {
	case *Request:
		eg.signal.Emit(NewRequestLeftEngineSignal(SenderProcessError, res, spider))
	case *Response:
		eg.signal.Emit(NewResponseLeftDownloaderSignal(SenderProcessError, request, res, spider))
	default:
		LogSpiderModulerError(
			logger, zap.FatalLevel,
			"ProcessError方法返回类型错误",
			spider.downloaderManager.Middlewares()[idx],
			fmt.Errorf("invalid type: %s", reflect.TypeOf(result).String()))
	}
}

// 调用Request的ErrbackFunc处理错误
func (eg *EnginerImpl) requestErrback(request *Request, response *Response, e error, spider *Spider, sender Sender) {
	var logger *zap.SugaredLogger
	if response != nil {
		logger = ResponseLogger(eg.Logger, response)
	} else {
		logger = RequestLogger(eg.Logger, request)
	}

	logger.Debug("开始处理请求错误Errback")
	defer logger.Debug("结束处理请求错误Errback")

	defer func() {
		if err := recover(); err != nil {
			logger.Errorw("ErrbackFunc方法出错", "error", err)
			if sender == SenderProcessSpiderInput {
				eg.signal.Emit(NewSpiderErrorSignal(SenderRequestErrback, response, e, spider))
			}
		}
	}()

	if request.Errback == "" {
		logger.Warn("未设置ErrbackFunc名称")
		if sender == SenderProcessSpiderInput {
			eg.signal.Emit(NewSpiderErrorSignal(SenderRequestErrback, response, e, spider))
		}
		return
	}
	errback, ok := GetRegisteredByName(request.Errback)
	if !ok {
		logger.Fatal("未注册的ErrbackFunc名称")
	}
	//errbackFunc, ok := errback.(ErrbackFunc)
	errbackFunc, ok := errback.(func(*Request, *Response, error, *Spider) Results)
	if !ok {
		logger.Fatalw("ErrbackFunc类型错误",
			"type", reflect.TypeOf(errback).String())
	}
	results := errbackFunc(request, response, e, spider)
	if results == nil {
		return
	}

	eg.signal.Emit(NewResultsLeftSpiderSignal(
		SenderRequestErrback, response, results, spider.spiderManager.Len()-1, spider))
}

func (eg *EnginerImpl) spiderIdle(spider *Spider) {
	eg.Logger.Debug("爬虫空闲")
	defer eg.Logger.Debug("即将关闭爬虫引擎")

	eg.signal.Emit(NewSpiderClosedSignal(SenderEngine, "finished", spider))
}

func (eg *EnginerImpl) spiderClosed(reason string, spider *Spider) {
	eg.Close(spider)
	eg.Logger.Infow("爬虫引擎关闭",
		"reason", reason)
}
