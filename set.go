package xspider

import (
	"errors"
	"sync"
)

type Setter interface {
	Add(elems ...interface{})
	Clear()
	Pop() interface{}
	Remove(elems ...interface{}) error
	Discard(elems ...interface{})
	Has(elems ...interface{}) bool
	Len() int
	//Intersection(setter Setter) Setter
	//Union(setter Setter) Setter
	//Difference(setter Setter) Setter
}

type empty struct{}

type set struct {
	m    map[interface{}]empty
	size int
	lock *sync.RWMutex
}

func NewSet(size int) Setter {
	s := &set{size: size, lock: &sync.RWMutex{}}
	if size > 0 {
		s.m = make(map[interface{}]empty, size)
	} else {
		s.m = make(map[interface{}]empty)
	}
	return s
}

func (s *set) Add(elems ...interface{}) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, v := range elems {
		s.m[v] = empty{}
	}
}

func (s *set) Clear() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.m = make(map[interface{}]empty, s.size)
}

func (s *set) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return len(s.m)
}

func (s *set) Pop() interface{} {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.m) > 0 {
		for k := range s.m {
			delete(s.m, k)
			return k
		}
	}
	return nil
}

func (s *set) Has(elems ...interface{}) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var has bool
	if len(s.m) == 0 {
		return has
	}
	has = true
	for _, v := range elems {
		if _, has = s.m[v]; !has {
			break
		}
	}
	return has
}

func (s *set) Remove(elems ...interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.Has(elems...) {
		return errors.New("部分待移除元素不在set中")
	}
	for _, v := range elems {
		delete(s.m, v)
	}
	return nil
}

func (s *set) Discard(elems ...interface{}) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.m) == 0 {
		return
	}
	for _, v := range elems {
		delete(s.m, v)
	}
}
