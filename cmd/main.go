package main

import (
	"aadith/libgen-search/internal/http"
	"aadith/libgen-search/internal/model"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
)

const (
	BaseUrlEnvKey      = "BASE_URL"
	DownloadUrlEnvKey  = "BASE_DL_URL"
	DownloadPathEnvKey = "DL_PATH"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()
	baseUrl := "https://libgen.is/fiction"
	downloadUrl := "http://library.lol/fiction"
	downloadPath := filepath.FromSlash("G:/My Drive/Books/Novels")
	scrapper, err := http.NewScrapper(baseUrl, downloadUrl, downloadPath)
	if err != nil {
		sugar.Fatal(err)
	}
	um := model.NewUiModel(*scrapper, logger)
	p := tea.NewProgram(um)
	if _, err := p.Run(); err != nil {
		sugar.Fatal(err)
	}
}
