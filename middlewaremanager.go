package xspider

import (
	"fmt"
	"reflect"
	"sort"
	"xspider/container"
)

func init() {
	RegisterSpiderModuler(&SpiderMiddlewareManagerImpl{})
	RegisterSpiderModuler(&DownloaderMiddlewareManagerImpl{})
	RegisterSpiderModuler(&ItemPipelineManagerImpl{})
	RegisterSpiderModuler(&ExtensionManagerImpl{})
}

var (
	SpiderMiddlewaresBase = map[string]int{
		"HttpErrorSpiderMiddleware": 50,
		//"RefererSpiderMiddleware":       700,
		"UrlLengthSpiderMiddleware":     800,
		"DepthSpiderMiddleware":         850,
		"StartSpiderMiddleware":         900,
		"AllowedDomainSpiderMiddleware": 950,
	}
	DownloaderMiddlewaresBase = map[string]int{
		//"OffsiteDownloaderMiddleware":         50,
		//"RobotsTxtDownloaderMiddleware":       100,
		"HttpAuthDownloaderMiddleware":        300,
		"DownloadTimeoutDownloaderMiddleware": 350,
		"DefaultHeadersDownloaderMiddleware":  400,
		"UserAgentDownloaderMiddleware":       500,
		"RetryDownloaderMiddleware":           550,
		//"AjaxCrawlDownloaderMiddleware":       560,
		//"MetaRefreshDownloaderMiddleware":     580,
		//"HttpCompressionDownloaderMiddleware": 590,
		//"RedirectDownloaderMiddleware":        600,
		//"CookiesDownloaderMiddleware":         700,
		//"HttpProxyDownloaderMiddleware":       750,
		"DownloaderStatsDownloaderMiddleware": 850,
		//"HttpCacheDownloaderMiddleware":       900,
	}
	ExtensionsBase = map[string]int{
		"CoreStatsExtension": 50,
		//"TelnetConsoleExtension":  500,
		//"MemoryUsageExtension":    500,
		//"MemoryDebuggerExtension": 500,
		//"CloseSpiderExtension":    500,
		//"FeedExporterExtension":   500,
		"LogStatsExtension": 500,
		//"SpiderStateExtension":    500,
		//"AutoThrottleExtension":   500,
	}
)

type MiddlewareManager[T any] struct {
	Middlewares []T
}

func NewMiddlewareManager[T any](middlewares map[string]int) (*MiddlewareManager[T], []int, error) {
	mm := &MiddlewareManager[T]{Middlewares: []T{}}
	if middlewares == nil {
		return mm, nil, nil
	}

	keys := make([]string, 0, len(middlewares))
	for k := range middlewares {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return middlewares[keys[i]] < middlewares[keys[j]]
	})

	idxs := []int{}
	for _, name := range keys {
		middleware, ok := GetRegisteredByName(name)
		if !ok {
			return nil, nil, fmt.Errorf("middleware %s not found", name)
		}
		mw, ok := middleware.(T)
		if !ok {
			typ := reflect.TypeOf((*T)(nil)).Elem()
			return nil, nil, fmt.Errorf("middleware %s is not of type %s", name, typ)
		}
		mm.Middlewares = append(mm.Middlewares, mw)
		idxs = append(idxs, middlewares[name])
	}
	return mm, idxs, nil
}

func creatManager[T any](settingName string, baseSetting map[string]int, spider *Spider) (*MiddlewareManager[T], []int) {
	base := spider.Settings.GetWithDefault(fmt.Sprintf("%s_BASE", settingName), baseSetting)

	baseMap := make(map[string]int)
	switch b := base.(type) {
	case map[string]int:
		baseMap = b
	case container.Dict:
		for _, key := range b.Keys() {
			if v, ok := b.GetInt(key); ok {
				baseMap[key] = v
			} else {
				panic(fmt.Errorf("%s_BASE is not a map[string]int or Dict", settingName))
			}
		}
	default:
		panic(fmt.Errorf("%s_BASE is not a map[string]int or Dict", settingName))
	}

	custom := spider.Settings.GetWithDefault(settingName, map[string]int{})
	customMap := make(map[string]int)
	switch c := custom.(type) {
	case map[string]int:
		customMap = c
	case container.Dict:
		for _, key := range c.Keys() {
			if v, ok := c.GetInt(key); ok {
				customMap[key] = v
			} else {
				panic(fmt.Errorf("%s is not a map[string]int or Dict", settingName))
			}
		}
	default:
		panic(fmt.Errorf("%s is not a map[string]int or Dict", settingName))
	}

	for k, v := range customMap {
		baseMap[k] = v
	}
	mm, idxs, err := NewMiddlewareManager[T](baseMap)
	if err != nil {
		panic(err)
	}
	return mm, idxs
}

type SpiderMiddlewareManagerImpl struct {
	BaseSpiderModule
	mm *MiddlewareManager[SpiderMiddlewarer]
}

func (sm *SpiderMiddlewareManagerImpl) Name() string {
	return "SpiderMiddlewareManagerImpl"
}

func (sm *SpiderMiddlewareManagerImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&sm.BaseSpiderModule, spider, sm.Name())
	sm.mm, _ = creatManager[SpiderMiddlewarer]("SPIDER_MIDDLEWARES", SpiderMiddlewaresBase, spider)
	for _, mw := range sm.mm.Middlewares {
		mw.FromSpider(spider)
	}
	sm.Logger.Info("爬虫中间件管理器已初始化")
}

func (sm *SpiderMiddlewareManagerImpl) Middlewares() []SpiderMiddlewarer {
	return sm.mm.Middlewares
}

func (sm *SpiderMiddlewareManagerImpl) Len() int {
	return len(sm.mm.Middlewares)
}

func (sm *SpiderMiddlewareManagerImpl) Close(spider *Spider) {
	for _, mw := range sm.mm.Middlewares {
		mw.Close(spider)
	}
	sm.BaseSpiderModule.Close(spider)
}

func (sm *SpiderMiddlewareManagerImpl) ProcessStartRequests(starts Results, spider *Spider) (results Results, idx int, err error) {
	idx = sm.Len() - 1

	if sm.Len() == 0 {
		return starts, idx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic occurred: %v", r)
			}
			results = nil
		}
	}()

	for i := idx; i >= 0; i-- {
		idx = i
		starts = sm.Middlewares()[i].ProcessStartRequests(starts, spider)
		if starts == nil {
			return nil, idx, nil
		}
	}
	return starts, idx, nil
}

func (sm *SpiderMiddlewareManagerImpl) ProcessSpiderInput(response *Response, spider *Spider) (idx int, err error) {
	idx = 0

	if sm.Len() == 0 {
		return idx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic occurred: %v", r)
			}
		}
	}()

	for i := idx; i < sm.Len(); i++ {
		idx = i
		sm.Middlewares()[i].ProcessSpiderInput(response, spider)
	}
	return idx, nil
}

func (sm *SpiderMiddlewareManagerImpl) ProcessSpiderOutput(response *Response, results Results, idx int, spider *Spider) (res Results, index int, err error) {
	index = idx

	if sm.Len() == 0 {
		return results, index, nil
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic occurred: %v", r)
			}
			res = nil
		}
	}()

	for i := index; i >= 0; i-- {
		index = i
		results = sm.Middlewares()[i].ProcessSpiderOutput(response, results, spider)
		if results == nil {
			return nil, index, nil
		}
	}
	return results, index, nil
}

func (sm *SpiderMiddlewareManagerImpl) ProcessSpiderError(response *Response, err error, spider *Spider) (results Results, idx int, e error) {
	idx = sm.Len() - 1

	if sm.Len() == 0 {
		return nil, idx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			if er, ok := r.(error); ok {
				e = er
			} else {
				e = fmt.Errorf("panic occurred: %v", r)
			}
			results = nil
		}
	}()

	for i := idx; i >= 0; i-- {
		idx = i
		results = sm.Middlewares()[i].ProcessSpiderError(response, err, spider)
		if results != nil {
			return results, idx, nil
		}
	}
	return nil, idx, nil
}

type DownloaderMiddlewareManagerImpl struct {
	BaseSpiderModule
	mm *MiddlewareManager[DownloaderMiddlewarer]
}

func (dm *DownloaderMiddlewareManagerImpl) Name() string {
	return "DownloaderMiddlewareManagerImpl"
}

func (dm *DownloaderMiddlewareManagerImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&dm.BaseSpiderModule, spider, dm.Name())
	dm.mm, _ = creatManager[DownloaderMiddlewarer]("DOWNLOADER_MIDDLEWARES", DownloaderMiddlewaresBase, spider)
	for _, mw := range dm.mm.Middlewares {
		mw.FromSpider(spider)
	}
	dm.Logger.Info("下载器中间件管理器已初始化")
}

func (dm *DownloaderMiddlewareManagerImpl) Middlewares() []DownloaderMiddlewarer {
	return dm.mm.Middlewares
}

func (dm *DownloaderMiddlewareManagerImpl) Len() int {
	return len(dm.mm.Middlewares)
}

func (dm *DownloaderMiddlewareManagerImpl) Close(spider *Spider) {
	for _, mw := range dm.mm.Middlewares {
		mw.Close(spider)
	}
	dm.BaseSpiderModule.Close(spider)
}

func (dm *DownloaderMiddlewareManagerImpl) ProcessRequest(request *Request, spider *Spider) (result Result, idx int, err error) {
	idx = 0

	if dm.Len() == 0 {
		return nil, idx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic occurred: %v", r)
			}
			result = nil
		}
	}()

	for i := idx; i < dm.Len(); i++ {
		idx = i
		result = dm.Middlewares()[i].ProcessRequest(request, spider)
		if result != nil {
			return result, idx, nil
		}
	}
	return nil, idx, nil
}

func (dm *DownloaderMiddlewareManagerImpl) ProcessResponse(request *Request, response *Response, spider *Spider) (result Result, idx int, err error) {
	idx = dm.Len() - 1

	if dm.Len() == 0 {
		return response, idx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic occurred: %v", r)
			}
			result = nil
		}
	}()

	for i := idx; i >= 0; i-- {
		idx = i
		result = dm.Middlewares()[i].ProcessResponse(request, response, spider)
		switch r := result.(type) {
		case *Response:
			response = r
			continue
		default:
			return r, idx, nil
		}
	}
	return response, idx, nil
}

func (dm *DownloaderMiddlewareManagerImpl) ProcessError(request *Request, err error, spider *Spider) (result Result, idx int, e error) {
	idx = dm.Len() - 1

	if dm.Len() == 0 {
		return nil, idx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			if er, ok := r.(error); ok {
				e = er
			} else {
				e = fmt.Errorf("panic occurred: %v", r)
			}
			result = nil
		}
	}()

	for i := idx; i >= 0; i-- {
		idx = i
		result = dm.Middlewares()[i].ProcessError(request, err, spider)
		if result != nil {
			return result, idx, nil
		}
	}
	return nil, idx, nil
}

type ItemPipelineManagerImpl struct {
	BaseSpiderModule
	mm *MiddlewareManager[ItemPipeliner]
}

func (ipm *ItemPipelineManagerImpl) Name() string {
	return "ItemPipelineManagerImpl"
}

func (ipm *ItemPipelineManagerImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&ipm.BaseSpiderModule, spider, ipm.Name())
	ipm.mm, _ = creatManager[ItemPipeliner]("ITEM_PIPELINES", map[string]int{}, spider)
	for _, mw := range ipm.mm.Middlewares {
		mw.FromSpider(spider)
	}
	ipm.Logger.Info("ItemPipeline管理器已初始化")
}

func (ipm *ItemPipelineManagerImpl) ItemPipelines() []ItemPipeliner {
	return ipm.mm.Middlewares
}

func (ipm *ItemPipelineManagerImpl) Len() int {
	return len(ipm.mm.Middlewares)
}

func (ipm *ItemPipelineManagerImpl) Close(spider *Spider) {
	for _, mw := range ipm.mm.Middlewares {
		mw.Close(spider)
	}
	ipm.BaseSpiderModule.Close(spider)
}

func (ipm *ItemPipelineManagerImpl) ProcessItem(item Item, spider *Spider) (it Item, idx int, err error) {
	idx = 0

	if ipm.Len() == 0 {
		return item, idx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic occurred: %v", r)
			}
			it = nil
		}
	}()

	for i := idx; i < ipm.Len(); i++ {
		idx = i
		it = ipm.ItemPipelines()[i].ProcessItem(item, spider)
		if it == nil {
			return nil, idx, nil
		} else {
			item = it
		}
	}
	return item, idx, nil
}

type ExtensionManagerImpl struct {
	BaseSpiderModule
	mm *MiddlewareManager[Extensioner]
}

func (em *ExtensionManagerImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&em.BaseSpiderModule, spider, em.Name())
	mm, idxs := creatManager[Extensioner]("EXTENSIONS", ExtensionsBase, spider)
	em.mm = mm
	for i, mw := range em.mm.Middlewares {
		mw.FromSpider(spider)
		mw.ConnectSignal(spider.Signal, idxs[i])
	}
	em.Logger.Info("扩展管理器已初始化")
}

func (em *ExtensionManagerImpl) Name() string {
	return "ExtensionManagerImpl"
}

func (em *ExtensionManagerImpl) Len() int {
	return len(em.mm.Middlewares)
}

func (em *ExtensionManagerImpl) Extensions() []Extensioner {
	return em.mm.Middlewares
}

func (em *ExtensionManagerImpl) Close(spider *Spider) {
	for _, mw := range em.mm.Middlewares {
		mw.Close(spider)
	}
	em.BaseSpiderModule.Close(spider)
}
