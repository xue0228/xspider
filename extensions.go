package xspider

import (
	"fmt"
	"sync"
	"time"

	"github.com/xue0228/xspider/container"
)

func init() {
	RegisterSpiderModuler(&CoreStatsExtension{})
	RegisterSpiderModuler(&LogStatsExtension{})
}

type CoreStatsExtension struct {
	BaseSpiderModule
	startTime int64
}

func (cs *CoreStatsExtension) ConnectSignal(sm SignalManager, idx int) {
	sm.Connect(cs.spiderOpened, StSpiderOpened, idx)
	sm.Connect(cs.spiderClosed, StSpiderClosed, idx)
	sm.Connect(cs.itemScraped, StItemScraped, idx)
	sm.Connect(cs.itemDropped, StItemDropped, idx)
	sm.Connect(cs.responseLeftDownloader, StResponseLeftDownloader, idx)
}

func (cs *CoreStatsExtension) Name() string {
	return "CoreStatsExtension"
}

func (cs *CoreStatsExtension) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&cs.BaseSpiderModule, spider, cs.Name())
}

func (cs *CoreStatsExtension) spiderOpened(spider *Spider) {
	cs.startTime = time.Now().UTC().UnixNano()
	cs.Stats.SetValue("start_time", int(cs.startTime))
}

func (cs *CoreStatsExtension) spiderClosed(reason string, spider *Spider) {
	if cs.startTime == 0 {
		panic("start time is zero")
	}
	finishTime := time.Now().UTC().UnixNano()
	elapsedTimeSeconds := float64(finishTime-cs.startTime) / 1000000000
	cs.Stats.SetValue("elapsed_time_seconds", elapsedTimeSeconds)
	cs.Stats.SetValue("finish_reason", reason)
	cs.Stats.SetValue("finish_time", int(finishTime))
}

func (cs *CoreStatsExtension) itemScraped(item any, response *Response, spider *Spider) {
	cs.Stats.IncValue("item_scraped_count", 1, 0)
}

func (cs *CoreStatsExtension) itemDropped(item any, response *Response, err error, spider *Spider) {
	cs.Stats.IncValue("item_dropped_count", 1, 0)
	cs.Stats.IncValue(fmt.Sprintf("item_dropped_count/%s", ErrorToReason(err)), 1, 0)
}

func (cs *CoreStatsExtension) responseLeftDownloader(request *Request, response *Response, spider *Spider) {
	cs.Stats.IncValue("response_received_count", 1, 0)
}

type LogStatsExtension struct {
	BaseSpiderModule
	interval   float64
	multiplier float64
	pages      int
	items      int
	pagesPrev  int
	itemsPrev  int
	pRate      float64
	iRate      float64
	quitChan   chan struct{}
	wg         sync.WaitGroup
}

func (ls *LogStatsExtension) Name() string {
	return "LogStatsExtension"
}

func (ls *LogStatsExtension) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&ls.BaseSpiderModule, spider, ls.Name())
	//ls.interval = spider.Settings.GetFloat64WithDefault("LOG_STATS_INTERVAL", 60.0)
	ls.interval = container.GetWithDefault[float64](spider.Settings, "LOG_STATS_INTERVAL", 60.0)
	ls.multiplier = 60.0 / ls.interval
	ls.quitChan = make(chan struct{})
}

func (ls *LogStatsExtension) ConnectSignal(sm SignalManager, idx int) {
	sm.Connect(ls.spiderOpened, StSpiderOpened, idx)
	sm.Connect(ls.spiderClosed, StSpiderClosed, idx)
}

func (ls *LogStatsExtension) calculateStats() {
	ls.items = ls.Stats.GetIntValue("item_scraped_count", 0)
	ls.pages = ls.Stats.GetIntValue("response_received_count", 0)
	ls.iRate = float64(ls.items-ls.itemsPrev) * ls.multiplier
	ls.pRate = float64(ls.pages-ls.pagesPrev) * ls.multiplier
	ls.pagesPrev, ls.itemsPrev = ls.pages, ls.items
}

func (ls *LogStatsExtension) calculateFinalStats() (float64, float64) {
	startTime := ls.Stats.GetIntValue("start_time", 0)
	finishTime := ls.Stats.GetIntValue("finish_time", 0)
	if finishTime == 0 {
		panic("finish_time is not set")
	}
	minsElapsed := float64(finishTime-startTime) / 1000000000 / 60
	if minsElapsed == 0 {
		return 0, 0
	}
	items := ls.Stats.GetFloat64Value("item_scraped_count", 0)
	pages := ls.Stats.GetFloat64Value("response_received_count", 0)
	return pages / minsElapsed, items / minsElapsed
}

func (ls *LogStatsExtension) log() {
	msg := fmt.Sprintf("Crawled %d pages (at %.2f pages/min), "+
		"scraped %d items (at %.2f items/min)", ls.pages, ls.pRate, ls.items, ls.iRate)
	ls.Logger.Info(msg)
}

func (ls *LogStatsExtension) task() {
	ls.wg.Add(1)
	defer ls.wg.Done()

	for {
		select {
		case <-time.After(time.Duration(ls.interval) * time.Second):
			ls.calculateStats()
			ls.log()
		case <-ls.quitChan:
			return
		}
	}
}

func (ls *LogStatsExtension) Close(spider *Spider) {
	close(ls.quitChan)
	ls.wg.Wait()
	ls.BaseSpiderModule.Close(spider)
}

func (ls *LogStatsExtension) spiderOpened(spider *Spider) {
	ls.pagesPrev = 0
	ls.itemsPrev = 0
	go ls.task()
}

func (ls *LogStatsExtension) spiderClosed(reason string, spider *Spider) {
	rpmFinal, ipmFinal := ls.calculateFinalStats()
	ls.Stats.SetValue("responses_per_minute", rpmFinal)
	ls.Stats.SetValue("items_per_minute", ipmFinal)
}
