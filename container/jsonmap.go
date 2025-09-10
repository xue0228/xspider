package container

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
)

type JsonMap interface {
	Set(string, any) error
	Get(key string) (any, error)
	Clear()
	Keys() []string
	Values() []any
	GetMap() map[string]any
	SetMap(map[string]any) error
	Len() int
	Has(string) bool
	Delete(string) (any, error)
	Copy() JsonMap
	Update(JsonMap) error
	// Dumps 将字典序列化为json格式的字节数组
	Dumps() ([]byte, error)
	// Loads 将json格式的字节数组反序列化为字典
	Loads([]byte) error
}

type SyncJsonMap struct {
	m    map[string]any
	lock sync.RWMutex
}

func (jm *SyncJsonMap) Dumps() ([]byte, error) {
	jm.lock.RLock()
	defer jm.lock.RUnlock()

	m := jm.GetMap()
	return dumps(m)
}

func dumps(m map[string]any) ([]byte, error) {
	if m == nil {
		return nil, errors.New("值不能为空")
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("{")

	for i, k := range keys {
		val := m[k]

		if nestedDict, ok := val.(map[string]any); ok {
			nestedData, err := dumps(nestedDict)
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

func (jm *SyncJsonMap) Loads(bs []byte) error {
	jm.lock.Lock()
	defer jm.lock.Unlock()

	var newMap map[string]any
	err := json.Unmarshal(bs, &newMap)
	if err != nil {
		return fmt.Errorf("JSON反序列化失败: %w", err)
	}
	// 深度复制值，确保存储的是JSON兼容的结构
	normalizedValue, err := NormalizeValue(newMap)
	if err != nil {
		return fmt.Errorf("规范化值失败: %w", err)
	}

	jm.m = normalizedValue.(map[string]any)
	return nil
}

func NewSyncJsonMap() *SyncJsonMap {
	return &SyncJsonMap{m: make(map[string]any)}
}

// Set 向字典中设置键值对，只接受JSON支持的类型
// 支持的类型：string, bool, 所有数值类型, nil, map[string]interface{}, []interface{}
// 以及这些类型的嵌套组合
func (jm *SyncJsonMap) Set(key string, value any) error {
	jm.lock.Lock()
	defer jm.lock.Unlock()

	// 验证值是否为JSON支持的类型
	if !IsJSONCompatible(value) {
		return fmt.Errorf("不支持的类型 %T，仅支持JSON兼容的类型", value)
	}

	// 深度复制值，确保存储的是JSON兼容的结构
	normalizedValue, err := NormalizeValue(value)
	if err != nil {
		return fmt.Errorf("规范化值失败: %w", err)
	}

	jm.m[key] = normalizedValue
	return nil
}

func (jm *SyncJsonMap) Get(key string) (any, error) {
	jm.lock.RLock()
	defer jm.lock.RUnlock()

	if v, ok := jm.m[key]; ok {
		return v, nil
	} else {
		return nil, fmt.Errorf("键 %s 不存在", key)
	}
}

func (jm *SyncJsonMap) Clear() {
	jm.lock.Lock()
	defer jm.lock.Unlock()

	jm.m = make(map[string]any)
}

func (jm *SyncJsonMap) Keys() []string {
	jm.lock.RLock()
	defer jm.lock.RUnlock()

	keys := make([]string, 0, len(jm.m))
	for k := range jm.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (jm *SyncJsonMap) Values() []any {
	jm.lock.RLock()
	defer jm.lock.RUnlock()

	values := make([]any, 0, len(jm.m))
	for _, key := range jm.Keys() {
		v, _ := jm.Get(key)
		values = append(values, v)
	}
	return values
}

func (jm *SyncJsonMap) GetMap() map[string]any {
	jm.lock.RLock()
	defer jm.lock.RUnlock()

	res, err := NormalizeValue(jm.m)
	if err != nil {
		panic(err)
	}
	return res.(map[string]any)
}

func (jm *SyncJsonMap) SetMap(m map[string]any) error {
	jm.lock.Lock()
	defer jm.lock.Unlock()

	// 验证值是否为JSON支持的类型
	if !IsJSONCompatible(m) {
		return fmt.Errorf("不支持的类型 %T，仅支持JSON兼容的类型", m)
	}

	// 深度复制值，确保存储的是JSON兼容的结构
	normalizedValue, err := NormalizeValue(m)
	if err != nil {
		return fmt.Errorf("规范化值失败: %w", err)
	}

	jm.m = normalizedValue.(map[string]any)
	return nil
}

func (jm *SyncJsonMap) Len() int {
	jm.lock.RLock()
	defer jm.lock.RUnlock()

	return len(jm.m)
}

func (jm *SyncJsonMap) Has(key string) bool {
	jm.lock.RLock()
	defer jm.lock.RUnlock()

	_, ok := jm.m[key]
	return ok
}

func (jm *SyncJsonMap) Delete(key string) (any, error) {
	jm.lock.Lock()
	defer jm.lock.Unlock()

	if v, ok := jm.m[key]; ok {
		delete(jm.m, key)
		return v, nil
	} else {
		return nil, fmt.Errorf("键 %s 不存在", key)
	}
}

func (jm *SyncJsonMap) Copy() JsonMap {
	jm.lock.RLock()
	defer jm.lock.RUnlock()

	m := NewSyncJsonMap()
	err := m.SetMap(jm.m)
	if err != nil {
		panic(err)
	}
	return m
}

func (jm *SyncJsonMap) Update(m JsonMap) error {
	jm.lock.Lock()
	defer jm.lock.Unlock()

	for _, key := range m.Keys() {
		v, err := m.Get(key)
		if err != nil {
			return err
		}
		vv, err := NormalizeValue(v)
		if err != nil {
			return err
		}
		jm.m[key] = vv
	}
	return nil
}

func GetWithDefault[T any](m JsonMap, key string, defaultValue T) T {
	v, err := m.Get(key)
	if err != nil {
		return defaultValue
	}
	res, err := ConvertToJsonSupportType[T](v)
	if err != nil {
		panic(err)
	}
	return res
}

func Get[T any](m JsonMap, key string) (T, error) {
	var zero T
	v, err := m.Get(key)
	if err != nil {
		return zero, err
	}
	res, err := ConvertToJsonSupportType[T](v)
	if err != nil {
		return zero, err
	}
	return res, nil
}

func Set(m JsonMap, key string, value any) {
	err := m.Set(key, value)
	if err != nil {
		panic(err)
	}
}

// IsJSONCompatible 检查值是否为JSON兼容的类型
func IsJSONCompatible(value any) bool {
	if value == nil {
		return true
	}

	val := reflect.ValueOf(value)

	// 如果是指针，获取其指向的值
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return true
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.String, reflect.Bool:
		return true

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true

	case reflect.Map:
		// 检查map的键是否为string类型
		if val.Type().Key().Kind() != reflect.String {
			return false
		}
		// 递归检查map中的所有值
		for _, key := range val.MapKeys() {
			elem := val.MapIndex(key)
			if !IsJSONCompatible(elem.Interface()) {
				return false
			}
		}
		return true

	case reflect.Slice, reflect.Array:
		// 递归检查slice中的所有元素
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			if !IsJSONCompatible(elem.Interface()) {
				return false
			}
		}
		return true

	default:
		return false
	}
}

// NormalizeValue 将值规范化为JSON兼容的基本类型
func NormalizeValue(value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	val := reflect.ValueOf(value)

	// 处理指针
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, nil
		}
		return NormalizeValue(val.Elem().Interface())
	}

	switch val.Kind() {
	case reflect.String:
		return val.String(), nil

	case reflect.Bool:
		return val.Bool(), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int(), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return val.Uint(), nil

	case reflect.Float32, reflect.Float64:
		return val.Float(), nil

	case reflect.Map:
		// 确保map键是string类型
		if val.Type().Key().Kind() != reflect.String {
			return nil, errors.New("map的键必须是string类型")
		}

		result := make(map[string]interface{})
		for _, key := range val.MapKeys() {
			elem, err := NormalizeValue(val.MapIndex(key).Interface())
			if err != nil {
				return nil, err
			}
			result[key.String()] = elem
		}
		return result, nil

	case reflect.Slice, reflect.Array:
		result := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			elem, err := NormalizeValue(val.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			result[i] = elem
		}
		return result, nil

	default:
		return nil, fmt.Errorf("不支持的类型 %s", val.Kind())
	}
}

// ConvertToJsonSupportType 将任意值递归转换为指定的 JSON 可支持类型 T（如 map[string]interface{}、[]interface{} 等）。
// 支持 map、slice、基本数值类型及其嵌套结构的转换。
// 若转换失败，返回零值和错误。
func ConvertToJsonSupportType[T any](value any) (T, error) {
	var zero T
	if value == nil {
		return zero, nil
	}

	targetType := reflect.TypeOf(zero)
	sourceVal := reflect.ValueOf(value)

	// 处理目标类型为指针的情况
	if targetType.Kind() == reflect.Ptr {
		elemType := targetType.Elem()           // 获取指针指向的元素类型
		elemVal := reflect.New(elemType).Elem() // 创建该类型的零值实例
		if err := setValue(elemVal, sourceVal.Interface()); err != nil {
			return zero, err
		}

		// 构造指针并设置值
		ptrVal := reflect.New(elemType)
		ptrVal.Elem().Set(elemVal)

		result, ok := ptrVal.Interface().(T)
		if !ok {
			return zero, fmt.Errorf("无法将 %T 转换为指针类型 %T", value, zero)
		}
		return result, nil
	}

	// 处理非指针类型：创建目标类型的可设置实例
	targetVal := reflect.New(targetType).Elem()
	if err := setValue(targetVal, value); err != nil {
		return zero, err
	}

	result, ok := targetVal.Interface().(T)
	if !ok {
		return zero, fmt.Errorf("无法将 %T 转换为非指针类型 %T", value, zero)
	}
	return result, nil
}

// setValue 将 source 值递归设置到 target reflect.Value 中
// target 必须是可设置的（如通过 reflect.New 创建的 Elem）
func setValue(target reflect.Value, value any) error {
	if value == nil {
		return nil
	}

	sourceVal := reflect.ValueOf(value)

	// 解包接口
	if sourceVal.Kind() == reflect.Interface && !sourceVal.IsNil() {
		sourceVal = sourceVal.Elem()
	}

	// 解包指针（多层指针也支持）
	for sourceVal.Kind() == reflect.Ptr {
		if sourceVal.IsNil() {
			return nil
		}
		sourceVal = sourceVal.Elem()
	}

	// 如果类型兼容，直接赋值（如 int -> int, string -> string）
	if sourceVal.Type().AssignableTo(target.Type()) {
		target.Set(sourceVal)
		return nil
	}

	// -----------------------------
	// 新增：处理目标为指针的情况
	// -----------------------------
	//if target.Kind() == reflect.Ptr {
	//	// 如果目标是指针，需要为其分配内存并递归设置指向的值
	//	if target.IsNil() {
	//		// 分配内存
	//		target.Set(reflect.New(target.Type().Elem()))
	//	}
	//
	//	// 递归设置指针指向的值
	//	return setValue(target.Elem(), value)
	//}

	// 分别处理 map、slice、数值类型
	switch target.Kind() {
	case reflect.Map:
		return convertToMap(target, sourceVal)
	case reflect.Slice, reflect.Array:
		return convertToSlice(target, sourceVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		if isNumeric(sourceVal.Kind()) {
			return setNumericValue(target, sourceVal)
		}
		return fmt.Errorf("无法将 %T 转换为数值类型 %s", value, target.Kind())
	default:
		return fmt.Errorf("不支持的类型转换：无法将 %T 转换为 %s", value, target.Type())
	}
}

// convertToMap 将 sourceVal 转换并赋值给目标 map[string]V 类型
// 要求：目标 map 的键必须是 string 类型
// 源 map 的键必须能被赋值给 string（如 string、[]byte），否则报错
func convertToMap(target reflect.Value, sourceVal reflect.Value) error {
	// 检查目标 map 的键类型是否为 string
	if target.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("目标 map 键必须是 string 类型，但实际为 %s", target.Type().Key().Kind())
	}

	if sourceVal.Kind() != reflect.Map {
		return fmt.Errorf("无法将 %T 转换为 map[string]... 类型", sourceVal.Interface())
	}

	// 初始化目标 map
	target.Set(reflect.MakeMap(target.Type()))
	elemType := target.Type().Elem()

	for _, key := range sourceVal.MapKeys() {
		sourceMapVal := sourceVal.MapIndex(key)

		// 创建目标值的实例并递归设置
		targetMapVal := reflect.New(elemType).Elem()
		if err := setValue(targetMapVal, sourceMapVal.Interface()); err != nil {
			return fmt.Errorf("转换 map 值失败，键 %v: %w", key.Interface(), err)
		}

		// 检查源键是否能赋值给 string（如 string、[]byte）
		stringType := reflect.TypeOf("")
		if !key.Type().AssignableTo(stringType) {
			return fmt.Errorf("源 map 键类型 %s 无法赋值给 string", key.Type())
		}

		// 直接使用已转换的 string 键
		target.SetMapIndex(key.Convert(stringType), targetMapVal)
	}
	return nil
}

// convertToSlice 将 sourceVal 转换并赋值给目标 slice 或 array
func convertToSlice(target reflect.Value, sourceVal reflect.Value) error {
	if sourceVal.Kind() != reflect.Slice && sourceVal.Kind() != reflect.Array {
		return fmt.Errorf("无法将 %T 转换为 slice 或 array 类型", sourceVal.Interface())
	}

	elemType := target.Type().Elem()
	length := sourceVal.Len()
	capacity := sourceVal.Cap()

	// 创建目标 slice
	newSlice := reflect.MakeSlice(reflect.SliceOf(elemType), length, capacity)
	target.Set(newSlice)

	// 逐个转换元素
	for i := 0; i < length; i++ {
		elemVal := newSlice.Index(i)
		if err := setValue(elemVal, sourceVal.Index(i).Interface()); err != nil {
			return fmt.Errorf("转换 slice 元素索引 %d 失败: %w", i, err)
		}
	}
	return nil
}

// isNumeric 判断 reflect.Kind 是否为数值类型
func isNumeric(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// setNumericValue 在 source 为数值类型时，将其值设置到 target
func setNumericValue(target, source reflect.Value) error {
	switch source.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setIntValue(target, source.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return setUintValue(target, source.Uint())
	case reflect.Float32, reflect.Float64:
		return setFloatValue(target, source.Float())
	default:
		return fmt.Errorf("不支持的源数值类型: %s", source.Kind())
	}
}

// setIntValue 将 int64 值转换并设置到目标数值类型
func setIntValue(target reflect.Value, val int64) error {
	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		target.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if val < 0 {
			return fmt.Errorf("无法将负数 %d 转换为无符号类型 %s", val, target.Kind())
		}
		target.SetUint(uint64(val))
	case reflect.Float32, reflect.Float64:
		target.SetFloat(float64(val))
	default:
		return fmt.Errorf("不支持的目标类型: %s", target.Kind())
	}
	return nil
}

// setUintValue 将 uint64 值转换并设置到目标数值类型
func setUintValue(target reflect.Value, val uint64) error {
	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val > uint64(math.MaxInt64) {
			return fmt.Errorf("值 %d 超出 int64 最大值，无法转换", val)
		}
		target.SetInt(int64(val))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		target.SetUint(val)
	case reflect.Float32, reflect.Float64:
		target.SetFloat(float64(val))
	default:
		return fmt.Errorf("不支持的目标类型: %s", target.Kind())
	}
	return nil
}

// setFloatValue 将 float64 值转换并设置到目标数值类型
func setFloatValue(target reflect.Value, val float64) error {
	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val < float64(math.MinInt64) || val > float64(math.MaxInt64) {
			return fmt.Errorf("值 %f 超出 int64 范围，无法转换", val)
		}
		target.SetInt(int64(val))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if val < 0 || val > float64(math.MaxUint64) {
			return fmt.Errorf("值 %f 超出 uint64 范围，无法转换", val)
		}
		target.SetUint(uint64(val))
	case reflect.Float32, reflect.Float64:
		target.SetFloat(val)
	default:
		return fmt.Errorf("不支持的目标类型: %s", target.Kind())
	}
	return nil
}
