package xspider

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"sync"

	"github.com/xue0228/xspider/container"
)

func init() {
	RegisterSpiderModuler(&SignalManagerImpl{})
}

type SignalManagerImpl struct {
	BaseSpiderModule
	quit         chan struct{}
	signalChan   chan Signaler
	receivers    map[SignalType][]ReceiverConfig
	running      bool
	verboseStats bool
	mu           sync.RWMutex
	wg           sync.WaitGroup
}

func (sm *SignalManagerImpl) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&sm.BaseSpiderModule, spider, sm.Name())
	//sm.verboseStats = spider.Settings.GetBoolWithDefault("SIGNAL_VERBOSE_STATS", false)
	sm.verboseStats = container.GetWithDefault[bool](spider.Settings, "SIGNAL_VERBOSE_STATS", false)
	sm.quit = make(chan struct{})
	sm.signalChan = make(chan Signaler)
	sm.receivers = make(map[SignalType][]ReceiverConfig)
	sm.running = false
	sm.mu = sync.RWMutex{}
	sm.wg = sync.WaitGroup{}

	sm.Logger.Info("模块初始化完成")
}

func (sm *SignalManagerImpl) add(signal Signaler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.verboseStats && signal != nil {
		sm.Stats.IncValue(fmt.Sprintf("signal_manager/%s/add", signal.Type()), 1, 0)
	}
	sm.Stats.IncValue("signal_manager/total/add", 1, 0)
}

func (sm *SignalManagerImpl) done(signal Signaler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.verboseStats && signal != nil {
		sm.Stats.IncValue(fmt.Sprintf("signal_manager/%s/done", signal.Type()), 1, 0)
	}
	sm.Stats.IncValue("signal_manager/total/done", 1, 0)
}

func (sm *SignalManagerImpl) IsAllDone() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.Stats.GetValue("signal_manager/total/done", 0) == sm.Stats.GetValue("signal_manager/total/add", 0)
}

func (sm *SignalManagerImpl) Connect(receiver SignalReceiver, signal SignalType, idx int, senderFilter ...Sender) {
	if receiver == nil || signal == "" {
		return
	}

	// 获取函数的反射值
	fnVal := reflect.ValueOf(receiver)

	// 检查是否为函数类型
	if fnVal.Kind() != reflect.Func {
		panic("fn is not a function")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 初始化 receivers map（如果尚未初始化）
	if sm.receivers == nil {
		sm.receivers = make(map[SignalType][]ReceiverConfig)
	}

	config := ReceiverConfig{
		Receiver:     fnVal,
		Index:        idx,
		SenderFilter: senderFilter,
	}

	sm.receivers[signal] = append(sm.receivers[signal], config)
}

func (sm *SignalManagerImpl) Disconnect(receiver SignalReceiver, signal SignalType) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	receivers := sm.receivers[signal]
	if receivers == nil {
		return false
	}
	for i, config := range receivers {
		if GetFunctionName(config.Receiver) == GetFunctionName(receiver) {
			sm.receivers[signal] = append(receivers[:i], receivers[i+1:]...)
			if len(sm.receivers[signal]) == 0 {
				delete(sm.receivers, signal)
			}
			return true
		}
	}
	return false
}

func (sm *SignalManagerImpl) DisconnectAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.receivers = make(map[SignalType][]ReceiverConfig)
}

// Emit 发送一个信号
// 如果 signalChan 缓冲区已满，此操作会阻塞直到有空间
// 如果 manager 已停止，信号将被丢弃
func (sm *SignalManagerImpl) Emit(signal Signaler) {
	sm.mu.RLock()
	running := sm.running
	sm.mu.RUnlock()

	if !running || signal == nil {
		return
	}

	sm.signalChan <- signal

	//go func() {
	//	sm.wg.Add(1)
	//	defer sm.wg.Done()
	//	select {
	//	case sm.signalChan <- signal:
	//		// 信号成功发送到通道
	//	case <-sm.quit:
	//		// manager 在发送前停止了，丢弃信号
	//		return
	//	}
	//}()
}

func (sm *SignalManagerImpl) callReceiver(fn reflect.Value, signal Signaler) {
	// 获取函数类型
	fnType := fn.Type()
	args := make([]any, len(signal.Data()), len(signal.Data())+1)
	copy(args, signal.Data())

	// 回调函数参数缺少一个时自动添加 sender 参数
	numIn := fnType.NumIn()
	if len(args)+1 == numIn {
		args = append(args, signal.Sender())
	}
	// 检查参数数量
	if len(args) != numIn {
		panic(fmt.Sprintf("function expects %d arguments, but got %d", numIn, len(args)))
	}

	// 将 []any 转换为 []reflect.Value
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		argVal := reflect.ValueOf(arg)
		expectedType := fnType.In(i)

		// 检查类型是否匹配
		if !argVal.Type().AssignableTo(expectedType) {
			panic(fmt.Sprintf("argument %d: expected %s, got %s", i, expectedType, argVal.Type()))
		}
		in[i] = argVal
	}

	// 调用函数
	fn.Call(in)
}

// dispatch 将接收到的信号分发给所有注册的监听器
// 分发是异步的，每个监听器在独立的 goroutine 中执行
// 同 index 的 receiver 并发执行，不同 index 按序号由小到大依次执行（支持负数 index）
func (sm *SignalManagerImpl) dispatch(signal Signaler) {
	logger := SignalLogger(sm.Logger, signal)
	logger.Debug("开始处理信号")
	defer logger.Debug("结束处理信号")

	if signal == nil {
		return
	}

	signalType := signal.Type()

	sm.mu.RLock()
	listeners, exists := sm.receivers[signalType]
	sm.mu.RUnlock()

	if !exists || len(listeners) == 0 {
		return
	}

	// 按 Index 分组 receivers
	indexGroups := make(map[int][]ReceiverConfig)
	for _, rc := range listeners {
		indexGroups[rc.Index] = append(indexGroups[rc.Index], rc)
	}

	// 提取所有唯一的 Index 并排序
	var indices []int
	for idx := range indexGroups {
		indices = append(indices, idx)
	}
	sort.Ints(indices) // 从小到大排序，支持负数

	// 按排序后的 Index 依次执行每组
	drop := false
	for _, idx := range indices {
		if drop {
			return
		}

		group := indexGroups[idx]

		// 使用 WaitGroup 等待当前 index 组内所有 receiver 执行完毕
		var groupWg sync.WaitGroup
		for _, config := range group {
			// SenderFilter 过滤：如果非空，则必须包含当前 sender
			sender := signal.Sender()
			if len(config.SenderFilter) > 0 {
				matched := slices.Contains(config.SenderFilter, sender)
				if !matched {
					continue
				}
			}

			groupWg.Add(1)
			sm.wg.Add(1) // 跟踪总 goroutine 数，用于优雅关闭
			sm.add(signal)
			go func(cfg ReceiverConfig) {
				defer sm.done(signal)
				defer groupWg.Done()
				defer sm.wg.Done()

				defer func() {
					if err := recover(); err != nil {
						if errors.Is(err.(error), ErrDropSignal) {
							drop = true
						} else {
							sm.Logger.Fatalw("信号处理错误",
								"signal_type", signal.Type(),
								"error", err)
						}
					}
				}()

				select {
				case <-sm.quit:
				default:
					sm.callReceiver(cfg.Receiver, signal)
				}
			}(config)
		}

		// 等待当前 index 组全部执行完成，再进入下一组
		// fmt.Println("正在处理 index:", idx)
		groupWg.Wait()
	}
}

// run 是 SignalManagerImpl 的核心处理循环
// 它在一个独立的 goroutine 中运行，持续监听信号并分发给监听器
func (sm *SignalManagerImpl) run() {
	for {
		select {
		case signal := <-sm.signalChan:
			// 收到信号，进行分发
			//fmt.Println(signal)
			go func() {
				sm.wg.Add(1)
				sm.add(nil)
				defer func() {
					sm.wg.Done()
					sm.done(nil)
				}()
				sm.dispatch(signal)
			}()
		case <-sm.quit:
			// fmt.Println("停止信号已收到，正在停止信号处理程序...")
			// 收到停止信号，退出循环
			return
		}
	}
}

// Start 启动 SignalManagerImpl 的内部处理循环
// 必须在使用前调用此方法
func (sm *SignalManagerImpl) Start() {
	sm.mu.Lock()
	if sm.running {
		sm.mu.Unlock()
		return
	}

	sm.running = true
	if sm.signalChan == nil {
		sm.signalChan = make(chan Signaler)
	}
	if sm.quit == nil {
		sm.quit = make(chan struct{})
	}
	sm.mu.Unlock()

	go func() {
		sm.wg.Add(1)
		defer sm.wg.Done()
		sm.run()
	}()
}

// Close 停止 SignalManagerImpl，关闭通道并等待内部 goroutine 结束
// 调用后该 manager 不可再使用
func (sm *SignalManagerImpl) Close(spider *Spider) {
	sm.mu.Lock()
	if !sm.running {
		sm.mu.Unlock()
		return
	}
	sm.running = false
	// sm.quit <- struct{}{}
	close(sm.quit)
	// sm.quit = nil
	sm.mu.Unlock()

	// 等待 run goroutine 和所有 dispatch goroutines 完成
	// fmt.Println("正在停止信号处理程序...")
	sm.wg.Wait()
	// fmt.Println("信号处理程序已停止")

	// 清理资源
	sm.mu.Lock()
	sm.quit = nil
	if sm.signalChan != nil {
		close(sm.signalChan)
		sm.signalChan = nil
	}
	// sm.receivers = make(map[string][]ReceiverConfig)
	sm.BaseSpiderModule.Close(spider)
	sm.mu.Unlock()
}

func (sm *SignalManagerImpl) Name() string {
	return "SignalManagerImpl"
}
