package main

import (
	"log"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()
	defaultSheet := f.GetSheetName(0)
	f.DeleteSheet(defaultSheet)

	addTemplateSheet(f, "101", [][]any{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"月份", "电起", "耗电度数", "电费", "水起", "数量", "水费", "额外", "总计金额", "已收", "剩余应收", "房间号", 101, "张三"},
		{"初始值", 100, 0, 0, 10, 0, 0, 0, 0, 0, 0, "房租", 1000, "2026-01-01"},
		{"电话:", "", "", "", "", "", "", "", "", "", "", "收租方式", "月度"},
		{"身份证", "440101199001011234"},
		{1, 100, 0, 10, 10, 0, 0, 0, 1010, 0, 1010},
		{2, 120, 20, 24, 15, 5, 27.5, 0, 1051.5, 500, 551.5},
	})

	addTemplateSheet(f, "201", [][]any{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		{"月份", "电起", "耗电度数", "电费", "水起", "数量", "水费", "额外", "总计金额", "已收", "剩余应收", "房间号", 201, "李四"},
		{"初始值", 300, 0, 0, 50, 0, 0, 0, 0, 0, 0, "房租", 1500, ""},
		{"电话:", "13800138000", "", "", "", "", "", "", "", "", "", "收租方式", "季度"},
		{"身份证", "440101199202021234"},
		{1, 320, 20, 24, 55, 5, 27.5, 100, 4651.5, 0, 4651.5},
	})

	addTemplateSheet(f, "总表", [][]any{
		{"总表"},
	})

	output := filepath.Join("..", "import_template.xlsx")
	if err := f.SaveAs(output); err != nil {
		log.Fatalf("save template failed: %v", err)
	}
	log.Printf("template generated: %s", output)
}

func addTemplateSheet(f *excelize.File, sheetName string, rows [][]any) {
	f.NewSheet(sheetName)
	for i, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			log.Fatalf("cell name failed: %v", err)
		}
		if err := f.SetSheetRow(sheetName, cell, &row); err != nil {
			log.Fatalf("set row failed: %v", err)
		}
	}
}
