package xspider

type ResponseSlot struct {
	minResponseSize int
	maxActiveSize   int
	activeSize      int
}

func maxInt(x int, y int) int {
	if x < y {
		return y
	} else {
		return x
	}
}

func (s *ResponseSlot) FromSpider(spider *Spider) {
	s.minResponseSize = 1024
	s.maxActiveSize = spider.Settings.ResponseMaxActiveSize
	s.activeSize = 0
}

func (s *ResponseSlot) AddResponse(response *Response) {
	if response == nil {
		return
	}
	s.activeSize += maxInt(len(response.Body), s.minResponseSize)
}

func (s *ResponseSlot) FinishResponse(response *Response) {
	if response == nil {
		return
	}
	s.activeSize -= maxInt(len(response.Body), s.minResponseSize)
}

func (s *ResponseSlot) IsFree() bool {
	return s.activeSize < s.maxActiveSize
}

type ItemSlot struct {
	concurrentItems int
	items           int
}

func (s *ItemSlot) FromSpider(spider *Spider) {
	s.concurrentItems = spider.Settings.ConcurrentItems
	s.items = 0
}

func (s *ItemSlot) AddItem(item Item) {
	if item == nil {
		return
	}
	s.items += 1
}

func (s *ItemSlot) FinishItem(item Item) {
	if item == nil {
		return
	}

	s.items -= 1
}

func (s *ItemSlot) IsFree() bool {
	return s.items <= s.concurrentItems
}
