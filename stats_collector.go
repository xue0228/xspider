package xspider

import "sync"

type StatsCollector interface {
	GetValue(key string, dft int) int
	GetStats() StatsMap
	SetValue(key string, value int)
	SetStats(stats StatsMap)
	IncValue(key string, count int, start int)
	MaxValue(key string, value int)
	MinValue(key string, value int)
	ClearStats()
}

type StatsMap map[string]int

type StatsCollect struct {
	stats StatsMap
	mu    *sync.RWMutex
}

func NewStatsCollector() StatsCollector {
	return &StatsCollect{
		stats: make(StatsMap),
		mu:    &sync.RWMutex{},
	}
}

func (s *StatsCollect) GetValue(key string, dft int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if v, ok := s.stats[key]; ok {
		return v
	} else {
		return dft
	}
}

func (s *StatsCollect) GetStats() StatsMap {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.stats
}

func (s *StatsCollect) SetValue(key string, value int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats[key] = value
}

func (s *StatsCollect) SetStats(stats StatsMap) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats = stats
}

func (s *StatsCollect) IncValue(key string, count int, start int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.stats[key]; !ok {
		s.stats[key] = start
	}
	s.stats[key] += count
}

func (s *StatsCollect) MaxValue(key string, value int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.stats[key]
	if !ok {
		s.stats[key] = value
		return
	}
	if value > v {
		s.stats[key] = value
	} else {
		s.stats[key] = v
	}
}

func (s *StatsCollect) MinValue(key string, value int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.stats[key]
	if !ok {
		s.stats[key] = value
		return
	}
	if value < v {
		s.stats[key] = value
	} else {
		s.stats[key] = v
	}
}

func (s *StatsCollect) ClearStats() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats = make(StatsMap)
}
