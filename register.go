package xspider

import (
	"fmt"
	"reflect"
	"sync"
)

type register struct {
	m  map[string]any
	mu sync.RWMutex
}

var (
	instance *register
	once     sync.Once
)

func getRegister() *register {
	once.Do(func() {
		instance = &register{
			m:  make(map[string]any),
			mu: sync.RWMutex{},
		}
	})
	return instance
}

func Register(name string, value any) error {
	if value == nil {
		return fmt.Errorf("value cannot be nil")
	}

	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	register := getRegister()
	register.mu.Lock()
	defer register.mu.Unlock()

	if _, exists := register.m[name]; exists {
		return fmt.Errorf("value with name '%s' is already registered", name)
	}

	register.m[name] = value
	return nil
}

func Unregister(name string) bool {
	register := getRegister()
	register.mu.Lock()
	defer register.mu.Unlock()

	if _, exists := register.m[name]; !exists {
		return false
	}

	delete(register.m, name)
	return true
}

// GetRegisteredByName 根据名称获取注册项
// 对于函数类型 (CallbackFunc、ErrbackFunc)，直接返回注册的实例
// 对于非函数类型 (Middlewarer 结构体)，通过反射创建一个新实例
func GetRegisteredByName(name string) (any, bool) {
	if name == "" {
		return nil, false
	}

	reg := getRegister()
	reg.mu.RLock()
	instance, exists := reg.m[name]
	reg.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// 获取实例的反射类型
	instanceType := reflect.TypeOf(instance)
	if instanceType == nil {
		return nil, false
	}

	// 判断是否为函数类型
	if instanceType.Kind() == reflect.Func {
		// 是函数，直接返回原实例
		return instance, true
	}

	// 不是函数，视为需要创建新实例的类型 (如 Middlewarer)
	// 检查是否是指针
	if instanceType.Kind() == reflect.Ptr {
		// 获取指针指向的具体类型
		elemType := instanceType.Elem()
		// 创建一个指向 elemType 零值的新指针
		newPtr := reflect.New(elemType)
		// 转换为 interface{}
		return newPtr.Interface(), true
	} else {
		// 是值类型，创建该类型的零值
		zeroValue := reflect.Zero(instanceType)
		return zeroValue.Interface(), true
	}
}

func RegisterSpiderModuler(sm SpiderModuler) {
	err := Register(sm.Name(), sm)
	if err != nil {
		panic(err)
	}
}

func RegisterStructs(structs ...any) {
	for _, s := range structs {
		err := Register(GetStructName(s), s)
		if err != nil {
			panic(err)
		}
	}
}
