package xspider

type Signal struct {
	From  int
	To    int
	Index int
	Body  interface{}
}

const (
	sProcessRequest = iota
	sProcessResponse
	sProcessException
	sProcessSpiderInput
	sProcessSpiderOutput
	sProcessSpiderException
	sProcessStartRequests
	sScheduler
	sDownloader
	sItemPipeline
	sSpider
)
