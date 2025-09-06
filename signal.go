package xspider

type Signal struct {
	typ    SignalType
	sender Sender
	data   []any
}

func (s *Signal) Type() SignalType {
	return s.typ
}

func (s *Signal) Sender() Sender {
	return s.sender
}

func (s *Signal) Data() []any {
	return s.data
}

func NewSignal(typ SignalType, sender Sender, data ...any) *Signal {
	return &Signal{typ: typ, sender: sender, data: data}
}

const (
	SenderEngine               Sender = "engine"
	SenderSpider               Sender = "spider"
	SenderScheduler            Sender = "scheduler"
	SenderDownloader           Sender = "downloader"
	SenderItemPipeline         Sender = "item_pipeline"
	SenderProcessStartRequests Sender = "process_start_requests"
	SenderProcessSpiderOutput  Sender = "process_spider_output"
	SenderProcessSpiderError   Sender = "process_spider_error"
	SenderProcessSpiderInput   Sender = "process_spider_input"
	SenderProcessRequest       Sender = "process_request"
	SenderProcessResponse      Sender = "process_response"
	SenderProcessError         Sender = "process_error"
	SenderRequestErrback       Sender = "request_errback"
)

const (
	// StStartsLeftSpider Spider和SpiderMiddleware之间
	StStartsLeftSpider SignalType = "starts_left_spider"
	// StStartsLeftSpiderMiddleware SpiderMiddleware和Engine之间
	StStartsLeftSpiderMiddleware SignalType = "starts_left_spider_middleware"
	// StRequestLeftEngine Engine和Scheduler之间
	StRequestLeftEngine SignalType = "request_left_engine"
	// StItemLeftEngine Engine和ItemPipeline之间
	StItemLeftEngine SignalType = "item_left_engine"
	// StRequestLeftScheduler Scheduler和Engine之间
	StRequestLeftScheduler SignalType = "request_left_scheduler"
	// StRequestReachedDownloaderMiddleware Engine和DownloaderMiddleware之间
	StRequestReachedDownloaderMiddleware SignalType = "request_reached_downloader_middleware"
	// StRequestReachedDownloader DownloaderMiddleware和Downloader之间
	StRequestReachedDownloader SignalType = "request_reached_downloader"
	// StResponseLeftDownloader Downloader和DownloaderMiddleware之间
	StResponseLeftDownloader SignalType = "response_left_downloader"
	// StResponseLeftDownloaderMiddleware DownloaderMiddleware和Engine之间
	StResponseLeftDownloaderMiddleware SignalType = "response_left_downloader_middleware"
	// StResponseReachedSpiderMiddleware Engine和SpiderMiddleware之间
	StResponseReachedSpiderMiddleware SignalType = "response_reached_spider_middleware"
	// StResponseReachedSpider SpiderMiddleware和Spider之间
	StResponseReachedSpider SignalType = "response_reached_spider"
	// StResultsLeftSpider Spider和SpiderMiddleware之间
	StResultsLeftSpider SignalType = "results_left_spider"
	// StResultsLeftSpiderMiddleware SpiderMiddleware和Engine之间
	StResultsLeftSpiderMiddleware SignalType = "results_left_spider_middleware"

	StSpiderError     SignalType = "spider_error"
	StErrorUnhandled  SignalType = "error_unhandled"
	StDownloaderError SignalType = "downloader_error"
	StRequestErrback  SignalType = "request_errback"
	StRequestDropped  SignalType = "request_dropped"

	// StSpiderOpened Spider开始运行
	StSpiderOpened SignalType = "spider_opened"
	// StSpiderIdle Spider空闲
	StSpiderIdle SignalType = "spider_idle"
	// StSpiderClosed Spider结束运行
	StSpiderClosed SignalType = "spider_closed"

	// StItemDropped Item被丢弃
	StItemDropped SignalType = "item_dropped"
	// StItemError Item报错
	StItemError SignalType = "item_error"
	// StItemScraped Item被成功处理
	StItemScraped SignalType = "item_scraped"
)

//type ResultsSignal struct {
//	Results Results `signal:"0"`
//	Spider  *Spider `signal:"1"`
//}
//
//type RequestSignal struct {
//	Request *Request `signal:"0"`
//	Spider  *Spider  `signal:"1"`
//}
//
//type RequestResponseSignal struct {
//	Request  *Request  `signal:"0"`
//	Response *Response `signal:"1"`
//	Spider   *Spider   `signal:"2"`
//}
//
//type RequestErrorSignal struct {
//	Request *Request `signal:"0"`
//	Error   error    `signal:"1"`
//	Spider  *Spider  `signal:"2"`
//}
//
//type RequestResponseErrorSignal struct {
//	Request  *Request  `signal:"0"`
//	Response *Response `signal:"1"`
//	Error    error     `signal:"2"`
//	Spider   *Spider   `signal:"3"`
//}

//type ItemResponseSignal struct {
//	Item     any       `signal:"0"`
//	Response *Response `signal:"1"`
//	Spider   *Spider   `signal:"2"`
//}

//type ItemResponseErrorSignal struct {
//	Item     any       `signal:"0"`
//	Response *Response `signal:"1"`
//	Error    error     `signal:"2"`
//	Spider   *Spider   `signal:"3"`
//}
//
//type ResponseSignal struct {
//	Response *Response `signal:"0"`
//	Spider   *Spider   `signal:"1"`
//}
//
//type ResponseResultsSignal struct {
//	Response *Response `signal:"0"`
//	Results  Results   `signal:"1"`
//	Spider   *Spider   `signal:"2"`
//}
//
//type ResponseErrorSignal struct {
//	Response *Response `signal:"0"`
//	Error    error     `signal:"1"`
//	Spider   *Spider   `signal:"2"`
//}
//
//type ResponseResultsIndexSignal struct {
//	Response *Response `signal:"0"`
//	Results  Results   `signal:"1"`
//	Index    int       `signal:"2"`
//	Spider   *Spider   `signal:"3"`
//}
//
//type SpiderOnlySignal struct {
//	Spider *Spider `signal:"0"`
//}
//
//type ReasonSignal struct {
//	Reason string  `signal:"0"`
//	Spider *Spider `signal:"1"`
//}

// ReadSignal 将信号数据按 signal 标签填充到结构体字段
// 该函数使用反射机制，根据结构体字段上的 `signal` 标签指定的索引位置，
// 从信号的 Data() 中提取对应的数据并填充到目标结构体的字段中
//func ReadSignal(signal Signaler, dest interface{}) error {
//	// 获取信号中的数据切片
//	data := signal.Data()
//
//	// 使用反射检查目标参数是否为指针且指向结构体
//	v := reflect.ValueOf(dest)
//	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
//		return fmt.Errorf("dest must be a pointer to struct, got %v", v.Kind()) // 如果不是结构体指针，返回错误
//	}
//
//	// 获取结构体的实际值（解引用）
//	v = v.Elem()
//	// 获取结构体的类型信息
//	t := v.Type()
//
//	// 遍历结构体的所有字段
//	for i := 0; i < v.NumField(); i++ {
//		// 获取字段的类型信息
//		field := t.Field(i)
//		// 获取字段上的 signal 标签值（表示数据在 data 切片中的索引）
//		indexTag := field.Tag.Get("signal")
//		if indexTag == "" {
//			continue // 如果没有 signal 标签，则跳过该字段
//		}
//
//		// 将标签值转换为整数索引
//		index, err := strconv.Atoi(indexTag)
//		// 检查索引是否有效（非负且不超过 data 切片长度）
//		if err != nil {
//			return fmt.Errorf("invalid signal tag '%s' on field %s: %v", indexTag, field.Name, err)
//		}
//		if index < 0 || index >= len(data) {
//			return fmt.Errorf("index %d out of range for field %s, data length is %d", index, field.Name, len(data))
//		}
//
//		// 检查字段是否可以设置（即首字母大写的可导出字段）
//		if v.Field(i).CanSet() {
//			// 将 data 中对应索引的数据设置到结构体字段中
//			v.Field(i).Set(reflect.ValueOf(data[index]))
//		}
//	}
//
//	// 成功处理所有带标签的字段后返回nil
//	return nil
//}

func NewSpiderOpenedSignal(sender Sender, spider *Spider) *Signal {
	return NewSignal(StSpiderOpened, sender, spider)
}

func NewStartsLeftSpiderSignal(sender Sender, starts Results, spider *Spider) *Signal {
	return NewSignal(StStartsLeftSpider, sender, starts, spider)
}

func NewStartsLeftSpiderMiddlewareSignal(sender Sender, starts Results, spider *Spider) *Signal {
	return NewSignal(StStartsLeftSpiderMiddleware, sender, starts, spider)
}

func NewRequestLeftEngineSignal(sender Sender, request *Request, spider *Spider) *Signal {
	return NewSignal(StRequestLeftEngine, sender, request, spider)
}

func NewItemLeftEngineSignal(sender Sender, item any, response *Response, spider *Spider) *Signal {
	return NewSignal(StItemLeftEngine, sender, item, response, spider)
}

func NewItemDroppedSignal(sender Sender, item any, response *Response, err error, spider *Spider) *Signal {
	return NewSignal(StItemDropped, sender, item, response, err, spider)
}

func NewItemErrorSignal(sender Sender, item any, response *Response, err error, spider *Spider) *Signal {
	return NewSignal(StItemError, sender, item, response, err, spider)
}

func NewItemScrapedSignal(sender Sender, item any, response *Response, spider *Spider) *Signal {
	return NewSignal(StItemScraped, sender, item, response, spider)
}

func NewSpiderErrorSignal(sender Sender, response *Response, err error, spider *Spider) *Signal {
	return NewSignal(StSpiderError, sender, response, err, spider)
}

func NewResultsLeftSpiderMiddlewareSignal(sender Sender, response *Response, results Results, spider *Spider) *Signal {
	return NewSignal(StResultsLeftSpiderMiddleware, sender, response, results, spider)
}

func NewResultsLeftSpiderSignal(sender Sender, response *Response, results Results, index int, spider *Spider) *Signal {
	return NewSignal(StResultsLeftSpider, sender, response, results, index, spider)
}

func NewErrorUnhandledSignal(sender Sender, request *Request, response *Response, err error, spider *Spider) *Signal {
	return NewSignal(StErrorUnhandled, sender, request, response, err, spider)
}

func NewResponseReachedSpiderSignal(sender Sender, response *Response, spider *Spider) *Signal {
	return NewSignal(StResponseReachedSpider, sender, response, spider)
}

func NewResponseReachedSpiderMiddlewareSignal(sender Sender, response *Response, spider *Spider) *Signal {
	return NewSignal(StResponseReachedSpiderMiddleware, sender, response, spider)
}

func NewRequestReachedDownloaderMiddlewareSignal(sender Sender, request *Request, spider *Spider) *Signal {
	return NewSignal(StRequestReachedDownloaderMiddleware, sender, request, spider)
}

func NewResponseLeftDownloaderSignal(sender Sender, request *Request, response *Response, spider *Spider) *Signal {
	return NewSignal(StResponseLeftDownloader, sender, request, response, spider)
}

func NewDownloaderErrorSignal(sender Sender, request *Request, err error, spider *Spider) *Signal {
	return NewSignal(StDownloaderError, sender, request, err, spider)
}

func NewRequestReachedDownloaderSignal(sender Sender, request *Request, spider *Spider) *Signal {
	return NewSignal(StRequestReachedDownloader, sender, request, spider)
}

func NewRequestDroppedSignal(sender Sender, request *Request, err error, spider *Spider) *Signal {
	return NewSignal(StRequestDropped, sender, request, err, spider)
}

func NewResponseLeftDownloaderMiddlewareSignal(sender Sender, response *Response, spider *Spider) *Signal {
	return NewSignal(StResponseLeftDownloaderMiddleware, sender, response, spider)
}

func NewRequestLeftSchedulerSignal(sender Sender, request *Request, spider *Spider) *Signal {
	return NewSignal(StRequestLeftScheduler, sender, request, spider)
}

func NewSpiderIdleSignal(sender Sender, spider *Spider) *Signal {
	return NewSignal(StSpiderIdle, sender, spider)
}

func NewSpiderClosedSignal(sender Sender, reason string, spider *Spider) *Signal {
	return NewSignal(StSpiderClosed, sender, reason, spider)
}

func NewRequestErrbackSignal(sender Sender, request *Request, response *Response, err error, spider *Spider) *Signal {
	return NewSignal(StRequestErrback, sender, request, response, err, spider)
}
