package xspider

import (
	"errors"
	"reflect"
	"sync"

	"github.com/xue0228/xspider/container"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func init() {
	RegisterSpiderModuler(&OrmItemPipeline{})
}

type OrmItemPipeline struct {
	BaseSpiderModule
	Db *gorm.DB
}

func (ipl *OrmItemPipeline) FromSpider(spider *Spider) {
	InitBaseSpiderModule(&ipl.BaseSpiderModule, spider, ipl.Name())
	file, err := container.Get[string](spider.Settings, "SQLITE_FILE")
	if err != nil {
		ipl.Logger.Warn("未检索到SQLITE_FILE配置项")
		ipl.Db = nil
		return
	}
	db, err := gorm.Open(sqlite.Open(file), &gorm.Config{})
	if err != nil {
		ipl.Logger.Fatalw("打开数据库失败",
			"error", err)
	}
	ipl.Db = db
}

func (ipl *OrmItemPipeline) Name() string {
	return "OrmItemPipeline"
}

func (ipl *OrmItemPipeline) ProcessItem(item Item, response *Response, spider *Spider) Item {
	logger := ResponseLogger(ipl.Logger, response)

	//err := AutoMigrateRelated(ipl.Db, item)
	//if err != nil {
	//	logger.Errorw("自动迁移数据库失败", "error", err)
	//	return item
	//}
	result := ipl.Db.Create(item)
	//fmt.Println(item)
	if result.Error != nil {
		logger.Errorw("插入数据失败", "error", result.Error)
		return item
	}

	id, err := container.Get[uint](response.Ctx, "id")
	if err != nil {
		logger.Debugw("获取id失败", "error", err)
		return item
	}
	table, err := container.Get[string](response.Ctx, "table")
	if err != nil {
		logger.Debugw("获取table失败", "error", err)
		return item
	}

	result = ipl.Db.Table(table).Where("id = ?", id).Update("status", 2)
	if result.Error != nil {
		logger.Debugw("更新状态失败", "error", result.Error)
		return item
	}
	return item
}

// 仅用于记录主结构体类型，避免重复迁移同一主结构体
var migratedMainStructs = struct {
	sync.RWMutex
	types map[reflect.Type]bool
}{
	types: make(map[reflect.Type]bool),
}

// AutoMigrateRelated 自动迁移主结构体及其所有关联结构体
// 只记录主结构体是否已迁移，确保关联关系（包括多对多）能正确创建
func AutoMigrateRelated(db *gorm.DB, mainStruct interface{}) error {
	// 获取主结构体类型
	mainType := getStructType(mainStruct)
	if mainType == nil {
		return nil
	}

	// 检查主结构体是否已迁移过
	migratedMainStructs.RLock()
	alreadyMigrated := migratedMainStructs.types[mainType]
	migratedMainStructs.RUnlock()

	if alreadyMigrated {
		return nil // 主结构体已迁移过，直接返回
	}

	// 收集所有相关结构体类型（包括主结构体和所有关联结构体）
	structTypes := collectRelatedStructs(mainStruct)

	// 准备迁移所需的实例
	var migrateTargets []interface{}
	for _, typ := range structTypes {
		migrateTargets = append(migrateTargets, reflect.New(typ).Interface())
	}

	// 执行迁移
	if len(migrateTargets) > 0 {
		if err := db.AutoMigrate(migrateTargets...); err != nil {
			return err
		}
	} else {
		return errors.New("未找到任何结构体")
	}

	// 标记主结构体为已迁移
	migratedMainStructs.Lock()
	migratedMainStructs.types[mainType] = true
	migratedMainStructs.Unlock()

	return nil
}

// 收集所有关联的结构体类型
func collectRelatedStructs(s interface{}) []reflect.Type {
	visited := make(map[reflect.Type]bool)
	var queue []reflect.Type

	// 获取主结构体的类型
	t := getStructType(s)
	if t == nil {
		return nil
	}

	// 初始化队列
	queue = append(queue, t)
	visited[t] = true

	// 广度优先搜索所有关联结构体
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// 检查结构体的每个字段
		for i := 0; i < current.NumField(); i++ {
			field := current.Field(i)
			fieldType := getStructTypeFromField(field)

			if fieldType != nil && !visited[fieldType] {
				visited[fieldType] = true
				queue = append(queue, fieldType)
			}
		}
	}

	// 转换为切片返回
	result := make([]reflect.Type, 0, len(visited))
	for typ := range visited {
		result = append(result, typ)
	}
	return result
}

// 获取结构体类型（处理指针和切片）
func getStructType(s interface{}) reflect.Type {
	t := reflect.TypeOf(s)

	// 解引用指针
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// 处理切片类型（如[]User）
	if t.Kind() == reflect.Slice {
		elemType := t.Elem()
		for elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct {
			return elemType
		}
		return nil
	}

	if t.Kind() == reflect.Struct {
		return t
	}
	return nil
}

// 从字段中获取结构体类型
func getStructTypeFromField(field reflect.StructField) reflect.Type {
	fieldType := field.Type

	// 处理指针字段
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	// 处理切片字段（如[]Post或多对多关系）
	if fieldType.Kind() == reflect.Slice {
		elemType := fieldType.Elem()
		for elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct {
			return elemType
		}
		return nil
	}

	// 处理普通结构体字段
	if fieldType.Kind() == reflect.Struct {
		return fieldType
	}

	return nil
}

// 使用示例（包含多对多关系）
//func exampleUsage(Db *gorm.DB) error {
//	// 定义多对多关联结构体
//	type Role struct {
//		gorm.Model
//		Name  string
//		Users []User `gorm:"many2many:user_roles;"`
//	}
//
//	type User struct {
//		gorm.Model
//		Name  string
//		Roles []Role `gorm:"many2many:user_roles;"`
//		Posts []Post
//	}
//
//	type Post struct {
//		gorm.Model
//		Title  string
//		UserID uint
//	}
//
//	// 迁移User及其所有关联（包括多对多的Role和中间表user_roles）
//	return AutoMigrateRelated(Db, &User{})
//}
