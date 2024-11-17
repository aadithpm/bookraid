package main

import (
	"aadith/libgen-search/internal/http"
	"aadith/libgen-search/internal/model"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
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
	err := godotenv.Load()
	if err != nil {
		logger.Fatal("error loading .env file")
	}
	baseUrl := os.Getenv(BaseUrlEnvKey)
	downloadUrl := os.Getenv(DownloadUrlEnvKey)
	downloadPath := os.Getenv(DownloadPathEnvKey)
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
