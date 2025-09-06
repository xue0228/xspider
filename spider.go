package xspider

import (
	"fmt"
	"xspider/container"

	"go.uber.org/zap"
)

type Spider struct {
	Name             string
	Signal           SignalManager
	Logger           *zap.SugaredLogger
	Stats            Statser
	Settings         container.Dict
	Starts           Results
	DefaultParseFunc string

	scheduler         Scheduler
	downloader        Downloader
	spiderManager     SpiderMiddlewareManager
	downloaderManager DownloaderMiddlewareManager
	itemManager       ItemPipelineManager
	extensionManager  ExtensionManager
	engine            Enginer
}

func NewSpider(settings container.Dict, starts Results, defaultParseFunc string) *Spider {
	return &Spider{
		Settings:         settings,
		Starts:           starts,
		DefaultParseFunc: defaultParseFunc,
	}
}

// 初始化
func (s *Spider) init() {
	// 设置爬虫机器人名称
	s.Name = s.Settings.GetStringWithDefault("BOT_NAME", "xbot")

	// 初始化日志
	if logEnabled := s.Settings.GetBoolWithDefault("LOG_ENABLED", true); logEnabled {
		logFile := s.Settings.GetStringWithDefault("LOG_FILE", fmt.Sprintf("%s_info.log", s.Name))
		errFile := s.Settings.GetStringWithDefault("LOG_ERR_FILE", fmt.Sprintf("%s_err.log", s.Name))
		logLevel := s.Settings.GetStringWithDefault("LOG_LEVEL", "debug")
		level, err := StringToLevel(logLevel)
		if err != nil {
			panic(fmt.Sprintf("log level error: %s", err))
		}
		s.Logger = NewLog(logFile, errFile, level)
	} else {
		s.Logger = zap.NewNop().Sugar()
	}
	s.Logger = s.Logger.With("bot_name", s.Name)

	// 初始化统计收集器
	statserName := s.Settings.GetStringWithDefault("STATS_STRUCT", "StatserImpl")
	s.Stats = GetAndAssertComponent[Statser](statserName)
	s.Stats.FromSpider(s)

	// 初始化信号管理器
	signalManagerName := s.Settings.GetStringWithDefault("SIGNAL_MANAGER_STRUCT", "SignalManagerImpl")
	s.Signal = GetAndAssertComponent[SignalManager](signalManagerName)
	s.Signal.FromSpider(s)

	// 初始化调度器
	schedulerName := s.Settings.GetStringWithDefault("SCHEDULER_STRUCT", "SchedulerImpl")
	s.scheduler = GetAndAssertComponent[Scheduler](schedulerName)
	s.scheduler.FromSpider(s)

	// 初始化下载器
	downloaderName := s.Settings.GetStringWithDefault("DOWNLOADER_STRUCT", "DownloaderImpl")
	s.downloader = GetAndAssertComponent[Downloader](downloaderName)
	s.downloader.FromSpider(s)

	// 初始化中间件
	spiderManagerName := s.Settings.GetStringWithDefault("SPIDER_MIDDLEWARE_MANAGER_STRUCT", "SpiderMiddlewareManagerImpl")
	s.spiderManager = GetAndAssertComponent[SpiderMiddlewareManager](spiderManagerName)
	s.spiderManager.FromSpider(s)
	downloaderManagerName := s.Settings.GetStringWithDefault("DOWNLOADER_MIDDLEWARE_MANAGER_STRUCT", "DownloaderMiddlewareManagerImpl")
	s.downloaderManager = GetAndAssertComponent[DownloaderMiddlewareManager](downloaderManagerName)
	s.downloaderManager.FromSpider(s)
	itemManagerName := s.Settings.GetStringWithDefault("ITEM_PIPELINE_MANAGER_STRUCT", "ItemPipelineManagerImpl")
	s.itemManager = GetAndAssertComponent[ItemPipelineManager](itemManagerName)
	s.itemManager.FromSpider(s)

	// 初始化扩展
	extensionManagerName := s.Settings.GetStringWithDefault("EXTENSION_MANAGER_STRUCT", "ExtensionManagerImpl")
	s.extensionManager = GetAndAssertComponent[ExtensionManager](extensionManagerName)
	s.extensionManager.FromSpider(s)

	// 初始化引擎
	engineName := s.Settings.GetStringWithDefault("ENGINE_STRUCT", "EnginerImpl")
	s.engine = GetAndAssertComponent[Enginer](engineName)
	s.engine.FromSpider(s)
}

func (s *Spider) Start() {
	s.init()
	s.Signal.Start()
	s.engine.Start(s)
	defer func() {
		for {
			if s.Signal.IsAllDone() {
				stats := s.Stats.GetStats()
				s.Close()
				s.Logger.Infow("统计信息", "stats", stats)
				break
			}
		}
	}()
}

func (s *Spider) Close() {
	//s.engine.Close(s)
	s.extensionManager.Close(s)
	s.itemManager.Close(s)
	s.downloaderManager.Close(s)
	s.spiderManager.Close(s)
	s.downloader.Close(s)
	s.scheduler.Close(s)
	s.Signal.Close(s)
	s.Stats.Close(s)
}
