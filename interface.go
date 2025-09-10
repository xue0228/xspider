package xspider

import (
	"reflect"
	"time"
)

type Results <-chan any
type Result any
type Item any
type Sender string
type SignalType string
type SignalReceiver any

// type SignalReceiver func(Signaler)

type ReceiverConfig struct {
	Receiver     reflect.Value
	Index        int
	SenderFilter []Sender
}
type Requests <-chan *Request

// SpiderModuler 爬虫模块
type SpiderModuler interface {
	// FromSpider 根据Spider初始化模块参数
	FromSpider(*Spider)
	// Close 爬虫关闭时调用此方法以清理资源
	Close(*Spider)
	// Name 组件名称
	Name() string
}

// Scheduler 爬虫调度器
type Scheduler interface {
	SpiderModuler
	// HasPendingRequests 是否有待处理的请求
	HasPendingRequests() bool
	// EnqueueRequest 将请求加入调度器，此处需要实现去重逻辑
	EnqueueRequest(*Request) bool
	// NextRequest 从调度器取出下一个待处理的请求
	NextRequest() *Request
}

// DupeFilter 爬虫去重
type DupeFilter interface {
	SpiderModuler
	// RequestSeen 判断请求是否重复
	RequestSeen(*Request) bool
	// RequestFingerprint 获取请求的指纹
	RequestFingerprint(*Request) string
	Log(*Request)
}

// Downloader 下载器
type Downloader interface {
	SpiderModuler
	// Fetch 下载请求
	Fetch(*Request, *Spider) (*Response, error)
}

// SpiderMiddlewarer 爬虫中间件
type SpiderMiddlewarer interface {
	SpiderModuler
	ProcessStartRequests(Results, *Spider) Results
	ProcessSpiderInput(*Response, *Spider)
	ProcessSpiderOutput(*Response, Results, *Spider) Results
	ProcessSpiderError(*Response, error, *Spider) Results
}

// DownloaderMiddlewarer 下载器中间件
type DownloaderMiddlewarer interface {
	SpiderModuler
	ProcessRequest(*Request, *Spider) Result
	ProcessResponse(*Request, *Response, *Spider) Result
	ProcessError(*Request, error, *Spider) Result
}

// ItemPipeliner 爬虫结果处理中间件
type ItemPipeliner interface {
	SpiderModuler
	ProcessItem(Item, *Response, *Spider) Item
}

// SpiderMiddlewareManager 爬虫中间件管理
type SpiderMiddlewareManager interface {
	SpiderModuler
	Len() int
	Middlewares() []SpiderMiddlewarer
	ProcessStartRequests(Results, *Spider) (Results, int, error)
	ProcessSpiderInput(*Response, *Spider) (int, error)
	ProcessSpiderOutput(*Response, Results, int, *Spider) (Results, int, error)
	ProcessSpiderError(*Response, error, *Spider) (Results, int, error)
}

// DownloaderMiddlewareManager 下载器中间件管理
type DownloaderMiddlewareManager interface {
	SpiderModuler
	Len() int
	Middlewares() []DownloaderMiddlewarer
	ProcessRequest(*Request, *Spider) (Result, int, error)
	ProcessResponse(*Request, *Response, *Spider) (Result, int, error)
	ProcessError(*Request, error, *Spider) (Result, int, error)
}

// ItemPipelineManager 爬虫结果处理中间件管理
type ItemPipelineManager interface {
	SpiderModuler
	Len() int
	ItemPipelines() []ItemPipeliner
	ProcessItem(Item, *Response, *Spider) (Item, int, error)
}

// ExtensionManager 爬虫扩展管理
type ExtensionManager interface {
	SpiderModuler
	Len() int
	Extensions() []Extensioner
}

// NumberString 可以是 int, float64, string
type NumberString any

// NumberOnly 可以是 int, float64
type NumberOnly any

// Statser 爬虫统计
type Statser interface {
	SpiderModuler
	// GetValue 获取统计值
	GetValue(string, NumberString) NumberString

	GetIntValue(string, int) int
	GetFloat64Value(string, float64) float64
	GetStringValue(string, string) string

	// GetStats 获取所有统计值
	GetStats() map[string]NumberString
	// SetValue 设置统计值
	SetValue(string, NumberString)
	// SetStats 设置所有统计值
	SetStats(map[string]NumberString)
	// IncValue 递增统计值，没有该字段时将其设置为start后再递增count
	IncValue(key string, count NumberOnly, start NumberOnly)
	// MaxValue 设置统计值，value比当前值大时才更新
	MaxValue(string, NumberOnly)
	// MinValue 递增统计值，value比当前值小时才更新
	MinValue(string, NumberOnly)
	// Clear 清空统计值
	Clear()
}

type Enginer interface {
	SpiderModuler
	// Start 启动爬虫，阻塞式
	Start(*Spider)
}

// Signaler 事件信号
type Signaler interface {
	// Type 事件类型
	Type() SignalType
	// Sender 发送方
	Sender() Sender
	// Data 事件数据
	Data() []any
}

// SignalManager 信号管理
type SignalManager interface {
	SpiderModuler
	// Connect 关联信号处理函数
	// 参数分别为处理函数、信号类型、信号处理函数的优先级（数值越小越先处理，同优先级以异步的方式同时处理）、发送方过滤器（省略则无过滤）
	Connect(SignalReceiver, SignalType, int, ...Sender)
	// Disconnect 移除信号处理函数
	// 因为函数本身无法比较，所以此处只能通过函数名来移除
	Disconnect(SignalReceiver, SignalType) bool
	// DisconnectAll 移除所有信号处理函数
	DisconnectAll()
	// Emit 发送信号
	Emit(Signaler)
	// Start 启动信号处理循环，非阻塞式
	Start()
	// IsAllDone 判断所有信号处理函数是否执行完毕
	IsAllDone() bool
}

// ResponseSloter 用来限制Response在内存中占用的总体积
type ResponseSloter interface {
	SpiderModuler
	Add(*Response)
	Done(*Response)
	IsFree() bool
}

// ItemSloter 用来限制处理Item的并发数
type ItemSloter interface {
	SpiderModuler
	Push(*ItemResponseSignal)
	Pop() *ItemResponseSignal
	Finish(*ItemResponseSignal)
	IsFree() bool
	IsEmpty() bool
}

// RequestSloter 用来限制下载器处理Request的并发数及时间间隔
type RequestSloter interface {
	SpiderModuler
	// Push 当Request加入下载器时调用此方法
	Push(*Request)
	// Finish 当Request下载完毕时调用此方法
	Finish(*Request)
	// Pop 取出当前时间点所有子slot中能处理的Request
	Pop() Requests
	// IsFree 判断slot是否空闲，为空的子slot不做判定
	IsFree() bool
	// IsEmpty 判断slot是否为空
	IsEmpty() bool
	// Clear 删除内部不活跃时间达到指定时间的子slot资源
	Clear(time.Duration)
}

// PriorityQueuer 优先级队列
type PriorityQueuer interface {
	SpiderModuler
	Push(any, int)
	// Pop 获取队列中第一个元素，并删除
	Pop() any
	// Peek 获取队列中第一个元素，但不删除
	Peek() any
	Len() int
}

type Extensioner interface {
	SpiderModuler
	ConnectSignal(SignalManager, int)
}
