// spider_test.go
package xspider

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"xspider/container"
)

func parse(response *Response, spider *Spider) Results {
	//fmt.Println(response.Body)
	fmt.Println(response.Request.Ctx.Map())
	fmt.Println(response.Request.Headers)
	return Generator(func(c chan<- any) {
		r, _ := NewRequest("https://www.baidu.com")
		c <- r
	})
}

func unstop(signal Signaler) {
	panic(fmt.Errorf("wrap: %w", ErrDropSignal))
}

// Call 动态调用函数 fn，传入 args 中的参数
// fn: 任意函数
// args: 参数列表，类型为 []any
// 返回值：[]any，表示函数的所有返回值
func Call(fn any, args []any) []any {
	// 获取函数的反射值
	fnVal := reflect.ValueOf(fn)

	// 检查是否为函数类型
	if fnVal.Kind() != reflect.Func {
		panic("fn is not a function")
	}

	// 获取函数类型
	fnType := fnVal.Type()

	// 检查参数数量
	numIn := fnType.NumIn()
	if len(args) != numIn {
		panic(fmt.Sprintf("function expects %d arguments, but got %d", numIn, len(args)))
	}

	// 将 []any 转换为 []reflect.Value
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		argVal := reflect.ValueOf(arg)
		expectedType := fnType.In(i)

		// 检查类型是否匹配
		if argVal.Type() != expectedType {
			panic(fmt.Sprintf("argument %d: expected %s, got %s", i, expectedType, argVal.Type()))
		}
		in[i] = argVal
	}

	// 调用函数
	results := fnVal.Call(in)

	// 将 []reflect.Value 转换为 []any
	out := make([]any, len(results))
	for i, result := range results {
		out[i] = result.Interface()
	}

	return out
}

func add(a int, b int) int {
	return a + b
}

func TestSpiderInit(t *testing.T) {
	settings := container.NewSyncDict()
	data, err := os.ReadFile("default_settings.json")
	if err != nil {
		panic(err)
	}
	err = settings.Loads(data)
	if err != nil {
		panic(err)
	}
	s := Spider{
		Settings: settings,
	}
	//s.Settings.Set("DOWNLOAD_DELAY", 3)
	//s.Settings.Set("CONCURRENT_REQUESTS_PER_DOMAIN", 10)
	//s.Settings.Set("DUPE_FILTER_ENABLED", false)
	//s.Settings.Set("LOG_STATS_INTERVAL", 2)
	//s.Settings.Set("LOG_LEVEL", "info")
	//s.Settings.Set("ALLOWED_DOMAINS", []string{"example.com"})
	//s.Settings.Set("URL_LENGTH_LIMIT", 5)
	//s.Settings.Set("DEPTH_STATS_VERBOSE", true)
	//s.Settings.Set("DEPTH_LIMIT", 1)
	//s.Settings.Set("HTTP_USER", "xue0228")

	s.Starts = Generator(func(c chan<- any) {
		for _ = range 1 {
			//if i == 5 {
			//	panic("wrong")
			//}
			var cookies []*http.Cookie
			cookies = append(cookies, &http.Cookie{Name: "name", Value: "value"})
			request, _ := NewRequest("https://www.baidu.com", WithBody(strings.NewReader("hello")), WithCookies(cookies))
			c <- request
		}
	})
	err = Register("parse", parse)
	if err != nil {
		panic(err)
	}
	s.DefaultParseFunc = "parse"
	//go func() {
	//	time.Sleep(5 * time.Second)
	//	s.Signal.Connect(unstop, StSpiderIdle, -1)
	//
	//}()

	s.Start()
	//x := fmt.Errorf("\t_ wrapx: %w", ErrDropSignal)
	//t.Log(ExtractPrefix(x))
	//t.Log(ExtractPrefix(ErrDropItem))

	//x := Call(add, []any{1, 2})
	//x := time.Now().Unix()
	//time.Sleep(5 * time.Second)
	//y := time.Now().UTC().Second()
	//y := syscall.ECONNREFUSED
	//y := fmt.Errorf("sdfsdf:%w", ErrDropRequest)
	//y := &url.Error{}
	//y := &net.OpError{Err: &net.DNSError{}}
	//x := ErrorToReason(y)
	//t.Log(reflect.ValueOf(y).Elem().Type().String())
	//t.Log(x)
}
