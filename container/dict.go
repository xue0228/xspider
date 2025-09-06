package container

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/emirpasic/gods/maps/hashmap"
)

// Dict 字典
type Dict interface {
	Update(Dict)
	Clear()
	Copy() Dict
	Len() int
	Get(string) (any, bool)
	Set(string, any)
	GetWithDefault(string, any) any
	Has(string) bool
	Delete(string) (any, bool)
	Keys() []string
	Values() []any
	Map() map[string]any
	ForEach(func(string, any) any) []any
	GetString(string) (string, bool)
	GetInt(string) (int, bool)
	GetBool(string) (bool, bool)
	GetFloat64(string) (float64, bool)
	GetStringWithDefault(string, string) string
	GetIntWithDefault(string, int) int
	GetBoolWithDefault(string, bool) bool
	GetFloat64WithDefault(string, float64) float64
	// Dumps 将字典序列化为json格式的字节数组
	Dumps() ([]byte, error)
	// Loads 将json格式的字节数组反序列化为字典
	Loads([]byte) error
}

// SyncDict 线程安全的有序字典实现
type SyncDict struct {
	m    *hashmap.Map
	lock sync.RWMutex // 读写锁
}

// NewSyncDict 创建一个新的线程安全有序字典
func NewSyncDict() Dict {
	return &SyncDict{
		m: hashmap.New(),
	}
}

// Update 合并另一个字典（保持顺序）
func (d *SyncDict) Update(other Dict) {
	otherMap := other.Map()
	d.lock.Lock()
	defer d.lock.Unlock()
	for k, v := range otherMap {
		d.m.Put(k, v)
	}
}

// Clear 清空字典
func (d *SyncDict) Clear() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.m.Clear()
}

// Copy 返回深拷贝（值为浅拷贝，结构深拷贝）
func (d *SyncDict) Copy() Dict {
	d.lock.RLock()
	defer d.lock.RUnlock()

	other := &SyncDict{
		m:    hashmap.New(),
		lock: sync.RWMutex{},
	}

	keys := d.m.Keys()
	for _, key := range keys {
		v, _ := d.m.Get(key)
		other.m.Put(key, v)
	}
	return other
}

// Len 返回字典长度
func (d *SyncDict) Len() int {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.m.Size()
}

// Get 获取值
func (d *SyncDict) Get(key string) (any, bool) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	value, exists := d.m.Get(key)
	return value, exists
}

// Set 设置值
func (d *SyncDict) Set(key string, value any) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.m.Put(key, value)
}

// GetWithDefault 获取值，不存在时返回默认值
func (d *SyncDict) GetWithDefault(key string, def any) any {
	if val, ok := d.Get(key); ok {
		return val
	}
	return def
}

// Has 检查键是否存在
func (d *SyncDict) Has(key string) bool {
	_, ok := d.Get(key)
	return ok
}

// Delete 删除键
func (d *SyncDict) Delete(key string) (any, bool) {
	d.lock.Lock()
	defer d.lock.Unlock()

	value, exists := d.m.Get(key)
	if !exists {
		return nil, false
	}
	d.m.Remove(key)
	return value, true
}

// Keys 返回所有键，按字典序排列
func (d *SyncDict) Keys() []string {
	d.lock.RLock()
	defer d.lock.RUnlock()

	keys := d.m.Keys()
	strKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		if str, ok := k.(string); ok {
			strKeys = append(strKeys, str)
		}
	}
	return strKeys
}

// Values 返回所有值，顺序与 Keys() 一致
func (d *SyncDict) Values() []any {
	d.lock.RLock()
	defer d.lock.RUnlock()

	values := d.m.Values()
	return values
}

// Map 返回 map[string]any，按键的字典序排列
func (d *SyncDict) Map() map[string]any {
	d.lock.RLock()
	defer d.lock.RUnlock()

	m := make(map[string]any)
	keys := d.m.Keys()
	for _, key := range keys {
		if k, ok := key.(string); ok {
			v, _ := d.m.Get(k)
			m[k] = v
		}
	}
	return m
}

// ForEach 遍历字典，按键的字典序执行函数，返回结果切片
func (d *SyncDict) ForEach(fn func(string, any) any) []any {
	d.lock.RLock()
	defer d.lock.RUnlock()

	var results []any
	keys := d.m.Keys()
	for _, key := range keys {
		if k, ok := key.(string); ok {
			v, _ := d.m.Get(k)
			res := fn(k, v)
			results = append(results, res)
		}
	}
	return results
}

// 类型获取方法
func (d *SyncDict) GetString(key string) (string, bool) {
	v, ok := d.Get(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (d *SyncDict) GetInt(key string) (int, bool) {
	v, ok := d.Get(key)
	if !ok {
		return 0, false
	}

	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		// 只有当浮点数表示的是整数（如 3.0）时才转换
		if n == float32(int(n)) {
			return int(n), true
		}
	case float64:
		// 同上
		if n == float64(int(n)) {
			return int(n), true
		}
	}

	return 0, false
}

func (d *SyncDict) GetBool(key string) (bool, bool) {
	v, ok := d.Get(key)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func (d *SyncDict) GetFloat64(key string) (float64, bool) {
	v, ok := d.Get(key)
	if !ok {
		return 0, false
	}
	switch v := v.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}

// 带默认值的类型获取

func (d *SyncDict) GetStringWithDefault(key string, def string) string {
	if v, ok := d.GetString(key); ok {
		return v
	}
	return def
}

func (d *SyncDict) GetIntWithDefault(key string, def int) int {
	if v, ok := d.GetInt(key); ok {
		return v
	}
	return def
}

func (d *SyncDict) GetBoolWithDefault(key string, def bool) bool {
	if v, ok := d.GetBool(key); ok {
		return v
	}
	return def
}

func (d *SyncDict) GetFloat64WithDefault(key string, def float64) float64 {
	if v, ok := d.GetFloat64(key); ok {
		return v
	}
	return def
}

//// Dumps 序列化为 JSON 字节数组，按键排序
//func (d *SyncDict) Dumps() ([]byte, error) {
//	d.lock.RLock()
//	defer d.lock.RUnlock()
//
//	m := d.Map()
//	// 步骤1：提取所有键并排序
//	keys := make([]string, 0, len(m))
//	for k := range m {
//		keys = append(keys, k)
//	}
//	// 按字母顺序排序（可自定义排序逻辑）
//	sort.Strings(keys)
//
//	// 步骤2：按排序后的键构建JSON
//	var sb strings.Builder
//	sb.WriteString("{")
//	for i, k := range keys {
//		// 序列化值
//		val, err := json.Marshal(m[k])
//		if err != nil {
//			return nil, err
//		}
//		// 写入键值对
//		sb.WriteString(fmt.Sprintf("%q:%s", k, val))
//		// 最后一个键不加逗号
//		if i != len(keys)-1 {
//			sb.WriteString(",")
//		}
//	}
//	sb.WriteString("}")
//
//	return []byte(sb.String()), nil
//}

// Dumps 序列化为 JSON 字节数组，按键排序，递归处理嵌套的 Dict 接口
func (d *SyncDict) Dumps() ([]byte, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return d.dumpsLocked()
}

// dumpsLocked 内部递归序列化方法
func (d *SyncDict) dumpsLocked() ([]byte, error) {
	m := d.Map()
	// 步骤1：提取所有键并排序
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 按字母顺序排序
	sort.Strings(keys)

	// 步骤2：按排序后的键构建JSON
	var sb strings.Builder
	sb.WriteString("{")

	for i, k := range keys {
		val := m[k]

		// 递归处理嵌套的 Dict 接口
		if nestedDict, ok := val.(*SyncDict); ok {
			nestedData, err := nestedDict.dumpsLocked()
			if err != nil {
				return nil, err
			}
			sb.WriteString(fmt.Sprintf("%q:%s", k, nestedData))
		} else {
			// 序列化普通值
			valBytes, err := json.Marshal(val)
			if err != nil {
				return nil, err
			}
			sb.WriteString(fmt.Sprintf("%q:%s", k, valBytes))
		}

		// 最后一个键不加逗号
		if i != len(keys)-1 {
			sb.WriteString(",")
		}
	}
	sb.WriteString("}")

	return []byte(sb.String()), nil
}

//// Loads 从 JSON 字节数组反序列化
//func (d *SyncDict) Loads(data []byte) error {
//	d.lock.Lock()
//	defer d.lock.Unlock()
//
//	// 清空旧数据
//	d.m.Clear()
//
//	if err := d.m.FromJSON(data); err != nil {
//		return err
//	}
//	return nil
//}

// Loads 从 JSON 字节数组反序列化，递归地将所有 map 转换为 Dict
func (d *SyncDict) Loads(data []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	// 清空旧数据
	d.m.Clear()

	// 先反序列化为 generic interface{}
	var rawData interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return err
	}

	// 递归处理数据，将 map 转换为 Dict
	processedData := d.processLoadedData(rawData)

	// 如果顶层是一个 map，将其内容放入当前字典
	if loadedMap, ok := processedData.(map[string]interface{}); ok {
		for k, v := range loadedMap {
			d.m.Put(k, v)
		}
	} else if dict, ok := processedData.(*SyncDict); ok {
		// 如果处理结果直接是一个 SyncDict，则复制其内容
		keys := dict.m.Keys()
		for _, key := range keys {
			if k, ok := key.(string); ok {
				v, _ := dict.m.Get(k)
				d.m.Put(k, v)
			}
		}
	}

	return nil
}

// processLoadedData 递归处理加载的数据，将 map 转换为 Dict
func (d *SyncDict) processLoadedData(data interface{}) interface{} {
	switch val := data.(type) {
	case map[string]interface{}:
		// 创建新的 SyncDict 来包装这个 map
		dict := &SyncDict{
			m:    hashmap.New(),
			lock: sync.RWMutex{},
		}

		// 递归处理 map 中的每个值
		for k, v := range val {
			processedValue := d.processLoadedData(v)
			dict.m.Put(k, processedValue)
		}

		return dict

	case []interface{}:
		// 递归处理数组/切片中的每个元素
		processedSlice := make([]interface{}, len(val))
		for i, item := range val {
			processedSlice[i] = d.processLoadedData(item)
		}
		return processedSlice

	default:
		// 其他类型保持原样
		return val
	}
}
