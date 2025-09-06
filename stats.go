package xspider

import (
	"sync"
)

func init() {
	RegisterSpiderModuler(&StatserImpl{})
}

type StatserImpl struct {
	m  map[string]any
	mu sync.RWMutex
}

func (ms *StatserImpl) GetValue(key string, defaultValue NumberString) NumberString {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if v, ok := ms.m[key]; ok {
		return v
	}
	return defaultValue
}

func (ms *StatserImpl) GetIntValue(key string, defaultValue int) int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if v, ok := ms.m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		default:
			panic("unsupported type for GetIntValue")
		}
	}
	return defaultValue
}

func (ms *StatserImpl) GetFloat64Value(key string, defaultValue float64) float64 {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if v, ok := ms.m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		default:
			panic("unsupported type for GetFloat64Value")
		}
	}
	return defaultValue
}

func (ms *StatserImpl) GetStringValue(key string, defaultValue string) string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if v, ok := ms.m[key]; ok {
		if stringValue, ok := v.(string); ok {
			return stringValue
		}
		panic("unsupported type for GetStringValue")
	}
	return defaultValue
}

func (ms *StatserImpl) GetStats() map[string]NumberString {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	res := make(map[string]NumberString, len(ms.m))
	for k, v := range ms.m {
		res[k] = v
	}
	return res
}

func (ms *StatserImpl) SetValue(key string, value NumberString) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	switch v := value.(type) {
	case int, float64, string:
		ms.m[key] = v
	default:
		panic("unsupported type for SetValue")
	}
}

func (ms *StatserImpl) SetStats(stats map[string]NumberString) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, v := range stats {
		switch v.(type) {
		case int, float64, string:
		default:
			panic("unsupported type for SetStats")
		}
	}

	ms.m = make(map[string]any, len(stats))
	for k, v := range stats {
		ms.m[k] = v
	}
}

func (ms *StatserImpl) IncValue(key string, count NumberOnly, start NumberOnly) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// 如果键不存在，初始化为start值
	if _, ok := ms.m[key]; !ok {
		switch s := start.(type) {
		case int, float64, string:
			ms.m[key] = s
		default:
			panic("unsupported type for IncValue")
		}
	}

	// 处理递增逻辑
	switch currentValue := ms.m[key].(type) {
	case int:
		switch countValue := count.(type) {
		case int:
			ms.m[key] = currentValue + countValue
		case float64:
			// int转float64进行计算，结果保存为float64
			ms.m[key] = float64(currentValue) + countValue
		default:
			panic("unsupported type for IncValue count parameter")
		}
	case float64:
		switch countValue := count.(type) {
		case int:
			ms.m[key] = currentValue + float64(countValue)
		case float64:
			ms.m[key] = currentValue + countValue
		default:
			panic("unsupported type for IncValue count parameter")
		}
	default:
		panic("unsupported type for IncValue")
	}
}

func (ms *StatserImpl) MaxValue(key string, value NumberOnly) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if v, ok := ms.m[key]; ok {
		switch currentValue := v.(type) {
		case int:
			switch newValue := value.(type) {
			case int:
				if currentValue < newValue {
					ms.m[key] = newValue
				}
			case float64:
				if float64(currentValue) < newValue {
					ms.m[key] = newValue
				}
			default:
				panic("unsupported type for MaxValue parameter")
			}
		case float64:
			switch newValue := value.(type) {
			case int:
				if currentValue < float64(newValue) {
					ms.m[key] = float64(newValue)
				}
			case float64:
				if currentValue < newValue {
					ms.m[key] = newValue
				}
			default:
				panic("unsupported type for MaxValue parameter")
			}
		default:
			panic("unsupported type for MaxValue")
		}
	} else {
		switch v := value.(type) {
		case int, float64:
			ms.m[key] = v
		default:
			panic("unsupported type for MaxValue")
		}
	}
}

func (ms *StatserImpl) MinValue(key string, value NumberOnly) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if v, ok := ms.m[key]; ok {
		switch currentValue := v.(type) {
		case int:
			switch newValue := value.(type) {
			case int:
				if currentValue > newValue {
					ms.m[key] = newValue
				}
			case float64:
				if float64(currentValue) > newValue {
					ms.m[key] = newValue
				}
			default:
				panic("unsupported type for MinValue parameter")
			}
		case float64:
			switch newValue := value.(type) {
			case int:
				if currentValue > float64(newValue) {
					ms.m[key] = float64(newValue)
				}
			case float64:
				if currentValue > newValue {
					ms.m[key] = newValue
				}
			default:
				panic("unsupported type for MinValue parameter")
			}
		default:
			panic("unsupported type for MinValue")
		}
	} else {
		switch v := value.(type) {
		case int, float64:
			ms.m[key] = v
		default:
			panic("unsupported type for MinValue")
		}
	}
}

func (ms *StatserImpl) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.m = make(map[string]any)
}

func (ms *StatserImpl) Close(spider *Spider) {
}

func (ms *StatserImpl) FromSpider(spider *Spider) {
	ms.m = make(map[string]any)
}

func (ms *StatserImpl) Name() string {
	return "StatserImpl"
}
