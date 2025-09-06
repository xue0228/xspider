package xspider

import (
	"github.com/emirpasic/gods/sets/hashset"
)

func init() {
	RegisterSpiderModuler(&SchedulerImpl{})
	RegisterSpiderModuler(&DupeFilterImpl{})
}

type DupeFilterImpl struct {
	BaseSpiderModule
	fingerprints *hashset.Set
}

func (d *DupeFilterImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&d.BaseSpiderModule, spider, d.Name())
	d.fingerprints = hashset.New()
	d.Logger.Info("去重过滤器已初始化")
}

func (d *DupeFilterImpl) Name() string {
	return "DupeFilterImpl"
}

func (d *DupeFilterImpl) RequestSeen(request *Request) bool {
	fp := d.RequestFingerprint(request)
	if d.fingerprints.Contains(fp) {
		return true
	}
	d.fingerprints.Add(fp)
	return false
}

func (d *DupeFilterImpl) RequestFingerprint(request *Request) string {
	return request.Fingerprint(nil, false)
}

func (d *DupeFilterImpl) Log(request *Request) {
	RequestLogger(d.Logger, request).Debug("Request已滤除")
	d.Stats.IncValue("dupe_filter/filtered", 1, 0)
}

type SchedulerImpl struct {
	BaseSpiderModule
	df            DupeFilter
	pq            PriorityQueuer
	filterEnabled bool
}

func (s *SchedulerImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&s.BaseSpiderModule, spider, s.Name())

	s.filterEnabled = spider.Settings.GetBoolWithDefault("DUPE_FILTER_ENABLED", true)

	dfStr := spider.Settings.GetStringWithDefault("DUPE_FILTER_STRUCT", "DupeFilterImpl")
	s.df = GetAndAssertComponent[DupeFilter](dfStr)
	s.df.FromSpider(spider)

	pqStr := spider.Settings.GetStringWithDefault("PRIORITY_QUEUE_STRUCT", "LIFOPriorityQueue")
	s.pq = GetAndAssertComponent[PriorityQueuer](pqStr)
	s.pq.FromSpider(spider)

	s.Logger.Info("调度器已初始化")
}

func (s *SchedulerImpl) Name() string {
	return "SchedulerImpl"
}

func (s *SchedulerImpl) HasPendingRequests() bool {
	return s.pq.Len() > 0
}

func (s *SchedulerImpl) EnqueueRequest(request *Request) bool {
	if s.filterEnabled && !request.DontFilter && s.df.RequestSeen(request) {
		s.df.Log(request)
		return false
	}
	s.pq.Push(request, request.Priority)
	s.Stats.IncValue("scheduler/enqueued", 1, 0)
	RequestLogger(s.Logger, request).Debug("Request入队")

	return true
}

func (s *SchedulerImpl) NextRequest() *Request {
	request := s.pq.Pop()
	if request != nil {
		req := request.(*Request)
		s.Stats.IncValue("scheduler/dequeued", 1, 0)
		RequestLogger(s.Logger, req).Debug("Request出队")
		return req
	} else {
		return nil
	}
}

func (s *SchedulerImpl) Close(spider *Spider) {
	s.df.Close(spider)
	s.pq.Close(spider)
	s.BaseSpiderModule.Close(spider)
}
