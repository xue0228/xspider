package xspider

import (
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Downloader interface {
	FromSpider(spider *Spider)
	Fetch(request *Request) (*Response, error)
	IsFree() bool
	IsEmpty() bool
	Close()
	NextRequestCircle(heartbeat time.Duration)
	ProcessDownloader(signal *Signal, spider *Spider)
}

type DefaultDownloader struct {
	moduleName string
	logger     *zap.SugaredLogger
	stats      StatsCollector
	ch         chan *Signal

	slots             map[string]*DownloaderSlot
	active            int
	totalConcurrency  int
	domainConcurrency int
	randomizeDelay    bool
	delay             time.Duration
	lock              *sync.RWMutex
}

func (d *DefaultDownloader) FromSpider(spider *Spider) {
	d.moduleName = "downloader"
	d.logger = spider.Log.With("module_name", d.moduleName)
	d.stats = spider.Stats
	d.ch = spider.signalChan

	d.slots = make(map[string]*DownloaderSlot)
	d.active = 0
	d.lock = &sync.RWMutex{}

	d.totalConcurrency = spider.Settings.ConcurrentRequests
	d.domainConcurrency = spider.Settings.ConcurrentRequestsPerDomain
	d.delay = spider.Settings.DownloadDelay
	d.randomizeDelay = spider.Settings.RandomizeDownloadDelay
}

func (d *DefaultDownloader) IsFree() bool {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return d.active < d.totalConcurrency
}

func (d *DefaultDownloader) getSlotKey(request *Request) string {
	// d.lock.RLock()
	// defer d.lock.RUnlock()

	if request.Ctx.Has("download_slot") {
		return request.Ctx.GetString("download_slot")
	} else {
		return request.Domain()
	}
}

func (d *DefaultDownloader) getSlot(request *Request) (key string, slot *DownloaderSlot) {
	d.lock.Lock()
	defer d.lock.Unlock()

	key = d.getSlotKey(request)
	if r, ok := d.slots[key]; !ok {
		slot = &DownloaderSlot{
			concurrency:    d.domainConcurrency,
			delay:          d.delay,
			randomizeDelay: d.randomizeDelay,
			active:         0,
			queue:          NewSet(0),
			lastSeen:       0,
		}
		d.slots[key] = slot
	} else {
		slot = r
	}
	return key, slot
}

func (d *DefaultDownloader) enqueueRequest(request *Request) {
	// d.lock.Lock()
	// defer d.lock.Unlock()

	key, slot := d.getSlot(request)
	request.Ctx.Put("download_slot", key)
	slot.queue.Add(request)
}

func (d *DefaultDownloader) finishRequest(request *Request) {
	d.lock.Lock()
	defer d.lock.Unlock()

	key := d.getSlotKey(request)
	d.slots[key].active -= 1
	d.active -= 1
}

func (d *DefaultDownloader) NextRequestCircle(heartbeat time.Duration) {
	for {
		flag := false

		for _, v := range d.slots {
			r := d.processQueue(v)
			if r != nil {
				d.send(&Signal{From: sDownloader, To: sDownloader, Body: r})
				flag = true
			}
		}

		if !flag {
			time.Sleep(heartbeat)
			d.removeSlot(time.Second * 300)
		}
	}
}

func (d *DefaultDownloader) processQueue(slot *DownloaderSlot) *Request {
	d.lock.Lock()
	defer d.lock.Unlock()

	now := time.Now().UnixNano()
	delay := slot.DownloadDelay()
	if delay > 0 {
		penalty := delay + slot.lastSeen - now
		if penalty > 0 {
			return nil
		}
	}

	if slot.queue.Len() > 0 && slot.IsFree() {
		slot.lastSeen = now
		slot.active += 1
		d.active += 1
		return slot.queue.Pop().(*Request)
	}
	return nil
}

func (d *DefaultDownloader) removeSlot(age time.Duration) {
	d.lock.Lock()
	defer d.lock.Unlock()

	for k, v := range d.slots {
		if v.active == 0 && v.queue.Len() == 0 && ((v.lastSeen + int64(v.delay)) < (time.Now().UnixNano() - int64(age))) {
			delete(d.slots, k)
		}
	}
}

func (d *DefaultDownloader) Fetch(request *Request) (*Response, error) {
	//创建网络请求客户端
	client := &http.Client{
		//超时
		Timeout: request.Ctx.GetWithDefault("download_timeout", time.Second*180).(time.Duration),
		//重定向
		//CheckRedirect: NewCheckRedirect(r),
	}
	//添加代理
	proxy := request.Ctx.GetString("proxy")
	if proxy != "" {
		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		trans := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		client.Transport = trans
	}

	//创建http.Request请求
	req, err := http.NewRequest(request.Method, request.Url.String(), request.Body)
	if err != nil {
		return nil, err
	}
	//向请求头中添加Cookies
	req.Header = *request.Headers
	for _, v := range request.Cookies {
		req.AddCookie(&v)
	}

	//发送网络请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	res, err := NewResponse(resp, request)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (d *DefaultDownloader) send(signal *Signal) {
	d.ch <- signal
}

func (d *DefaultDownloader) Close() {}

func (d *DefaultDownloader) ProcessDownloader(signal *Signal, spider *Spider) {
	request := signal.Body.(*Request)

	if signal.From == sProcessRequest {
		d.enqueueRequest(request)
	} else if signal.From == sDownloader {
		rsp, err := d.Fetch(request)
		if err != nil {
			d.send(&Signal{From: sDownloader, To: sProcessException, Body: &Failure{Request: request, Spider: spider, Error: err}})
		} else {
			d.send(&Signal{From: sDownloader, To: sProcessResponse, Body: rsp})
			NewResponseLogger(d.logger, rsp).Info("Request请求成功")
		}
		d.finishRequest(request)
	}
}

func (d *DefaultDownloader) IsEmpty() bool {
	for _, v := range d.slots {
		if v.Len() != 0 {
			return false
		}
	}
	return true
}

type DownloaderSlot struct {
	concurrency    int
	delay          time.Duration
	randomizeDelay bool
	active         int
	queue          Setter
	lastSeen       int64
}

func (s *DownloaderSlot) DownloadDelay() int64 {
	rand.Seed(time.Now().UnixNano())
	var delay int64
	if s.randomizeDelay {
		delay = int64((rand.Float64() + 0.5) * float64(s.delay))
	} else {
		delay = int64(s.delay)
	}
	return delay
}

func (s *DownloaderSlot) IsFree() bool {
	if s.concurrency <= 0 {
		return true
	}
	if s.concurrency > s.active {
		return true
	}
	return false
}

func (s *DownloaderSlot) Len() int {
	return s.queue.Len()
}
