package xspider

import (
	"sync"
)

// Context provides a tiny layer for passing data between callbacks
type Context struct {
	contextMap map[string]interface{}
	lock       *sync.RWMutex
}

// NewContext initializes a new Context instance
func NewContext() *Context {
	return &Context{
		contextMap: make(map[string]interface{}),
		lock:       &sync.RWMutex{},
	}
}

// Put stores a value of any type in Context
func (c *Context) Put(key string, value interface{}) {
	c.lock.Lock()
	c.contextMap[key] = value
	c.lock.Unlock()
}

func (c *Context) GetString(key string) string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v.(string)
	}
	return ""
}

func (c *Context) GetStringWithDefault(key string, dft string) string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v.(string)
	}
	return dft
}

func (c *Context) GetInt(key string) int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v.(int)
	}
	return 0
}

func (c *Context) GetIntWithDefault(key string, dft int) int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v.(int)
	}
	return dft
}

func (c *Context) GetBool(key string) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v.(bool)
	}
	return false
}

func (c *Context) GetBoolWithDefault(key string, dft bool) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v.(bool)
	}
	return dft
}

func (c *Context) GetFloat64(key string) float64 {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v.(float64)
	}
	return 0.0
}

func (c *Context) GetFloat64WithDefault(key string, dft float64) float64 {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v.(float64)
	}
	return dft
}

func (c *Context) Get(key string) interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v
	}
	return nil
}

func (c *Context) GetWithDefault(key string, dft interface{}) interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.contextMap[key]; ok {
		return v
	}
	return dft
}

func (c *Context) Has(key string) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for k := range c.contextMap {
		if k == key {
			return true
		}
	}
	return false
}

// ForEach iterate context
func (c *Context) ForEach(fn func(k string, v interface{}) interface{}) []interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()

	ret := make([]interface{}, 0, len(c.contextMap))
	for k, v := range c.contextMap {
		ret = append(ret, fn(k, v))
	}

	return ret
}
