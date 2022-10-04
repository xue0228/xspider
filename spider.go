package xspider

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

type Spider struct {
	// 该spider的名称，用于日志记录
	Name string
	// 可选。包含了spider允许爬取的域名(domain)列表(list)。
	// 当OffsiteMiddleware启用时，域名不在列表中的URL不会被跟进。
	AllowedDomains []string
	// URL列表。当没有制定特定的URL时，spider将从该列表中开始进行爬取。
	// 因此，第一个被获取到的页面的URL将是该列表之一。
	// 后续的URL将会从获取到的数据中提取。
	StartUrls []string
	// 用于生成该爬虫爬取的起始Request，默认使用StartUrls中的链接生成Request
	StartRequestsFunc func(*Spider) []*Request
	// 当response没有指定回调函数时，该方法是xspider处理下载的response的默认方法。
	DefaultParseFunc ParseFunc
	// 爬取结束后执行的自定义函数
	CloseFunc func(*Spider)
	// 爬虫的设置参数，多个爬虫可共用一个相同的设置
	Settings *Setting
	// 日志记录器
	Log *zap.SugaredLogger
	// 爬虫状态记录器
	Stats StatsCollector

	signalChan chan *Signal
}

func Name(name string) func(*Spider) {
	return func(s *Spider) {
		s.Name = name
	}
}

func AllowedDomains(domains []string) func(*Spider) {
	return func(s *Spider) {
		s.AllowedDomains = domains
	}
}

func StartUrls(urls []string) func(*Spider) {
	return func(s *Spider) {
		s.StartUrls = urls
	}
}

func StartRequestsFunc(fc func(*Spider) []*Request) func(*Spider) {
	return func(s *Spider) {
		s.StartRequestsFunc = fc
	}
}

func DefaultParseFunc(fc ParseFunc) func(*Spider) {
	return func(s *Spider) {
		s.DefaultParseFunc = fc
	}
}

func CloseFunc(fc func(*Spider)) func(*Spider) {
	return func(s *Spider) {
		s.CloseFunc = fc
	}
}

func Settings(settings *Setting) func(*Spider) {
	return func(s *Spider) {
		s.Settings = settings
	}
}

func DefaultStartRequests(s *Spider) []*Request {
	ret := make([]*Request, 0)
	for _, v := range s.StartUrls {
		r, err := NewRequest("GET", v, nil)
		if err != nil {
			panic(err)
		}
		ret = append(ret, r)
	}
	return ret
}

func (s *Spider) Init() {
	s.Name = "xspider"
	s.Settings = NewSetting()
	s.StartRequestsFunc = DefaultStartRequests
}

func NewSpider(options ...func(*Spider)) *Spider {
	s := &Spider{}
	s.Init()

	for _, f := range options {
		f(s)
	}

	return s
}

func (s *Spider) add(module string) {
	s.Stats.IncValue(fmt.Sprintf("goroutine/%s", module), 1, 0)
	s.Stats.IncValue("goroutine/total", 1, 0)
}

func (s *Spider) done() {
	s.Stats.IncValue("goroutine/done", 1, 0)
}

func (s *Spider) isAllDone() bool {
	return s.Stats.GetValue("goroutine/total", 0) == s.Stats.GetValue("goroutine/done", 0)
}

func (s *Spider) Run() {
	// 初始化对象参数
	s.signalChan = make(chan *Signal)
	s.Log = InitLog(s.Settings.LogFile, s.Settings.ErrFile, s.Settings.LogLevel).With("name", s.Name)
	s.Stats = NewStatsCollector()

	// 创建对象
	scheduler := CopyNew(s.Settings.SchedulerClass).(Scheduler)
	scheduler.FromSpider(s)
	downloader := CopyNew(s.Settings.DownloaderClass).(Downloader)
	downloader.FromSpider(s)
	responseSlot := &ResponseSlot{}
	responseSlot.FromSpider(s)
	itemSlot := &ItemSlot{}
	itemSlot.FromSpider(s)
	downloaderMiddlewareManager := &DownloaderMiddlewareManager{}
	downloaderMiddlewareManager.FromSpider(s)
	spiderMiddlewareManager := &SpiderMiddlewareManager{}
	spiderMiddlewareManager.FromSpider(s)
	itemPipelineManager := &ItemPipelineManager{}
	itemPipelineManager.FromSpider(s)
	itemPipelineManager.OpenSpider(s)

	// 生成初始请求
	requests := s.StartRequestsFunc(s)
	if requests == nil {
		s.Log.Warn("起始请求为空")
		return
	} else {
		go func() {
			s.signalChan <- &Signal{From: sSpider, To: sProcessStartRequests, Body: requests}
		}()
	}

	// 处理信号调度
	go func() {
		for {
			signal := <-s.signalChan
			switch signal.To {
			case sProcessStartRequests:
				s.add("process_start_requests")
				go func() {
					defer func() {
						s.done()
					}()

					spiderMiddlewareManager.ProcessStartRequests(signal, s)
				}()

			case sScheduler:
				s.add("scheduler_enqueue_request")
				go func() {
					defer func() {
						s.done()
					}()

					request := signal.Body.(*Request)
					scheduler.EnqueueRequest(request)
				}()
			case sProcessRequest:
				s.add("process_request")
				go func() {
					defer func() {
						s.done()
					}()

					downloaderMiddlewareManager.ProcessRequest(signal, s)
				}()

			case sDownloader:
				s.add("process_downloader")
				go func() {
					defer func() {
						s.done()
					}()

					downloader.ProcessDownloader(signal, s)
				}()
			case sProcessResponse:
				s.add("process_response")
				go func() {
					defer func() {
						s.done()
					}()

					downloaderMiddlewareManager.ProcessResponse(signal, s)
				}()
			case sProcessSpiderInput:
				s.add("process_spider_input")
				go func() {
					defer func() {
						s.done()
					}()

					spiderMiddlewareManager.ProcessSpiderInput(signal, s)
				}()
			case sSpider:
				s.add("spider_parse_response")
				go func() {
					defer func() {
						s.done()
					}()

					response := signal.Body.(*Response)
					responseSlot.AddResponse(response)
					var res RequestItems
					if response.Request.Callback != nil {
						res = response.Request.Callback(response, s)
					} else {
						if s.DefaultParseFunc == nil {
							s.Log.Fatal("Request无默认解析函数")
						} else {
							res = s.DefaultParseFunc(response, s)
						}
					}
					responseSlot.FinishResponse(response)
					s.signalChan <- &Signal{From: sSpider, To: sProcessSpiderOutput, Body: &SpiderOutputData{Response: response, Result: res}}
				}()
			case sProcessSpiderOutput:
				s.add("process_spider_output")
				go func() {
					defer func() {
						s.done()
					}()
					spiderMiddlewareManager.ProcessSpiderOutput(signal, s)
				}()
			case sItemPipeline:
				s.add("process_item")
				go func() {
					defer func() {
						s.done()
					}()

					item := signal.Body.(Item)
					itemSlot.AddItem(item)
					itemPipelineManager.ProcessItem(item, s)
					itemSlot.FinishItem(item)
				}()
			case sProcessSpiderException:
				s.add("process_spider_exception")
				go func() {
					defer func() {
						s.done()
					}()

					spiderMiddlewareManager.ProcessSpiderException(signal, s)
				}()
			case sProcessException:
				s.add("process_exception")
				go func() {
					defer func() {
						s.done()
					}()

					downloaderMiddlewareManager.ProcessException(signal, s)
				}()
			}
		}
	}()

	heartbeat := time.Millisecond * 100

	go downloader.NextRequestCircle(heartbeat)

	for {
		if scheduler.HasPendingRequests() && downloader.IsFree() && responseSlot.IsFree() && itemSlot.IsFree() {
			s.signalChan <- &Signal{From: sSpider, To: sProcessRequest, Body: scheduler.NextRequest()}
			continue
		}

		time.Sleep(heartbeat)

		if !scheduler.HasPendingRequests() && downloader.IsEmpty() && s.isAllDone() {
			select {
			case r := <-s.signalChan:
				s.signalChan <- r
			case <-time.After(time.Second * 5):
				goto close
			}
		}
	}

close:
	scheduler.Close()
	downloader.Close()
	itemPipelineManager.CloseSpider(s)
	if s.CloseFunc != nil {
		s.CloseFunc(s)
	}

	s.Log.Infow("爬取结束", "stats", s.Stats.GetStats())
}
