package xspider

import (
	"fmt"
	"reflect"
	"sort"

	"go.uber.org/zap"
)

type ItemPipeliner interface {
	GetModuleName() string
	ProcessItem(Item, *Spider) Item
	OpenSpider(*Spider)
	CloseSpider(*Spider)
}

type BaseItemPipeline struct {
	ModuleName string
	Logger     *zap.SugaredLogger
	Stats      StatsCollector
}

func (i *BaseItemPipeline) GetModuleName() string {
	return i.ModuleName
}

func (i *BaseItemPipeline) FromSpider(spider *Spider) {
	i.Logger = spider.Log.With("module_name", i.ModuleName)
	i.Stats = spider.Stats
}

func (i *BaseItemPipeline) ProcessItem(item *Item, spider *Spider) {}

func (i *BaseItemPipeline) OpenSpider(spider *Spider) {}

func (i *BaseItemPipeline) CloseSpider(spider *Spider) {}

type ProcessItemFunc func(Item, *Spider) Item
type OpenSpiderFunc func(*Spider)
type CloseSpiderFunc func(*Spider)

type ItemPipelineManager struct {
	ModuleName    string
	logger        *zap.SugaredLogger
	stats         StatsCollector
	itemPipelines []ItemPipeliner
	process       []ProcessItemFunc
	open          []OpenSpiderFunc
	close         []CloseSpiderFunc
	ch            chan *Signal
}

func (i *ItemPipelineManager) FromSpider(spider *Spider) {
	i.ModuleName = "item_pipeline_manager"
	i.logger = spider.Log.With("module_name", i.ModuleName)
	i.stats = spider.Stats
	i.ch = spider.signalChan

	// 合并两个map
	tem := map[int]ItemPipeliner{}
	for k, v := range spider.Settings.ItemPipelinesBase {
		tem[k] = v
	}
	for k, v := range spider.Settings.ItemPipelines {
		tem[k] = v
	}
	// 升序排列
	tem2 := []int{}
	for k := range tem {
		tem2 = append(tem2, k)
	}
	sort.Ints(tem2)
	// 按顺序添加到切片
	for _, v := range tem2 {
		i.itemPipelines = append(i.itemPipelines, CopyNew(tem[v]).(ItemPipeliner))
	}

	for _, v := range i.itemPipelines {
		i.open = append(i.open, v.OpenSpider)
		i.process = append(i.process, v.ProcessItem)
		i.close = append(i.close, v.CloseSpider)
	}
}

func (i *ItemPipelineManager) OpenSpider(spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			i.logger.Fatalw("OpenSpider方法出错",
				"panic_module", i.itemPipelines[index].GetModuleName(),
				"error", err)
		}
	}()

	for index = 0; index < len(i.itemPipelines); index++ {
		i.open[index](spider)
	}
	return
}

func (i *ItemPipelineManager) CloseSpider(spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			i.logger.Fatalw("CloseSpider方法出错",
				"panic_module", i.itemPipelines[index].GetModuleName(),
				"error", err)
		}
	}()

	for index = 0; index < len(i.itemPipelines); index++ {
		i.close[index](spider)
	}
	return
}

func (i *ItemPipelineManager) ProcessItem(item Item, spider *Spider) (index int) {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case *ErrDropItem:
				i.stats.IncValue(fmt.Sprintf("item_pipeline/drop_item_count/%s", err), 1, 0)
				i.stats.IncValue("item_pipeline/drop_item_count/total", 1, 0)
				i.logger.Debugw("ProcessItem方法中出现DropItem错误",
					"panic_module", i.itemPipelines[index].GetModuleName(),
					"reason", err, "item", item)
			default:
				i.logger.Fatalw("ProcessItem方法出错",
					"panic_module", i.itemPipelines[index].GetModuleName(),
					"error", err)
			}
		}
	}()

	var res Item
	for index = 0; index < len(i.itemPipelines); index++ {
		res = i.process[index](item, spider)
		if res == nil {
			i.logger.Fatalw("ProcessItem方法返回错误数据类型",
				"panic_module", i.itemPipelines[index].GetModuleName(),
				"return_type", reflect.TypeOf(res))
		} else {
			item = res
		}
	}
	return
}
