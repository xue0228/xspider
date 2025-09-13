package xspider

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

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
	//failMap map[string]map[uint]struct{}
	//lock    sync.RWMutex
}

//func (p *BaseGormItemPipeline) setFail(table string, id uint) {
//	p.lock.Lock()
//	defer p.lock.Unlock()
//	if _, ok := p.failMap[table]; !ok {
//		p.failMap[table] = make(map[uint]struct{})
//	}
//	p.failMap[table][id] = struct{}{}
//}
//
//func (p *BaseGormItemPipeline) isFail(table string, id uint) bool {
//	p.lock.RLock()
//	defer p.lock.RUnlock()
//	if _, ok := p.failMap[table]; !ok {
//		return false
//	}
//	_, ok := p.failMap[table][id]
//	return ok
//}
//
//func (p *BaseGormItemPipeline) removeFail(table string, id uint) {
//	p.lock.Lock()
//	defer p.lock.Unlock()
//	if _, ok := p.failMap[table]; !ok {
//		return
//	}
//	delete(p.failMap[table], id)
//}

func (p *BaseGormItemPipeline) finishRequest(item Item, response *Response, spider *Spider) {
	log := ResponseLogger(p.Logger, response)
	table, err := container.Get[string](response.Ctx, "table")
	if err != nil {
		log.Debugw("请求状态更新失败", "error", err, "table", table)
		return
	}
	id, err := container.Get[uint](response.Ctx, "id")
	if err != nil {
		log.Debugw("请求状态更新失败", "error", err, "table", table, "id", id)
		return
	}

	//if !p.isFail(table, id) {
	//	result := p.db.Table(table).Where("id = ?", id).Update("status", 2)
	//	if result.Error != nil {
	//		p.Logger.Error("更新请求状态失败", "error", result.Error, "table", table, "id", id)
	//	}
	//	p.removeFail(table, id)
	//}
}

func (p *BaseGormItemPipeline) ProcessItem(item Item, response *Response, spider *Spider) Item {
	log := ResponseLogger(p.Logger, response)

	//needRecord := true

	//table, err := container.Get[string](response.Ctx, "table")
	//if err != nil {
	//	needRecord = false
	//}
	//id, err := container.Get[uint](response.Ctx, "id")
	//if err != nil {
	//	needRecord = false
	//}

	// 检查类型并转换为指针（如果需要）
	itemPtr, err := ensureStructPointer(item)
	if err != nil {
		log.Errorw("数据类型错误", "error", err, "type", reflect.TypeOf(item).String())
		//if needRecord {
		//	p.setFail(table, id)
		//}
		return item
	}

	if err := p.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(itemPtr).Error
	}); err != nil {
		if strings.Contains(err.Error(), "Error 1062") {
			log.Debugw("数据已存在，跳过保存", "type", GetStructName(item))
		} else {
			log.Errorw("保存数据失败", "error", err, "type", GetStructName(item))
			//if needRecord {
			//	p.setFail(table, id)
			//}
			return item
		}
	}

	return item
}

func (p *BaseGormItemPipeline) Close(spider *Spider) {
	sqlDB, err := p.db.DB()
	if err != nil {
		p.Logger.Error("关闭数据库连接失败", "error", err)
	} else {
		err := sqlDB.Close()
		if err != nil {
			p.Logger.Error("关闭数据库连接失败", "error", err)
		}
	}
	p.BaseSpiderModule.Close(spider)
}

// 检查是否为结构体或结构体指针，并确保返回指针形式
func ensureStructPointer(v any) (any, error) {
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
	//spider.Signal.Connect(base.finishRequest, StItemDropped, 500)
	//spider.Signal.Connect(base.finishRequest, StItemScraped, 500)

	//base.failMap = make(map[string]map[uint]struct{})

	db, err := gorm.Open(d, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error), // 只输出警告和错误日志
	})
	if err != nil {
		base.Logger.Fatalw("打开数据库失败",
			"error", err)
	}

	// 2. 获取底层 database/sql 的 DB 实例，配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		base.Logger.Fatalw("获取数据库连接失败", "error", err)
	}

	// 3. 连接池核心配置（根据爬虫并发量调整）
	sqlDB.SetMaxOpenConns(100)                 // 最大活跃连接数：不超过 MySQL 允许的最大连接数（默认 151）
	sqlDB.SetMaxIdleConns(50)                  // 最大闲置连接数：建议小于等于 MaxOpenConns，避免闲置连接过多
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // 连接最大存活时间：超过后自动关闭（避免长期占用端口）
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // 连接最大闲置时间：闲置超此时长自动关闭（释放端口）

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
