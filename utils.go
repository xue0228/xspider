package xspider

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"

	"github.com/kennygrant/sanitize"
	"github.com/xue0228/xspider/encoder"
	"go.uber.org/zap/zapcore"
)

// GetFunctionName 获取函数名称
func GetFunctionName(i any) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func Generator(yield func(chan<- any)) <-chan any {
	ch := make(chan any)
	go func() {
		defer close(ch)
		yield(ch)
	}()
	return ch
}

// Max 返回两个类型为 T 的值中的较大者。
// 约束 `cmp.Ordered` 确保类型 T 支持 > 操作符。
func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// StringToLevel 将字符串转换为 zapcore.Level
func StringToLevel(levelStr string) (zapcore.Level, error) {
	var level zapcore.Level
	err := level.UnmarshalText([]byte(levelStr))
	if err != nil {
		return level, err
	}
	return level, nil
}

// GetAndAssertComponent 从注册表中获取组件并进行类型断言
func GetAndAssertComponent[T any](componentName string) T {
	component, ok := GetRegisteredByName(componentName)
	if !ok {
		typeName := reflect.TypeOf((*T)(nil)).Elem().Name()
		if typeName == "" {
			typeName = reflect.TypeOf((*T)(nil)).Elem().String()
		}
		panic(fmt.Sprintf("%s %s not found", typeName, componentName))
	}

	result, ok := component.(T)
	if !ok {
		typeName := reflect.TypeOf((*T)(nil)).Elem().Name()
		if typeName == "" {
			typeName = reflect.TypeOf((*T)(nil)).Elem().String()
		}
		panic(fmt.Sprintf("%s %s is not of the expected type", typeName, componentName))
	}

	return result
}

func ReadRequestBody(request *Request) []byte {
	var p []byte
	if request.Body != nil {
		if b, err := io.ReadAll(request.Body); err == nil {
			p = b
		}
		request.Body = bytes.NewBuffer(p)
	}
	return p
}

func GetResponseSize(response *Response) int {
	return len(response.Body) + GetHeaderSize(*response.Headers) + GetStatusSize(response.StatusCode) + 4
}

// CookiesToString 将 []*http.Cookie 转换为分号分隔的字符串
func CookiesToString(cookies []*http.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}

	cookieStrings := make([]string, len(cookies))
	for i, cookie := range cookies {
		cookieStrings[i] = cookie.String()
	}
	return strings.Join(cookieStrings, ";")
}

// SanitizeFileName replaces dangerous characters in a string
// so the return value can be used as a safe file name.
func SanitizeFileName(fileName string) string {
	ext := filepath.Ext(fileName)
	cleanExt := sanitize.BaseName(ext)
	if cleanExt == "" {
		cleanExt = ".unknown"
	}
	return strings.Replace(fmt.Sprintf(
		"%s.%s",
		sanitize.BaseName(fileName[:len(fileName)-len(ext)]),
		cleanExt[1:],
	), "-", "_", -1)
}

// ExtractPrefix 提取错误信息中冒号前的字符串，并处理空格和下划线
// 1. 提取冒号前的内容
// 2. 去除最前和最后的空格、下划线
// 3. 将中间的空格替换为下划线
func ExtractPrefix(err error) string {
	if err == nil {
		return ""
	}

	// 获取错误字符串并按冒号分割
	parts := strings.Split(err.Error(), ":")
	if len(parts) == 0 {
		return ""
	}

	// 提取冒号前的内容
	prefix := strings.TrimSpace(parts[0])

	// 去除最前和最后的空格、下划线
	prefix = strings.TrimFunc(prefix, func(r rune) bool {
		return r == ' ' || r == '_'
	})

	// 将中间的空格替换为下划线
	// 先将多个连续空格替换为单个空格
	prefix = strings.Join(strings.Fields(prefix), " ")
	// 再将空格替换为下划线
	prefix = strings.ReplaceAll(prefix, " ", "_")

	return prefix
}

func BasicAuthHeader(username, password, encoding string) string {
	if encoding == "" {
		encoding = "iso-8859-1"
	}
	auth := fmt.Sprintf("%s:%s", username, password)
	authBytes, err := encoder.ToBytes(auth, encoding)
	if err != nil {
		panic(err)
	}
	return "Basic " + encoder.StandardB64Encode(authBytes)
}

// ErrorToReason 将错误转换为对应的原因标识字符串
// 对于nil错误返回空字符串，其他情况返回相应的错误原因
func ErrorToReason(err error) string {
	if err == nil {
		return ""
	}

	// 特殊情况：优先处理超时错误
	if isTimeoutError(err) {
		return "Timeout"
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch {
		case errors.Is(err, syscall.ECONNREFUSED):
			return "syscall.ECONNREFUSED"
		case errors.Is(err, syscall.ETIMEDOUT):
			return "syscall.ETIMEDOUT"
		default:
			return "syscall.Errno"
		}
	}

	// 条件解包错误
	unwrappedErr := conditionalUnwrap(err)
	if isWrappedError(unwrappedErr) {
		return ExtractPrefix(err)
	} else {
		return getErrorTypeName(unwrappedErr)
	}
}

// conditionalUnwrap 解包错误，当遇到不是fmt.Errorf或errors.New创建的错误时停止
func conditionalUnwrap(err error) error {
	current := err
	for {
		next := errors.Unwrap(current)
		if next == nil {
			break
		}

		// 检查下一层错误是否是由fmt.Errorf或errors.New创建的包装错误
		//if isWrappedError(next) {
		//	current = next
		//} else {
		//	// 遇到非包装错误，停止解包
		//	break
		//}
		current = next
	}
	return current
}

// 判断错误是否是由fmt.Errorf或errors.New创建的包装错误
func isWrappedError(err error) bool {
	if err == nil {
		return false
	}

	val := reflect.ValueOf(err)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 获取类型名称
	typeName := val.Type().String()

	// fmt.Errorf和errors.New创建的错误类型名为errorString
	return typeName == "errors.errorString" || typeName == "fmt.wrapError"
}

// 获取错误类型名称并转换为原因标识
func getErrorTypeName(err error) string {
	if err == nil {
		return "nil_error"
	}

	val := reflect.ValueOf(err)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	//typeName := val.Type().Name()
	//if typeName == "" {
	//	// 对于匿名类型，使用包路径和类型信息
	//	typeName = val.Type().String()
	//}
	typeName := val.Type().String()
	return typeName
}

// 判断是否为超时错误
func isTimeoutError(err error) bool {
	type timeout interface {
		Timeout() bool
	}

	var te timeout
	return errors.As(err, &te) && te.Timeout()
}

func GetHeaderSize(headers http.Header) int {
	size := 0
	headerCount := 0

	for key, values := range headers {
		// 确保 key 是有效的 UTF-8（或按字节处理）
		keyBytes := len(key)
		for _, v := range values {
			// 每个 header 字段格式: "key: value\r\n"
			// 包括 ": " (2 字节), 以及末尾的 "\r\n" 在循环外统一处理
			size += keyBytes + 2 + len(v)
			headerCount++
		}
	}

	// 每个 header 项后跟一个 \r\n，如果有 n 个 header 项，则有 n 个 \r\n
	// 但原 Python 代码中是按 headers.keys() 数量减一来加 \r\n，可能只在 headers 间加，不包含最后的 \r\n
	// 原 Python: len(b"\r\n") * (len(headers.keys()) - 1)
	// 注意：Python 中是按 key 的数量减一，不是 value 项的数量！
	// 所以我们这里模仿 Python 行为：只在 key 之间插入 \r\n，不包括最后一个 \r\n

	// 获取 key 的数量
	keyCount := len(headers)
	if keyCount > 0 {
		size += 2 * (keyCount - 1) // "\r\n" 是 2 字节，插入 keyCount - 1 次
	}

	return size
}

func GetStatusSize(statusCode int) int {
	// 获取状态文本，例如 200 -> "OK", 404 -> "Not Found"
	statusText := http.StatusText(statusCode)

	// 构造状态行: "HTTP/1.1 XXX <reason>\r\n"
	// 固定版本 + 状态码（3位）+ 状态文本
	// 示例: "HTTP/1.1 200 OK\r\n" -> len("HTTP/1.1 ") + 3 + len(" OK") + len("\r\n")
	// 实际上是: "HTTP/1.1 %d %s\r\n"
	// 注意：状态码是数字，我们按3位算（100~599），但 Go 中用 %d 自动处理

	// 我们可以精确计算：
	statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, statusText)
	return len(statusLine)

	// 原 Python 中是：
	// len(to_bytes(http.RESPONSES.get(response_status, b""))) + 15
	// +15 大概是 "HTTP/1.1 xxx " 的长度（包括空格），我们直接构造更准确。
}
