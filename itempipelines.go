package xspider

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/xue0228/xspider/container"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	RegisterSpiderModuler(&MysqlItemPipeline{})
}

type BaseGormItemPipeline struct {
	BaseSpiderModule
	db *gorm.DB
}

func (p *BaseGormItemPipeline) ProcessItem(item Item, response *Response, spider *Spider) Item {
	log := ResponseLogger(p.Logger, response)

	// 检查类型并转换为指针（如果需要）
	itemPtr, err := ensureStructPointer(item)
	if err != nil {
		log.Errorw("数据类型错误", "error", err, "type", reflect.TypeOf(item).String())
		return item
	}

	if err := p.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(itemPtr).Error
	}); err != nil {
		log.Errorw("保存数据失败", "error", err, "type", GetStructName(item))
		return item
	}
	return item
}

// 检查是否为结构体或结构体指针，并确保返回指针形式
func ensureStructPointer(v interface{}) (interface{}, error) {
	if v == nil {
		return nil, errors.New("item不能为nil")
	}

	val := reflect.ValueOf(v)
	t := val.Type()

	// 如果是结构体，转换为指针
	if t.Kind() == reflect.Struct {
		ptrVal := reflect.New(t)
		ptrVal.Elem().Set(val)
		return ptrVal.Interface(), nil
	}

	// 如果是结构体指针，直接返回
	if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		return v, nil
	}

	// 不支持的类型
	return nil, fmt.Errorf("不支持的类型 %s，需要结构体或结构体指针", t.Kind())
}

func initBaseGormItemPipeline(base *BaseGormItemPipeline, spider *Spider, d gorm.Dialector) {
	db, err := gorm.Open(d, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error), // 只输出警告和错误日志
	})
	if err != nil {
		base.Logger.Fatalw("打开数据库失败",
			"error", err)
	}
	base.db = db
	structsName, err := container.Get[[]string](spider.Settings, "GORM_STRUCTS")
	if err != nil {
		base.Logger.Fatalw("获取GORM_STRUCTS失败",
			"error", err)
	}
	var structs []any
	for _, name := range structsName {
		s := GetAndAssertComponent[any](name)
		structs = append(structs, s)
	}
	err = base.db.AutoMigrate(structs...)
	if err != nil {
		base.Logger.Fatalw("自动迁移失败", "error", err)
	}
	base.Logger.Info("模块初始化完成")
}

type MysqlItemPipeline struct {
	BaseGormItemPipeline
}

func (p *MysqlItemPipeline) Name() string {
	return "MysqlItemPipeline"
}

func (p *MysqlItemPipeline) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&p.BaseSpiderModule, spider, p.Name())

	dsn, err := container.Get[string](spider.Settings, "MYSQL_DSN")
	if err != nil {
		p.Logger.Fatalw("获取MYSQL_DSN失败", "error", err)
	}
	initBaseGormItemPipeline(&p.BaseGormItemPipeline, spider, mysql.Open(dsn))
}
