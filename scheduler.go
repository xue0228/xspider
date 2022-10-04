package xspider

import (
	"errors"
	"sync"

	"go.uber.org/zap"
)

type Scheduler interface {
	FromSpider(spider *Spider)
	HasPendingRequests() bool
	Close()
	EnqueueRequest(request *Request) bool
	NextRequest() *Request
}

type DefaultScheduler struct {
	moduleName    string
	logger        *zap.SugaredLogger
	stats         StatsCollector
	df            DupeFilter
	pq            PriorityQueuer
	filterEnabled bool
	lock          *sync.RWMutex
}

func (s *DefaultScheduler) FromSpider(spider *Spider) {
	s.moduleName = "scheduler"
	s.logger = spider.Log.With("module_name", s.moduleName)
	s.stats = spider.Stats
	s.filterEnabled = spider.Settings.FilterEnabled
	s.lock = &sync.RWMutex{}

	if spider.Settings.FilterClass == nil || spider.Settings.SchedulerPriorityQueue == nil {
		panic(errors.New("设置中未找到Filter或SchedulerPriorityQueue"))
	}
	s.df = CopyNew(spider.Settings.FilterClass).(DupeFilter)
	s.df.FromSpider(spider)

	s.pq = CopyNew(spider.Settings.SchedulerPriorityQueue).(PriorityQueuer)
	s.pq.FromSpider((spider))
}

func (s *DefaultScheduler) HasPendingRequests() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.pq.Size() > 0
}

func (s *DefaultScheduler) EnqueueRequest(request *Request) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.filterEnabled && !request.DontFilter && s.df.RequestSeen(request) {
		s.df.Log(request)
		return false
	}
	s.pq.Push(request, request.Priority)
	s.stats.IncValue("scheduler/enqueued", 1, 0)
	NewRequestLogger(s.logger, request).Debug("Request入队")

	return true
}

func (s *DefaultScheduler) NextRequest() *Request {
	s.lock.Lock()
	defer s.lock.Unlock()

	request := s.pq.Pop()

	if request != nil {
		s.stats.IncValue("scheduler/dequeued", 1, 0)
		NewRequestLogger(s.logger, request.(*Request)).Debug("Request出队")
		return request.(*Request)
	} else {
		return nil
	}
}

func (s *DefaultScheduler) Close() {
	s.df.Close()
	s.pq.Close()
}
