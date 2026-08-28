package main

import (
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
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
		// 背景色与页面 Mica 底色一致，避免窗口缩放时闪出白边
		BackgroundColour: &options.RGBA{R: 242, G: 243, B: 246, A: 255},
		OnStartup:         app.startup,
		OnDomReady:        app.domReady,
		OnBeforeClose:     app.beforeClose,
		Bind:              []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
