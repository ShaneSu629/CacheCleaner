package main

import (
	"cachecleaner/internal/applog"
	"cachecleaner/internal/singleflight"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// wailsLogAdapter 把 Wails 框架内部日志（WebView2、绑定、资产服务等）
// 一并写入 applog 落盘日志，框架层报错不再不可见。
type wailsLogAdapter struct{}

func (wailsLogAdapter) Print(m string)   { applog.Info("wails: " + m) }
func (wailsLogAdapter) Trace(m string)   { applog.Info("wails[trace]: " + m) }
func (wailsLogAdapter) Debug(m string)   { applog.Info("wails[debug]: " + m) }
func (wailsLogAdapter) Info(m string)    { applog.Info("wails: " + m) }
func (wailsLogAdapter) Warning(m string) { applog.Info("wails[warn]: " + m) }
func (wailsLogAdapter) Error(m string)   { applog.Error("wails: " + m) }
func (wailsLogAdapter) Fatal(m string)   { applog.Error("wails[fatal]: " + m) }

func main() {
	// 单实例：重复双击时静默定位已运行的窗口，恢复并前置到前台，本进程直接退出。
	// 必须在日志初始化之前：第二个实例根本不配写日志（避免两个进程争抢日志文件）。
	if !singleflight.Acquire("CacheCleaner", "智能缓存清理工具") {
		return
	}

	// 落盘日志必须最先初始化：后续任何报错（含启动失败）才有据可查。
	logPath := applog.Init()
	defer applog.Close()
	defer applog.Recover() // panic 兜底：写入日志后再退出
	if logPath != "" {
		applog.Info("日志文件: %s", logPath)
	} else {
		println("警告: 日志文件初始化失败（exe 目录与临时目录均不可写）")
	}

	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "智能缓存清理工具",
		Width:     1140,
		Height:    760,
		MinWidth:  760,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Logger: wailsLogAdapter{},
		// 背景色与页面 Mica 底色一致，避免窗口缩放时闪出白边
		BackgroundColour: &options.RGBA{R: 242, G: 243, B: 246, A: 255},
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnBeforeClose:     app.beforeClose,
		Bind:              []interface{}{app},
	})
	if err != nil {
		applog.Error("wails.Run 失败: %v", err)
		println("Error:", err.Error())
		println("详细日志见:", logPath)
	}
}
