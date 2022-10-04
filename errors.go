package xspider

// //框架基础错误类型
// type BaseError struct {
// 	//错误原因（类型），用于记录
// 	Reason string
// 	//实际错误
// 	Err error
// }

// Failure Request请求失败创建的失败信息结构体
type Failure struct {
	Request  *Request
	Response *Response
	Spider   *Spider
	Error    error
}

// var (
// 	ErrIgnoreRequest = errors.New("忽略该请求")
// )

type ErrBase struct {
	Reason string
}

func (err *ErrBase) Error() string {
	return err.Reason
}

type ErrDropItem struct {
	ErrBase
}

func DropItem(reason string) error {
	if reason == "" {
		reason = "none"
	}
	return &ErrDropItem{ErrBase{Reason: reason}}
}

type ErrIgnoreRequest struct {
	ErrBase
}

func IgnoreRequest(reason string) error {
	if reason == "" {
		reason = "none"
	}
	return &ErrIgnoreRequest{ErrBase{Reason: reason}}
}

type ErrHttpError struct {
	ErrBase
}

func HttpError(reason string) error {
	if reason == "" {
		reason = "none"
	}
	return &ErrHttpError{ErrBase{Reason: reason}}
}
