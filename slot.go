package xspider

import (
	"math/rand/v2"
	"sync"
	"time"
	"xspider/container"

	llq "github.com/emirpasic/gods/queues/linkedlistqueue"
)

func init() {
	RegisterSpiderModuler(&RequestSlotImpl{})
	RegisterSpiderModuler(&ItemSlotImpl{})
	RegisterSpiderModuler(&ResponseSlotImpl{})
}

type ResponseSlotImpl struct {
	BaseSpiderModule
	minResponseSize int
	maxActiveSize   int
	activeSize      int
	mu              sync.RWMutex
}

func (rs *ResponseSlotImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&rs.BaseSpiderModule, spider, rs.Name())
	rs.mu = sync.RWMutex{}
	rs.minResponseSize = 1024
	rs.maxActiveSize = spider.Settings.GetIntWithDefault("DOWNLOAD_MAXSIZE", 1073741824)
	rs.activeSize = 0
	rs.Logger.Info("响应并发控制模块已初始化")
}

func (rs *ResponseSlotImpl) Name() string {
	return "ResponseSlotImpl"
}

func (rs *ResponseSlotImpl) Add(response *Response) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if response == nil {
		return
	}
	rs.activeSize += Max(len(response.Body), rs.minResponseSize)
}

func (rs *ResponseSlotImpl) Done(response *Response) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if response == nil {
		return
	}
	rs.activeSize -= Max(len(response.Body), rs.minResponseSize)
}

func (rs *ResponseSlotImpl) IsFree() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return rs.activeSize < rs.maxActiveSize
}

type ItemSlotImpl struct {
	BaseSpiderModule
	concurrentItems int
	active          int
	items           *llq.Queue
	mu              sync.RWMutex
}

func (is *ItemSlotImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&is.BaseSpiderModule, spider, is.Name())
	is.items = llq.New()
	is.mu = sync.RWMutex{}
	is.concurrentItems = spider.Settings.GetIntWithDefault("CONCURRENT_ITEMS", 100)
	is.Logger.Info("Item并发控制模块已初始化")
}

func (is *ItemSlotImpl) Name() string {
	return "ItemSlotImpl"
}

func (is *ItemSlotImpl) Push(signal *ItemResponseSignal) {
	is.mu.Lock()
	defer is.mu.Unlock()
	if signal == nil {
		return
	}
	is.items.Enqueue(signal)
}

func (is *ItemSlotImpl) Pop() *ItemResponseSignal {
	is.mu.Lock()
	defer is.mu.Unlock()
	signal, ok := is.items.Dequeue()
	if !ok {
		return nil
	}
	is.active++
	return signal.(*ItemResponseSignal)
}

func (is *ItemSlotImpl) Finish(signal *ItemResponseSignal) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.active--
}

func (is *ItemSlotImpl) IsFree() bool {
	is.mu.RLock()
	defer is.mu.RUnlock()
	if is.concurrentItems <= 0 {
		return true
	}
	return is.active < is.concurrentItems
}

func (is *ItemSlotImpl) IsEmpty() bool {
	is.mu.RLock()
	defer is.mu.RUnlock()
	return is.items.Empty() && is.active <= 0
}

type requestSlot struct {
	concurrency    int
	maxQueueSize   int
	delay          time.Duration
	randomizeDelay bool
	requests       *llq.Queue
	lastSeen       int64
	lastDelay      int64
	active         int
	mu             sync.RWMutex
}

func newRequestSlot(concurrency int, delay time.Duration, randomizeDelay bool, maxQueueSize int) *requestSlot {
	return &requestSlot{
		concurrency:    concurrency,
		delay:          delay,
		randomizeDelay: randomizeDelay,
		maxQueueSize:   maxQueueSize,
		requests:       llq.New(),
		lastSeen:       0,
		active:         0,
	}
}

func (ds *requestSlot) push(request *Request) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.requests.Enqueue(request)
}

func (ds *requestSlot) pop() *Request {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	res, ok := ds.requests.Dequeue()
	if !ok {
		return nil
	}
	ds.active++
	ds.lastSeen = time.Now().UnixNano()
	ds.lastDelay = 0
	return res.(*Request)
}

func (ds *requestSlot) finish() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.active--
}

func (ds *requestSlot) downloadDelay() int64 {
	var delay int64
	if ds.randomizeDelay {
		delay = int64((rand.Float64() + 0.5) * float64(ds.delay))
	} else {
		delay = int64(ds.delay)
	}
	return delay
}

func (ds *requestSlot) isFree() bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if ds.concurrency <= 0 {
		return true
	}
	return ds.active < ds.concurrency
}

func (ds *requestSlot) isEmpty() bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.requests.Empty() && ds.active <= 0
}

func (ds *requestSlot) isQueueFull() bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.requests.Size() >= ds.maxQueueSize
}

func (ds *requestSlot) queueLen() int {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.requests.Size()
}

func (ds *requestSlot) activeLen() int {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.active
}

type requestSlotConfig struct {
	concurrency    int
	delay          time.Duration
	randomizeDelay bool
}

type RequestSlotImpl struct {
	BaseSpiderModule
	slots                      map[string]*requestSlot
	concurrentRequests         int
	maxQueueSize               int
	concurrentRequestPerDomain int
	downloadDelay              int
	randomizeDelay             bool
	requestSlots               map[string]requestSlotConfig
	mu                         sync.RWMutex
}

func (rs *RequestSlotImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&rs.BaseSpiderModule, spider, rs.Name())
	rs.slots = make(map[string]*requestSlot)

	rs.concurrentRequests = spider.Settings.GetIntWithDefault("CONCURRENT_REQUESTS", 16)
	rs.maxQueueSize = spider.Settings.GetIntWithDefault("MAX_REQUEST_QUEUE_SIZE_PER_DOMAIN", rs.concurrentRequests)
	if rs.maxQueueSize <= 0 {
		rs.maxQueueSize = 16
	}
	rs.concurrentRequestPerDomain = spider.Settings.GetIntWithDefault("CONCURRENT_REQUESTS_PER_DOMAIN", 1)
	rs.downloadDelay = spider.Settings.GetIntWithDefault("DOWNLOAD_DELAY", 1)
	rs.randomizeDelay = spider.Settings.GetBoolWithDefault("RANDOMIZE_DOWNLOAD_DELAY", true)
	rs.requestSlots = make(map[string]requestSlotConfig)
	requestSlotsAny := spider.Settings.GetWithDefault("REQUEST_SLOTS", map[string]map[string]any{})
	requestSlots := make(map[string]map[string]any)
	switch requestSlotsT := requestSlotsAny.(type) {
	case container.Dict:
		requestSlotsT2 := requestSlotsT.Map()
		for k, v := range requestSlotsT2 {
			switch v2 := v.(type) {
			case container.Dict:
				requestSlots[k] = v2.Map()
			case map[string]any:
				requestSlots[k] = v2
			default:
				panic("REQUEST_SLOTS参数类型错误")
			}
		}
	case map[string]map[string]any:
		requestSlots = requestSlotsT
	default:
		panic("REQUEST_SLOTS参数类型错误")
	}
	for domain, config := range requestSlots {
		var (
			concurrency    = rs.concurrentRequestPerDomain // 默认值
			delay          = time.Duration(rs.downloadDelay) * time.Second
			randomizeDelay = rs.randomizeDelay
		)

		// 解析 concurrency
		if val, ok := config["concurrency"]; ok {
			if c, ok := val.(int); ok {
				concurrency = c
			}
		}

		// 解析 delay（单位：秒）
		if val, ok := config["delay"]; ok {
			switch v := val.(type) {
			case int:
				delay = time.Duration(v) * time.Second
			case float64:
				delay = time.Duration(v) * time.Second
			}
		}

		// 解析 randomizeDelay
		if val, ok := config["randomize_delay"]; ok {
			if b, ok := val.(bool); ok {
				randomizeDelay = b
			}
		}

		rs.requestSlots[domain] = requestSlotConfig{
			concurrency:    concurrency,
			delay:          delay,
			randomizeDelay: randomizeDelay,
		}
	}
	rs.Logger.Info("请求并发控制模块已初始化")
}

func (rs *RequestSlotImpl) Name() string {
	return "RequestSlotImpl"
}

func (rs *RequestSlotImpl) Push(request *Request) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	domain := request.Domain()
	if slot, ok := rs.slots[domain]; ok {
		slot.push(request)
	} else {
		var s *requestSlot
		if config, ok := rs.requestSlots[domain]; ok {
			maxQueueSize := config.concurrency
			if maxQueueSize <= 0 {
				maxQueueSize = rs.maxQueueSize
			}
			s = newRequestSlot(config.concurrency, config.delay, config.randomizeDelay, maxQueueSize)
		} else {
			s = newRequestSlot(
				rs.concurrentRequests,
				time.Duration(rs.downloadDelay)*time.Second,
				rs.randomizeDelay,
				rs.maxQueueSize)
		}
		s.push(request)
		rs.slots[domain] = s
	}
}

func (rs *RequestSlotImpl) Finish(request *Request) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	domain := request.Domain()
	rs.slots[domain].finish()
}

func (rs *RequestSlotImpl) Pop() Requests {
	res := make(chan *Request)
	go func() {
		defer close(res)
		for _, slot := range rs.slots {
			request := rs.processQueue(slot)
			if request != nil {
				res <- request
			}
		}
	}()
	return res
}

func (rs *RequestSlotImpl) IsFree() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	res := false
	active := 0
	for _, slot := range rs.slots {
		if rs.concurrentRequests > 0 {
			active += slot.activeLen()
			if active >= rs.concurrentRequests {
				res = false
				break
			}
		}
		if !slot.isQueueFull() && slot.isFree() && !slot.isEmpty() {
			res = true
			break
		}
	}
	return res
}

func (rs *RequestSlotImpl) IsEmpty() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	for _, slot := range rs.slots {
		if !slot.isEmpty() {
			return false
		}
	}
	return true
}

func (rs *RequestSlotImpl) Clear(age time.Duration) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for domain, slot := range rs.slots {
		if slot.activeLen() <= 0 &&
			slot.queueLen() <= 0 &&
			slot.lastSeen+int64(slot.delay) < time.Now().UnixNano()-int64(age) {
			delete(rs.slots, domain)
		}
	}
}

func (rs *RequestSlotImpl) processQueue(slot *requestSlot) *Request {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now().UnixNano()
	if slot.lastDelay == 0 {
		slot.lastDelay = slot.downloadDelay()
		//fmt.Println(slot.lastDelay)
	}
	if slot.delay > 0 {
		penalty := slot.lastDelay + slot.lastSeen - now
		if penalty > 0 {
			return nil
		}
	}
	if slot.isFree() {
		return slot.pop()
	}
	return nil
}
