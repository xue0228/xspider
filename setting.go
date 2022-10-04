package xspider

import (
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/kennygrant/sanitize"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Setting struct {
	// 爬虫机器人名称
	// BotName string
	// 是否启用下载状态记录功能
	DownloadStats bool
	// 下载超时时间
	DownloadTimeout time.Duration
	// 单个域名下每个Request的请求间隔
	DownloadDelay time.Duration
	//启用后实际请求间隔会在0.5到1.5倍的DownloadDelay之间随机选取
	RandomizeDownloadDelay bool
	//单个Request最大下载数据量
	DownloadMaxSize int
	//单个Request开始在日志中输出警报信息的下载数据量
	DownloadWarnSize int
	//单个页面允许爬取的最大深度，为0代表无限制
	DepthLimit int
	//用来根据请求深度调整Request中Priority值的整数
	//调整计算公式如下：
	//Request.Priority = Request.Priority - ( Request.Depth * DepthPriority )
	DepthPriority int
	// 是否启用request深度相关记录功能
	DepthStatsVerbose bool
	//是否自动重试
	RetryEnabled bool
	//除第一次外最大重试次数
	MaxRetryTimes int
	//自动重试的状态码
	RetryHttpCodes []int
	// 重试请求优先级的调整参数
	// request.Priority += priorityAdjust
	RetryPriorityAdjust int

	// RedirectEnabled             bool
	// MaxRedirectTimes            int
	// RedirectPriorityAdjust      int

	//是否对Request进行去重过滤
	FilterEnabled bool
	//同时处理的最大Item个数
	ConcurrentItems int
	//爬虫下载器同时下载的最大Request个数
	ConcurrentRequests int
	//单个域名允许同时访问的请求个数
	ConcurrentRequestsPerDomain int
	// 允许下载数据占用的最大内存
	ResponseMaxActiveSize int
	// 是否启用日志功能
	LogEnabled bool
	// 除终端外额外将日志内容保存到指定文件
	// 指定ErrFile时仅将Warning以下的日志信息保存到该文件
	LogFile string
	// 指定时会将Warning及以上的日志信息保存到指定文件
	// 仅指定ErrFile时，Warning以下的日志信息仅在终端显示而不保存
	ErrFile string
	// 日志显示及保存的最低日志等级
	LogLevel zapcore.Level
	// url最大长度限制
	UrlLengthLimit int

	HttpErrorAllowAll     bool
	HttpErrorAllowedCodes []int
	// 默认请求头
	DefaultRequestHeaders http.Header
	// 默认UserAgent
	UserAgent string
	// 请求过滤器
	FilterClass DupeFilter
	// 调度器优先级队列
	SchedulerPriorityQueue PriorityQueuer
	// 用户自定义的设置项
	ExtensionSettings *Context

	DownloaderMiddlewaresBase map[int]DownloaderMiddlewarer

	DownloaderMiddlewares map[int]DownloaderMiddlewarer

	SpiderMiddlewaresBase map[int]SpiderMiddlewarer

	SpiderMiddlewares map[int]SpiderMiddlewarer

	ItemPipelinesBase map[int]ItemPipeliner

	ItemPipelines map[int]ItemPipeliner

	SchedulerClass Scheduler

	DownloaderClass Downloader

	HttpUser       string
	HttpPass       string
	HttpAuthDomain []string
}

type RequestItems []interface{}

// ParseFunc 用户自定义用于解析Response的函数
type ParseFunc func(*Response, *Spider) RequestItems

// ErrorbackFunc Request请求失败后调用的回调函数
type ErrorbackFunc func(*Failure) RequestItems

func (s *Setting) Init() {
	// s.BotName = "xspider"
	s.DownloadStats = true
	s.DownloadTimeout = 180 * time.Second
	s.DownloadDelay = time.Duration(0)
	s.RandomizeDownloadDelay = true
	s.DownloadMaxSize = 1024 * 1024 * 1024
	s.DownloadWarnSize = 32 * 1024 * 1024
	s.DepthLimit = 10
	s.DepthPriority = 0
	s.DepthStatsVerbose = true
	s.RetryEnabled = true
	s.MaxRetryTimes = 5
	s.RetryHttpCodes = []int{500, 502, 503, 504, 522, 524, 408, 429}
	s.RetryPriorityAdjust = -1
	s.FilterEnabled = true
	s.ConcurrentItems = 100
	s.ConcurrentRequests = 16
	s.ConcurrentRequestsPerDomain = 8
	s.ResponseMaxActiveSize = 5000000
	s.LogEnabled = true
	s.LogFile = ""
	s.ErrFile = ""
	s.LogLevel = zap.InfoLevel
	s.UrlLengthLimit = 2083
	s.HttpErrorAllowAll = false
	s.HttpErrorAllowedCodes = []int{}
	header := http.Header{}
	header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	header.Add("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	s.DefaultRequestHeaders = header
	s.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36 Edg/97.0.1072.76"
	s.FilterClass = &DefaultDupeFilter{}
	s.SchedulerPriorityQueue = &LifoPriorityQueue{}
	s.DownloaderMiddlewaresBase = map[int]DownloaderMiddlewarer{300: &HttpAuthMiddleware{}, 350: &DownloadTimeoutMiddleware{}, 500: &UserAgentMiddleware{}, 550: &RetryMiddleware{}, 850: &DownloadStatsMiddleware{}}
	s.DownloaderMiddlewares = map[int]DownloaderMiddlewarer{}
	s.SpiderMiddlewaresBase = map[int]SpiderMiddlewarer{50: &HttpErrorMiddleware{}, 800: &UrlLengthMiddleware{}, 900: &DepthMiddleware{}}
	s.SpiderMiddlewares = map[int]SpiderMiddlewarer{}
	s.ItemPipelinesBase = map[int]ItemPipeliner{}
	s.ItemPipelines = map[int]ItemPipeliner{}
	s.SchedulerClass = &DefaultScheduler{}
	s.DownloaderClass = &DefaultDownloader{}
	s.ExtensionSettings = NewContext()
}

// func (s *Setting) parseSettingsFromEnv() {}

func NewSetting(options ...func(*Setting)) *Setting {
	s := &Setting{}
	s.Init()

	for _, f := range options {
		f(s)
	}

	// todo：从环境变量中获取参数
	// s.parseSettingsFromEnv()

	// if s.LogEnabled {
	// 	s.Log = InitLog(s.LogFile, s.ErrFile, s.LogLevel)
	// }
	return s
}

// func BotName(name string) func(*Setting) {
// 	return func(s *Setting) {
// 		s.BotName = name
// 	}
// }

func DownloadStats(stats bool) func(*Setting) {
	return func(s *Setting) {
		s.DownloadStats = stats
	}
}

func DownloadTimeout(t time.Duration) func(*Setting) {
	return func(s *Setting) {
		s.DownloadTimeout = t
	}
}

func DownloadDelay(t time.Duration) func(*Setting) {
	return func(s *Setting) {
		s.DownloadDelay = t
	}
}

func RandomizeDownloadDelay(delay bool) func(*Setting) {
	return func(s *Setting) {
		s.RandomizeDownloadDelay = delay
	}
}

func DownloadMaxSize(size int) func(*Setting) {
	return func(s *Setting) {
		s.DownloadMaxSize = size
	}
}

func DownloadWarnSize(size int) func(*Setting) {
	return func(s *Setting) {
		s.DownloadWarnSize = size
	}
}

func DepthLimit(depth int) func(*Setting) {
	return func(s *Setting) {
		s.DepthLimit = depth
	}
}

func DepthPriority(depth int) func(*Setting) {
	return func(s *Setting) {
		s.DepthPriority = depth
	}
}

func DepthStatsVerbose(depth bool) func(*Setting) {
	return func(s *Setting) {
		s.DepthStatsVerbose = depth
	}
}

func RetryEnabled(retry bool) func(*Setting) {
	return func(s *Setting) {
		s.RetryEnabled = retry
	}
}

func MaxRetryTimes(retry int) func(*Setting) {
	return func(s *Setting) {
		s.MaxRetryTimes = retry
	}
}

func RetryHttpCodes(retry []int) func(*Setting) {
	return func(s *Setting) {
		s.RetryHttpCodes = retry
	}
}

func RetryPriorityAdjust(retry int) func(*Setting) {
	return func(s *Setting) {
		s.RetryPriorityAdjust = retry
	}
}

func FilterEnabled(filter bool) func(*Setting) {
	return func(s *Setting) {
		s.FilterEnabled = filter
	}
}

func ConcurrentItems(items int) func(*Setting) {
	return func(s *Setting) {
		s.ConcurrentItems = items
	}
}

func ConcurrentRequests(requests int) func(*Setting) {
	return func(s *Setting) {
		s.ConcurrentRequests = requests
	}
}

func ConcurrentRequestsPerDomain(requests int) func(*Setting) {
	return func(s *Setting) {
		s.ConcurrentRequestsPerDomain = requests
	}
}

func ResponseMaxActiveSize(size int) func(*Setting) {
	return func(s *Setting) {
		s.ResponseMaxActiveSize = size
	}
}

func LogEnabled(log bool) func(*Setting) {
	return func(s *Setting) {
		s.LogEnabled = log
	}
}

func LogFile(log string) func(*Setting) {
	return func(s *Setting) {
		s.LogFile = log
	}
}

func ErrFile(file string) func(*Setting) {
	return func(s *Setting) {
		s.ErrFile = file
	}
}

func LogLevel(log zapcore.Level) func(*Setting) {
	return func(s *Setting) {
		s.LogLevel = log
	}
}

func UrlLengthLimit(limits int) func(*Setting) {
	return func(s *Setting) {
		s.UrlLengthLimit = limits
	}
}

func HttpErrorAllowAll(http bool) func(*Setting) {
	return func(s *Setting) {
		s.HttpErrorAllowAll = http
	}
}

func HttpErrorAllowedCodes(http []int) func(*Setting) {
	return func(s *Setting) {
		s.HttpErrorAllowedCodes = http
	}
}

func DefaultRequestHeaders(headers http.Header) func(*Setting) {
	return func(s *Setting) {
		s.DefaultRequestHeaders = headers
	}
}

func UserAgent(ua string) func(*Setting) {
	return func(s *Setting) {
		s.UserAgent = ua
	}
}

func DownloaderMiddlewaresBase(d map[int]DownloaderMiddlewarer) func(*Setting) {
	return func(s *Setting) {
		s.DownloaderMiddlewaresBase = d
	}
}

func DownloaderMiddlewares(d map[int]DownloaderMiddlewarer) func(*Setting) {
	return func(s *Setting) {
		s.DownloaderMiddlewares = d
	}
}

func SpiderMiddlewaresBase(m map[int]SpiderMiddlewarer) func(*Setting) {
	return func(s *Setting) {
		s.SpiderMiddlewaresBase = m
	}
}

func ItemPipelinesBase(i map[int]ItemPipeliner) func(*Setting) {
	return func(s *Setting) {
		s.ItemPipelinesBase = i
	}
}

func ItemPipelines(i map[int]ItemPipeliner) func(*Setting) {
	return func(s *Setting) {
		s.ItemPipelines = i
	}
}

func SpiderMiddlewares(m map[int]SpiderMiddlewarer) func(*Setting) {
	return func(s *Setting) {
		s.SpiderMiddlewares = m
	}
}

func SchedulerPriorityQueue(pq PriorityQueuer) func(*Setting) {
	return func(s *Setting) {
		s.SchedulerPriorityQueue = pq
	}
}

func FilterClass(f DupeFilter) func(*Setting) {
	return func(s *Setting) {
		s.FilterClass = f
	}
}

func SchedulerClass(c Scheduler) func(*Setting) {
	return func(s *Setting) {
		s.SchedulerClass = c
	}
}

func DownloaderClass(c Downloader) func(*Setting) {
	return func(s *Setting) {
		s.DownloaderClass = c
	}
}

func HttpUser(h string) func(*Setting) {
	return func(s *Setting) {
		s.HttpUser = h
	}
}

func HttpPass(h string) func(*Setting) {
	return func(s *Setting) {
		s.HttpPass = h
	}
}

func HttpAuthDomain(h []string) func(*Setting) {
	return func(s *Setting) {
		s.HttpAuthDomain = h
	}
}

// SanitizeFileName replaces dangerous characters in a string
// so the return value can be used as a safe file name.
func SanitizeFileName(fileName string) string {
	ext := filepath.Ext(fileName)
	cleanExt := sanitize.BaseName(ext)
	if cleanExt == "" {
		cleanExt = ".unknown"
	}
	return strings.Replace(fmt.Sprintf(
		"%s.%s",
		sanitize.BaseName(fileName[:len(fileName)-len(ext)]),
		cleanExt[1:],
	), "-", "_", -1)
}

// 根据对象类型创建一个新对象
func CopyNew(old interface{}) interface{} {
	switch reflect.ValueOf(old).Kind() {
	case reflect.Ptr:
		return reflect.New(reflect.TypeOf(old).Elem()).Interface()
	default:
		return reflect.New(reflect.TypeOf(old)).Interface()
	}
}
