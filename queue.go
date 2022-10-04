package xspider

import (
	"container/list"
)

type PriorityQueuer interface {
	FromSpider(*Spider)
	Push(item interface{}, priority int)
	Pop() interface{}
	Peek() interface{}
	Size() int
	IsEmpty() bool
	Close()
}

type LifoPriorityQueue struct {
	m map[int]*list.List
}

func (q *LifoPriorityQueue) FromSpider(spider *Spider) {
	q.m = make(map[int]*list.List)
}

func (q *LifoPriorityQueue) Size() int {
	var size int = 0
	for _, v := range q.m {
		size = size + v.Len()
	}
	return size
}

func (q *LifoPriorityQueue) IsEmpty() bool {
	for _, v := range q.m {
		if v.Len() != 0 {
			return false
		}
	}
	return true
}

func (q *LifoPriorityQueue) Push(v interface{}, i int) {
	_, ok := q.m[i]
	if !ok {
		p := list.New()
		q.m[i] = p
	}
	q.m[i].PushBack(v)
}

func max(l *[]int) int {
	re := (*l)[0]
	for _, v := range *l {
		if v > re {
			re = v
		}
	}
	return re
}

func (q *LifoPriorityQueue) Pop() interface{} {
	if q.IsEmpty() {
		return nil
	}
	var l []int
	for i := range q.m {
		l = append(l, i)
	}
	i := max(&l)
	e := q.m[i].Back()
	v := e.Value
	q.m[i].Remove(e)
	if q.m[i].Len() == 0 {
		delete(q.m, i)
	}
	return v
}

func (q *LifoPriorityQueue) Peek() interface{} {
	if q.IsEmpty() {
		return nil
	}
	var l []int
	for i := range q.m {
		l = append(l, i)
	}
	i := max(&l)
	return q.m[i].Back().Value
}

func (q *LifoPriorityQueue) Close() {}
