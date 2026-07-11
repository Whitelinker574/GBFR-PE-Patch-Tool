package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"strconv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 {
		if err := runWrightstoneCLI(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	app := NewApp()
	sigilGen := NewSigilGen()
	wrightstoneGen := NewWrightstoneGen()

	err := wails.Run(&options.App{
		Title:     "GBFR PE Patch Tool",
		Width:     800,
		Height:    600,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			sigilGen.startup(ctx)
			wrightstoneGen.startup(ctx)
		},
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.shutdown,
		Bind: []interface{}{
			app,
			sigilGen,
			wrightstoneGen,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func runWrightstoneCLI(args []string) error {
	values, err := parseWrightstoneCLIArgs(args)
	if err != nil {
		return err
	}

	quantity := 1
	if raw := values["quantity"]; raw != "" {
		quantity, err = strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("--quantity 必须是数字: %w", err)
		}
	}

	firstLevel, err := requiredIntArg(values, "first-level")
	if err != nil {
		return err
	}
	secondLevel, err := requiredIntArg(values, "second-level")
	if err != nil {
		return err
	}
	thirdLevel, err := requiredIntArg(values, "third-level")
	if err != nil {
		return err
	}

	wg := NewWrightstoneGen()
	if _, err := wg.LoadSaveFile(values["input"]); err != nil {
		return err
	}
	result, err := wg.ApplyItems([]WrightstoneQueueItem{{
		WrightstoneID: values["wrightstone"],
		FirstTraitID:  values["first-trait"],
		FirstLevel:    firstLevel,
		SecondTraitID: values["second-trait"],
		SecondLevel:   secondLevel,
		ThirdTraitID:  values["third-trait"],
		ThirdLevel:    thirdLevel,
		Quantity:      quantity,
	}}, values["output"])
	if err != nil {
		return err
	}

	fmt.Printf("Created %d Wrightstone(s).\n", result.CreatedCount)
	fmt.Printf("Output written: %s\n", result.OutputPath)
	fmt.Printf("Verified %d Wrightstone(s).\n", result.VerifiedCount)
	return nil
}

func parseWrightstoneCLIArgs(args []string) (map[string]string, error) {
	values := map[string]string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) < 3 || arg[:2] != "--" {
			return nil, fmt.Errorf("未知参数: %s", arg)
		}
		key := arg[2:]
		if i+1 >= len(args) || len(args[i+1]) >= 2 && args[i+1][:2] == "--" {
			return nil, fmt.Errorf("参数 --%s 缺少值", key)
		}
		values[key] = args[i+1]
		i++
	}

	for _, key := range []string{
		"input", "output", "wrightstone",
		"first-trait", "first-level",
		"second-trait", "second-level",
		"third-trait", "third-level",
	} {
		if values[key] == "" {
			return nil, fmt.Errorf("缺少参数 --%s", key)
		}
	}
	return values, nil
}

func requiredIntArg(values map[string]string, key string) (int, error) {
	value, err := strconv.Atoi(values[key])
	if err != nil {
		return 0, fmt.Errorf("--%s 必须是数字: %w", key, err)
	}
	return value, nil
}
