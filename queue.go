package xspider

import "github.com/emirpasic/gods/queues/priorityqueue"

func init() {
	RegisterSpiderModuler(&FIFOPriorityQueue{})
	RegisterSpiderModuler(&LIFOPriorityQueue{})
}

// queueElement 队列元素
type queueElement struct {
	value     any
	priority  int
	timestamp int64 // 用于打破优先级相等时的顺序
}

// BasePriorityQueue 共同字段（嵌入 BaseSpiderModule）
type BasePriorityQueue struct {
	BaseSpiderModule
	queue   *priorityqueue.Queue
	counter int64
}

func (q *BasePriorityQueue) Push(value any, priority int) {
	q.counter++
	elem := &queueElement{
		value:     value,
		priority:  priority,
		timestamp: q.counter,
	}
	q.queue.Enqueue(elem)

	if q.Stats != nil {
		q.Stats.IncValue("queue/push", 1, 0)
	}
}

func (q *BasePriorityQueue) Pop() any {
	if q.queue.Empty() {
		return nil
	}
	value, _ := q.queue.Dequeue()

	if q.Stats != nil {
		q.Stats.IncValue("queue/pop", 1, 0)
	}

	return value.(*queueElement).value
}

func (q *BasePriorityQueue) Peek() any {
	if q.queue.Empty() {
		return nil
	}
	values := q.queue.Values()
	if len(values) == 0 {
		return nil
	}

	if q.Stats != nil {
		q.Stats.IncValue("queue/peek", 1, 0)
	}

	return values[0].(*queueElement).value
}

func (q *BasePriorityQueue) Len() int {
	return q.queue.Size()
}

// FIFOPriorityQueue 优先级相同时 FIFO
type FIFOPriorityQueue struct {
	BasePriorityQueue
}

func (q *FIFOPriorityQueue) Name() string {
	return "FIFOPriorityQueue"
}

func (q *FIFOPriorityQueue) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&q.BasePriorityQueue.BaseSpiderModule, spider, q.Name()) // 调用基类初始化 Logger/Stats

	// 初始化内部队列
	compare := func(a, b interface{}) int {
		ae := a.(*queueElement)
		be := b.(*queueElement)

		// 优先级高者优先
		if ae.priority > be.priority {
			return -1
		} else if ae.priority < be.priority {
			return 1
		}

		// 优先级相同：timestamp 小的在前（FIFO）
		if ae.timestamp < be.timestamp {
			return -1
		} else if ae.timestamp > be.timestamp {
			return 1
		}
		return 0
	}

	q.queue = priorityqueue.NewWith(compare)
	q.counter = 0
	q.Logger.Info("模块初始化完成")
}

// LIFOPriorityQueue 优先级相同时 LIFO
type LIFOPriorityQueue struct {
	BasePriorityQueue
}

func (q *LIFOPriorityQueue) Name() string {
	return "LIFOPriorityQueue"
}

func (q *LIFOPriorityQueue) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&q.BasePriorityQueue.BaseSpiderModule, spider, q.Name())

	compare := func(a, b interface{}) int {
		ae := a.(*queueElement)
		be := b.(*queueElement)

		if ae.priority > be.priority {
			return -1
		} else if ae.priority < be.priority {
			return 1
		}

		// 优先级相同：timestamp 大的在前（LIFO）
		if ae.timestamp > be.timestamp {
			return -1
		} else if ae.timestamp < be.timestamp {
			return 1
		}
		return 0
	}

	q.queue = priorityqueue.NewWith(compare)
	q.counter = 0
	q.Logger.Info("模块初始化完成")
}
