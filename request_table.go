package xspider

import (
	"bytes"
	"encoding/base64"
	"io"

	"github.com/xue0228/xspider/container"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type RequestTable struct {
	gorm.Model
	Url    string
	Method string `gorm:"default:'GET'"`
	Body   string `gorm:"default:''"`
	Fp     string `gorm:"uniqueIndex;not null"`
	Status uint8  `gorm:"default:0"`
}

type SqliteRequestTable struct {
	db    *gorm.DB
	table string
}

func NewSqliteRequestTable(file, table string) *SqliteRequestTable {
	db, err := gorm.Open(sqlite.Open(file), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	err = db.Table(table).AutoMigrate(&RequestTable{})
	if err != nil {
		panic(err)
	}
	return &SqliteRequestTable{db: db, table: table}
}

func (t *SqliteRequestTable) Add(request *Request) (uint, error) {
	rt := request.ToRequestTable()
	result := t.db.Table(t.table).Create(rt)
	if result.Error != nil {
		return 0, result.Error
	}
	return rt.ID, nil
}

func (t *SqliteRequestTable) Done(request *Request) (uint, error) {
	id, err := container.Get[uint](request.Ctx, "id")
	if err != nil {
		return 0, err
	}
	result := t.db.Table(t.table).Where("id = ?", id).Update("status", 2)
	if result.Error != nil {
		return id, result.Error
	}
	return id, nil
}

func (t *SqliteRequestTable) Drop(request *Request) (uint, error) {
	id, err := container.Get[uint](request.Ctx, "id")
	if err != nil {
		return 0, err
	}
	result := t.db.Table(t.table).Where("id = ?", id).Update("status", 3)
	if result.Error != nil {
		return id, result.Error
	}
	return id, nil
}

func (t *SqliteRequestTable) Pop() (*Request, error) {
	var rt RequestTable
	result := t.db.Table(t.table).Where("status = ?", 0).First(&rt)
	if result.Error != nil {
		return nil, result.Error
	}
	result = t.db.Table(t.table).Where("id = ?", rt.ID).Update("status", 1)
	if result.Error != nil {
		return nil, result.Error
	}
	ctx := container.NewSyncJsonMap()
	container.Set(ctx, "id", rt.ID)
	container.Set(ctx, "table", t.table)
	var body io.Reader
	if rt.Body != "" {
		data, err := base64.StdEncoding.DecodeString(rt.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(data)
	}
	return NewRequest(rt.Url, WithMethod(rt.Method), WithBody(body), WithCtx(ctx)), nil
}

func (t *SqliteRequestTable) SetStatus(old, new uint8) error {
	result := t.db.Table(t.table).Where("status = ?", old).Update("status", new)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (t *SqliteRequestTable) Generator() Results {
	c := make(chan any)
	go func() {
		defer close(c)
		_ = t.SetStatus(1, 0)
		for {
			request, err := t.Pop()
			if err != nil {
				return
			}
			c <- request
		}
	}()
	return c
}
