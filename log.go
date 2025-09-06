package xspider

import (
	"io"
	"os"
	"time"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLog 初始化日志 logger
func NewLog(logPath, errPath string, logLevel zapcore.Level) *zap.SugaredLogger {
	config := zapcore.EncoderConfig{
		MessageKey:   "msg",                       //结构化（json）输出：msg的key
		LevelKey:     "level",                     //结构化（json）输出：日志级别的key（INFO，WARN，ERROR等）
		TimeKey:      "ts",                        //结构化（json）输出：时间的key
		CallerKey:    "file",                      //结构化（json）输出：打印日志的文件对应的Key
		EncodeLevel:  zapcore.CapitalLevelEncoder, //将日志级别转换成大写（INFO，WARN，ERROR等）
		EncodeCaller: zapcore.ShortCallerEncoder,  //采用短文件路径编码输出（test/main.go:14 ）
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02 15:04:05"))
		}, //输出的时间格式
		EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendInt64(int64(d) / 1000000)
		},
	}

	// 自定义Info级别（低于Warn级别的日志）
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.WarnLevel && lvl >= logLevel
	})

	// 自定义Warn级别（Warn级别及以上的日志）
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.WarnLevel && lvl >= logLevel
	})

	var cores []zapcore.Core

	// 始终将日志输出到控制台
	consoleEncoder := zapcore.NewConsoleEncoder(config)
	cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), logLevel))

	// 根据配置添加文件输出
	if logPath != "" {
		infoWriter := getWriter(logPath)
		fileEncoder := zapcore.NewJSONEncoder(config)
		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(infoWriter), infoLevel))
	}

	if errPath != "" {
		warnWriter := getWriter(errPath)
		fileEncoder := zapcore.NewJSONEncoder(config)
		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(warnWriter), warnLevel))
	}

	// 创建核心
	core := zapcore.NewTee(cores...)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.WarnLevel))
	return logger.Sugar()
}

func getWriter(filename string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    2,     //最大M数，超过则切割
		MaxBackups: 0,     //最大文件保留数，超过就删除最老的日志文件
		MaxAge:     0,     //保存最大天数
		Compress:   false, //是否压缩
	}
}

func RequestLogger(log *zap.SugaredLogger, request *Request) *zap.SugaredLogger {
	return log.With(
		"url", request.Url.String(),
		"method", request.Method,
		"priority", request.Priority,
		//"callback", request.Callback,
		//"errback", request.Errback,
		//"dont_filter", request.DontFilter,
	)
}

func ResponseLogger(log *zap.SugaredLogger, response *Response) *zap.SugaredLogger {
	return RequestLogger(log, response.Request).With(
		"status_code", response.StatusCode,
		"content_length", len(response.Body))
}

func SignalLogger(log *zap.SugaredLogger, signal Signaler) *zap.SugaredLogger {
	return log.With(
		"sender", signal.Sender(),
		"type", signal.Type(),
		"len", len(signal.Data()),
	)
}

func FatalSignalError(log *zap.SugaredLogger, signal Signaler, err error) {
	log.Fatalw("Signaler.Data()解析失败",
		"sender", signal.Sender(),
		"type", signal.Type(),
		"error", err)
}

func LogSpiderModulerError(log *zap.SugaredLogger, level zapcore.Level, msg string, module SpiderModuler, err error) {
	log.Logw(level, msg,
		"panic_module", module.Name(),
		"error", err)
}
