package xspider

import (
	"reflect"
	"sort"

	"go.uber.org/zap"
)

// 下载器中间件管理器
type DownloaderMiddlewareManager struct {
	ModuleName  string
	logger      *zap.SugaredLogger
	middlewares []DownloaderMiddlewarer
	request     []ProcessRequestFunc
	response    []ProcessResponseFunc
	exception   []ProcessExceptionFunc
	ch          chan *Signal
}

func (d *DownloaderMiddlewareManager) FromSpider(spider *Spider) {
	d.ModuleName = "downloader_middleware_manager"
	d.logger = spider.Log.With("module_name", d.ModuleName)
	d.ch = spider.signalChan

	// 合并两个map
	tem := map[int]DownloaderMiddlewarer{}
	for k, v := range spider.Settings.DownloaderMiddlewaresBase {
		tem[k] = v
	}
	for k, v := range spider.Settings.DownloaderMiddlewares {
		tem[k] = v
	}
	// 升序排列
	tem2 := []int{}
	for k := range tem {
		tem2 = append(tem2, k)
	}
	sort.Ints(tem2)
	// 按顺序添加到切片
	for _, v := range tem2 {
		d.middlewares = append(d.middlewares, CopyNew(tem[v]).(DownloaderMiddlewarer))
	}

	for _, v := range d.middlewares {
		v.FromSpider(spider)

		d.request = append(d.request, v.ProcessRequest)
		d.response = append(d.response, v.ProcessResponse)
		d.exception = append(d.exception, v.ProcessException)
	}
}

func (d *DownloaderMiddlewareManager) send(signal *Signal) {
	d.ch <- signal
}

func (d *DownloaderMiddlewareManager) ProcessRequest(signal *Signal, spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case *ErrIgnoreRequest:
				d.logger.Debugw("ProcessRequest方法中出现IgnoreRequest错误",
					"panic_module", d.middlewares[index].GetModuleName(),
					"reason", err)
			default:
				d.logger.Debugw("ProcessRequest方法出错",
					"panic_module", d.middlewares[index].GetModuleName(),
					"error", err)
			}
			d.send(&Signal{From: sProcessRequest, To: sProcessException, Index: index, Body: &Failure{
				Request: signal.Body.(*Request), Spider: spider, Error: err.(error),
			}})
		}
	}()

	request := signal.Body.(*Request)
	var res RequestResponse
	for index = signal.Index; index < len(d.middlewares); index++ {
		res = d.request[index](request, spider)
		if res != nil {
			switch r := res.(type) {
			case *Request:
				d.send(&Signal{From: sProcessRequest, To: sScheduler, Body: r})
			case *Response:
				d.send(&Signal{From: sProcessRequest, To: sProcessResponse, Body: r})
			default:
				d.logger.Fatalw("ProcessRequest返回错误数据类型",
					"panic_module", d.middlewares[index].GetModuleName(),
					"return_type", reflect.TypeOf(r))
			}
			return
		}
	}
	d.send(&Signal{From: sProcessRequest, To: sDownloader, Body: request})
	return
}

func (d *DownloaderMiddlewareManager) ProcessResponse(signal *Signal, spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case *ErrIgnoreRequest:
				d.logger.Debugw("ProcessResponse方法中出现IgnoreRequest错误",
					"panic_module", d.middlewares[index].GetModuleName(),
					"reason", err)
				d.send(&Signal{From: sProcessResponse, To: sProcessException, Index: index, Body: &Failure{
					Request: signal.Body.(*Response).Request, Response: signal.Body.(*Response), Spider: spider, Error: err.(error),
				}})
			default:
				d.logger.Fatalw("ProcessResponse方法出错",
					"panic_module", d.middlewares[index].GetModuleName(),
					"error", err)
			}
		}
	}()

	response := signal.Body.(*Response)
	request := response.Request
	var res RequestResponse
	for index = len(d.middlewares) - 1; index >= 0; index-- {
		res = d.response[index](request, response, spider)
		if res == nil {
			d.logger.Fatalw("ProcessResponse返回错误数据类型",
				"panic_module", d.middlewares[index].GetModuleName(),
				"return_type", reflect.TypeOf(res))
		}
		switch r := res.(type) {
		case *Response:
			response = r
			continue
		case *Request:
			d.send(&Signal{From: sProcessResponse, To: sScheduler, Body: r})
		default:
			d.logger.Fatalw("ProcessResponse返回错误数据类型",
				"panic_module", d.middlewares[index].GetModuleName(),
				"return_type", reflect.TypeOf(r))
		}
		return
	}
	d.send(&Signal{From: sProcessResponse, To: sProcessSpiderInput, Body: response})
	return
}

func (d *DownloaderMiddlewareManager) ProcessException(signal *Signal, spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			d.logger.Fatalw("ProcessException方法出错",
				"panic_module", d.middlewares[index].GetModuleName(),
				"error", err)
		}
	}()

	failure := signal.Body.(*Failure)
	if signal.From == sProcessRequest || signal.From == sDownloader {
		var res RequestResponse
		for index = signal.Index; index < len(d.middlewares); index++ {
			res = d.exception[index](failure.Request, failure.Error, spider)
			if res != nil {
				switch r := res.(type) {
				case *Request:
					d.send(&Signal{From: sProcessException, To: sScheduler, Body: r})
				case *Response:
					d.send(&Signal{From: sProcessException, To: sProcessResponse, Body: r})
				default:
					d.logger.Fatalw("ProcessException返回错误数据类型",
						"panic_module", d.middlewares[index].GetModuleName(),
						"return_type", reflect.TypeOf(r))
				}
				return
			}
		}
		switch failure.Error.(type) {
		case *ErrIgnoreRequest:
			if failure.Request.Errback != nil {
				rs := failure.Request.Errback(failure)
				for _, v := range rs {
					switch r := v.(type) {
					case *Request:
						d.send(&Signal{From: sProcessException, To: sScheduler, Body: r})
					case *Item:
						d.send(&Signal{From: sProcessException, To: sItemPipeline, Body: r})
					}
				}
			}
		}
	} else if signal.From == sProcessResponse {
		if failure.Request.Errback != nil {
			rs := failure.Request.Errback(failure)
			for _, v := range rs {
				switch r := v.(type) {
				case *Request:
					d.send(&Signal{From: sProcessException, To: sScheduler, Body: r})
				case *Item:
					d.send(&Signal{From: sProcessException, To: sItemPipeline, Body: r})
				}
			}
		}
	}
	return
}

// 爬虫中间件管理器
type SpiderMiddlewareManager struct {
	ModuleName  string
	logger      *zap.SugaredLogger
	middlewares []SpiderMiddlewarer
	input       []ProcessSpiderInputFunc
	output      []ProcessSpiderOutputFunc
	exception   []ProcessSpiderExceptionFunc
	start       []ProcessStartRequestsFunc
	ch          chan *Signal
}

func (s *SpiderMiddlewareManager) FromSpider(spider *Spider) {
	s.ModuleName = "spider_middleware_manager"
	s.logger = spider.Log.With("module_name", s.ModuleName)
	s.ch = spider.signalChan

	// 合并两个map
	tem := map[int]SpiderMiddlewarer{}
	for k, v := range spider.Settings.SpiderMiddlewaresBase {
		tem[k] = v
	}
	for k, v := range spider.Settings.SpiderMiddlewares {
		tem[k] = v
	}
	// 升序排列
	tem2 := []int{}
	for k := range tem {
		tem2 = append(tem2, k)
	}
	sort.Ints(tem2)
	// 按顺序添加到切片
	for _, v := range tem2 {
		s.middlewares = append(s.middlewares, CopyNew(tem[v]).(SpiderMiddlewarer))
	}

	for _, v := range s.middlewares {
		v.FromSpider(spider)

		s.input = append(s.input, v.ProcessSpiderInput)
		s.output = append(s.output, v.ProcessSpiderOutput)
		s.exception = append(s.exception, v.ProcessSpiderException)
		s.start = append(s.start, v.ProcessStartRequests)
	}
}

func (s *SpiderMiddlewareManager) send(signal *Signal) {
	s.ch <- signal
}

func (s *SpiderMiddlewareManager) ProcessSpiderInput(signal *Signal, spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Debugw("ProcessSpiderInput方法出错",
				"panic_module", s.middlewares[index].GetModuleName(),
				"error", err)
			s.send(&Signal{From: sProcessSpiderInput, To: sProcessSpiderException, Index: index, Body: &Failure{
				Request: signal.Body.(*Response).Request, Response: signal.Body.(*Response), Spider: spider, Error: err.(error),
			}})
		}
	}()

	response := signal.Body.(*Response)
	for index = signal.Index; index < len(s.middlewares); index++ {
		s.input[index](response, spider)
	}
	s.send(&Signal{From: sProcessSpiderInput, To: sSpider, Body: response})
	return
}

type SpiderOutputData struct {
	Response *Response
	Result   RequestItems
}

func (s *SpiderMiddlewareManager) ProcessSpiderOutput(signal *Signal, spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Debugw("ProcessSpiderOutput方法出错",
				"panic_module", s.middlewares[index].GetModuleName(),
				"error", err)
			s.send(&Signal{From: sProcessSpiderOutput, To: sProcessSpiderException, Index: index, Body: &Failure{
				Request: signal.Body.(*SpiderOutputData).Response.Request, Response: signal.Body.(*SpiderOutputData).Response, Spider: spider, Error: err.(error),
			}})
		}
	}()

	data := signal.Body.(*SpiderOutputData)
	var res RequestItems
	var ind int
	if signal.From == sSpider {
		ind = len(s.middlewares) - 1
	} else {
		ind = signal.Index
	}

	for index = ind; index >= 0; index-- {
		res = s.output[index](data.Response, data.Result, spider)

		if res == nil {
			s.logger.Fatalw("ProcessSpiderOutput返回错误数据类型",
				"panic_module", s.middlewares[index].GetModuleName(),
				"return_type", reflect.TypeOf(res))
			return
		} else {
			data.Result = res
		}
	}
	for _, v := range data.Result {
		switch r := v.(type) {
		case *Request:
			s.send(&Signal{From: sProcessSpiderOutput, To: sScheduler, Body: r})
		case Item:
			s.send(&Signal{From: sProcessSpiderOutput, To: sItemPipeline, Body: r})
		default:
			s.logger.Fatalw("ProcessSpiderOutput返回错误数据类型",
				// "panic_module", s.middlewares[index].GetModuleName(),
				"return_type", reflect.TypeOf(res))
		}
	}
	return
}

func (s *SpiderMiddlewareManager) ProcessSpiderException(signal *Signal, spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Fatalw("ProcessSpiderException方法出错",
				"panic_module", s.middlewares[index].GetModuleName(),
				"error", err)
		}
	}()

	failure := signal.Body.(*Failure)
	if signal.From == sProcessSpiderInput {
		if failure.Request.Errback != nil {
			rs := failure.Request.Errback(failure)
			if rs != nil {
				s.send(&Signal{From: sProcessSpiderException, To: sProcessSpiderOutput, Index: len(s.middlewares) - 1, Body: rs})
			}
		} else {
			var res RequestItems
			for index = signal.Index; index < len(s.middlewares); index++ {
				if res != nil {
					s.send(&Signal{From: sProcessSpiderException, To: sProcessSpiderOutput, Index: len(s.middlewares) - 1, Body: res})
					return
				}
			}
		}
	} else if signal.From == sProcessSpiderOutput {
		var res RequestItems
		for index = signal.Index; index >= 0; index-- {
			res = s.exception[index](failure.Response, failure.Error, spider)
			if res != nil {
				s.send(&Signal{From: sProcessSpiderException, To: sProcessSpiderOutput, Index: signal.Index - 1, Body: res})
				return
			}
		}
	}
	return
}

func (s *SpiderMiddlewareManager) ProcessStartRequests(signal *Signal, spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Fatalw("ProcessStartRequests方法出错",
				"panic_module", s.middlewares[index].GetModuleName(),
				"error", err)
		}
	}()

	requests := signal.Body.([]*Request)
	var res []*Request
	for index = signal.Index; index < len(s.middlewares); index++ {
		res = s.start[index](requests, spider)
		if res != nil {
			requests = res
			continue
		} else {
			s.logger.Fatalw("ProcessStartRequests返回错误数据类型",
				"panic_module", s.middlewares[index].GetModuleName(),
				"return_type", reflect.TypeOf(res))
		}
	}
	for _, v := range requests {
		s.send(&Signal{From: sProcessStartRequests, To: sScheduler, Body: v})
	}
	return
}
