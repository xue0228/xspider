package container

import (
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSyncDict(t *testing.T) {
	dict := NewSyncDict()
	assert.NotNil(t, dict)
	assert.Equal(t, 0, dict.Len())
}

func TestSyncDict_SetGet(t *testing.T) {
	dict := NewSyncDict()

	// 测试设置和获取值
	dict.Set("key1", "value1")
	dict.Set("key2", 123)
	dict.Set("key3", true)

	value, exists := dict.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)

	value, exists = dict.Get("key2")
	assert.True(t, exists)
	assert.Equal(t, 123, value)

	value, exists = dict.Get("key3")
	assert.True(t, exists)
	assert.Equal(t, true, value)

	// 测试不存在的键
	value, exists = dict.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, value)
}

func TestSyncDict_GetWithDefault(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("key1", "value1")

	// 测试存在的键
	value := dict.GetWithDefault("key1", "default")
	assert.Equal(t, "value1", value)

	// 测试不存在的键
	value = dict.GetWithDefault("nonexistent", "default")
	assert.Equal(t, "default", value)
}

func TestSyncDict_Has(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("key1", "value1")

	assert.True(t, dict.Has("key1"))
	assert.False(t, dict.Has("nonexistent"))
}

func TestSyncDict_Delete(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("key1", "value1")
	dict.Set("key2", "value2")

	// 删除存在的键
	value, exists := dict.Delete("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)
	assert.False(t, dict.Has("key1"))

	// 删除不存在的键
	value, exists = dict.Delete("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, value)
}

func TestSyncDict_Len(t *testing.T) {
	dict := NewSyncDict()
	assert.Equal(t, 0, dict.Len())

	dict.Set("key1", "value1")
	assert.Equal(t, 1, dict.Len())

	dict.Set("key2", "value2")
	assert.Equal(t, 2, dict.Len())

	dict.Delete("key1")
	assert.Equal(t, 1, dict.Len())
}

func TestSyncDict_Clear(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("key1", "value1")
	dict.Set("key2", "value2")

	assert.Equal(t, 2, dict.Len())

	dict.Clear()
	assert.Equal(t, 0, dict.Len())
}

func TestSyncDict_Update(t *testing.T) {
	dict1 := NewSyncDict()
	dict1.Set("key1", "value1")
	dict1.Set("key2", "value2")

	dict2 := NewSyncDict()
	dict2.Set("key3", "value3")
	dict2.Set("key1", "updated_value1") // 覆盖dict1中的值

	dict1.Update(dict2)

	assert.Equal(t, 3, dict1.Len())
	assert.Equal(t, "updated_value1", dict1.GetWithDefault("key1", ""))
	assert.Equal(t, "value2", dict1.GetWithDefault("key2", ""))
	assert.Equal(t, "value3", dict1.GetWithDefault("key3", ""))
}

func TestSyncDict_Copy(t *testing.T) {
	dict1 := NewSyncDict()
	dict1.Set("key1", "value1")
	dict1.Set("key2", "value2")

	dict2 := dict1.Copy()

	assert.Equal(t, dict1.Len(), dict2.Len())
	assert.Equal(t, dict1.GetWithDefault("key1", ""), dict2.GetWithDefault("key1", ""))
	assert.Equal(t, dict1.GetWithDefault("key2", ""), dict2.GetWithDefault("key2", ""))

	// 修改原字典，确保拷贝是独立的
	dict1.Set("key3", "value3")
	assert.Equal(t, 3, dict1.Len())
	assert.Equal(t, 2, dict2.Len())
}

func TestSyncDict_Map(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("key1", "value1")
	dict.Set("key2", 123)

	mapData := dict.Map()
	assert.Equal(t, 2, len(mapData))
	assert.Equal(t, "value1", mapData["key1"])
	assert.Equal(t, 123, mapData["key2"])
}

func TestSyncDict_ForEach(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("a", 1)
	dict.Set("b", 2)
	dict.Set("c", 3)

	results := dict.ForEach(func(key string, value any) any {
		if v, ok := value.(int); ok {
			return v * 2
		}
		return value
	})

	assert.Equal(t, []any{2, 4, 6}, results)
}

func TestSyncDict_TypeSpecificGetters(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("str", "hello")
	dict.Set("int", 42)
	dict.Set("bool", true)
	dict.Set("float64", 3.14)
	dict.Set("float32", float32(2.71))
	dict.Set("int64", int64(100))

	// 测试字符串获取
	str, ok := dict.GetString("str")
	assert.True(t, ok)
	assert.Equal(t, "hello", str)

	_, ok = dict.GetString("int") // 错误类型
	assert.False(t, ok)

	// 测试整数获取
	i, ok := dict.GetInt("int")
	assert.True(t, ok)
	assert.Equal(t, 42, i)

	_, ok = dict.GetInt("str") // 错误类型
	assert.False(t, ok)

	// 测试布尔获取
	b, ok := dict.GetBool("bool")
	assert.True(t, ok)
	assert.True(t, b)

	_, ok = dict.GetBool("str") // 错误类型
	assert.False(t, ok)

	// 测试浮点数获取
	f, ok := dict.GetFloat64("float64")
	assert.True(t, ok)
	assert.Equal(t, 3.14, f)

	f, ok = dict.GetFloat64("float32")
	assert.True(t, ok)
	assert.Equal(t, float64(float32(2.71)), f)

	f, ok = dict.GetFloat64("int")
	assert.True(t, ok)
	assert.Equal(t, 42.0, f)

	f, ok = dict.GetFloat64("int64")
	assert.True(t, ok)
	assert.Equal(t, 100.0, f)

	_, ok = dict.GetFloat64("str") // 错误类型
	assert.False(t, ok)
}

func TestSyncDict_TypeSpecificGettersWithDefault(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("str", "hello")
	dict.Set("int", 42)
	dict.Set("bool", true)
	dict.Set("float", 3.14)

	// 测试字符串默认值
	str := dict.GetStringWithDefault("str", "default")
	assert.Equal(t, "hello", str)

	str = dict.GetStringWithDefault("nonexistent", "default")
	assert.Equal(t, "default", str)

	// 测试整数默认值
	i := dict.GetIntWithDefault("int", 99)
	assert.Equal(t, 42, i)

	i = dict.GetIntWithDefault("nonexistent", 99)
	assert.Equal(t, 99, i)

	// 测试布尔默认值
	b := dict.GetBoolWithDefault("bool", false)
	assert.True(t, b)

	b = dict.GetBoolWithDefault("nonexistent", true)
	assert.True(t, b)

	// 测试浮点数默认值
	f := dict.GetFloat64WithDefault("float", 1.0)
	assert.Equal(t, 3.14, f)

	f = dict.GetFloat64WithDefault("nonexistent", 1.0)
	assert.Equal(t, 1.0, f)
}

func TestSyncDict_ConcurrentSafety(t *testing.T) {
	dict := NewSyncDict()
	const goroutines = 100
	const operations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// 并发写入
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := "key_" + strconv.Itoa(id) + "_" + strconv.Itoa(j)
				dict.Set(key, j)
			}
		}(i)
	}

	// 并发读取
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := "key_" + strconv.Itoa(id) + "_" + strconv.Itoa(j)
				dict.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// 验证最终状态
	expectedCount := goroutines * operations
	assert.Equal(t, expectedCount, dict.Len())
}

func TestSyncDict_DumpsLoads(t *testing.T) {
	dict1 := NewSyncDict()
	dict1.Set("key3", true)
	dict1.Set("key1", "value1")
	dict1.Set("key2", 123)

	dict4 := NewSyncDict()
	dict4.Set("key4", false)
	dict4.Set("key5", 456)

	dict3 := NewSyncDict()
	//dict3.Set("key6", "value6")
	//dict3.Set("key7", 789)

	dict1.Set("key8", dict4)
	dict4.Set("key9", dict3)

	// 序列化
	data, err := dict1.Dumps()
	t.Log("序列化结果：", string(data))
	assert.NoError(t, err)
	assert.NotNil(t, data)

	// 反序列化
	dict2 := NewSyncDict()
	err = dict2.Loads(data)
	t.Log("反序列化结果：", dict2)
	assert.NoError(t, err)

	// 验证数据一致性
	assert.Equal(t, dict1.Len(), dict2.Len())

	str, _ := dict2.GetString("key1")
	assert.Equal(t, "value1", str)

	i, _ := dict2.GetInt("key2")
	assert.Equal(t, 123, i)

	b, _ := dict2.GetBool("key3")
	assert.True(t, b)
}

func TestSyncDict_LoadsEmpty(t *testing.T) {
	dict := NewSyncDict()
	dict.Set("key1", "value1")

	// 创建空的JSON
	emptyJSON := []byte("{}")

	err := dict.Loads(emptyJSON)
	assert.NoError(t, err)
	assert.Equal(t, 0, dict.Len())
}
