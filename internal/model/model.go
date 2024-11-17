package model

import (
	"aadith/libgen-search/internal/http"
	"fmt"
	"net/url"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hashicorp/go-getter"
	"go.uber.org/zap"
)

type DownloadState int32
type UiModelState int32

const (
	Start        UiModelState = 0
	Loading      UiModelState = 1
	ListView     UiModelState = 2
	Confirmation UiModelState = 3
	Downloading  UiModelState = 4
	Done         UiModelState = 5
	Kill         UiModelState = 6
)

const (
	DownloadInit     DownloadState = 0
	DownloadStarted  DownloadState = 1
	DownloadFinished DownloadState = 2
)

type UiModel struct {
	textInput   textinput.Model
	searchQuery *string
	catalogId   string
	savePath    string

	table *table.Model

	spinner spinner.Model
	state   UiModelState

	scrapper        http.LibgenScrapper
	scrappedResults *[]http.SearchListing

	downloadClient  *getter.Client
	downloadCounter int
	downloadState   DownloadState
	downloadErrors  chan (error)

	logger *zap.Logger
	err    error
}

type scrappedMsg struct{}

func NewUiModel(httpScrapper http.LibgenScrapper, logger *zap.Logger) *UiModel {
	sp := httpScrapper.GetDownloadPath()
	ti := textinput.New()
	ti.Placeholder = "Dune Frank Herbert"
	ti.Focus()
	ti.CharLimit = 256

	t := table.New(
		table.WithFocused(true),
		table.WithHeight(7),
	)
	t.Help.ShowAll = true
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)
	t.SetStyles(s)

	sn := spinner.New()
	sn.Spinner = spinner.Dot
	sn.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))

	return &UiModel{
		textInput:       ti,
		savePath:        sp,
		table:           &t,
		spinner:         sn,
		state:           Start,
		scrapper:        httpScrapper,
		downloadCounter: 0,
		downloadState:   DownloadInit,
		downloadErrors:  make(chan error),
		logger:          logger,
	}
}

func (u *UiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (u *UiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// quit on the next tick if program was done
	// always listen for exit events
	if u.state == Done {
		return u, tea.Quit
	}
	if m, ok := msg.(tea.KeyMsg); ok {
		if m.Type == tea.KeyCtrlC {
			return u, tea.Quit
		}
	}

	switch u.state {
	case Start:
		{
			switch msg := msg.(type) {
			case tea.KeyMsg:
				switch msg.Type {
				case tea.KeyEnter:
					if u.textInput.Value() == "" {
						return u, tea.Printf("search term cannot be empty")
					}
					v := u.textInput.Value()
					u.searchQuery = &v
					u.state = Loading
					return u, tea.Batch(func() tea.Msg {
						results, err := u.scrapper.SearchByTerm(*u.searchQuery, 0)
						u.scrappedResults = &results
						if err != nil {
							u.logger.Fatal(err.Error())
						}
						u.table.SetColumns(http.GetSearchListingColumns())
						u.table.SetRows(http.GetSearchListingTableRows(*u.scrappedResults))
						return scrappedMsg{}
					},
						u.spinner.Tick,
					)
				case tea.KeyEsc:
					return u, tea.Quit
				default:
					u.textInput, cmd = u.textInput.Update(msg)
					return u, cmd
				}
			}
		}
	case Loading:
		{
			switch msg := msg.(type) {
			case scrappedMsg:
				u.state = ListView
				u.spinner, _ = u.spinner.Update(nil)
				return u, cmd
			case spinner.TickMsg:
				u.spinner, cmd = u.spinner.Update(msg)
				return u, cmd
			}
		}
	case ListView:
		{
			switch msg := msg.(type) {
			case tea.KeyMsg:
				switch msg.Type {
				case tea.KeyEsc:
					u.state = Start
					return u, cmd
				case tea.KeyEnter:
					selected, err := http.GetSearchListingByTableRow(u.table.SelectedRow(), u.scrappedResults)
					if err != nil {
						u.logger.Fatal("selected book not found")
					}
					u.catalogId = selected.CatalogId
					savePath, err := http.MakeDownloadPathFromListing(u.savePath, *selected, http.Epub)
					u.savePath = savePath.String()
					if err != nil {
						u.logger.Fatal("error downloading book")
					}
					u.textInput.SetValue(u.savePath)
					u.state = Confirmation
					return u, cmd
				default:
					t, cmd := u.table.Update(msg)
					u.table = &t
					return u, cmd
				}
			}
		}
	case Confirmation:
		{
			switch msg := msg.(type) {
			case tea.KeyMsg:
				switch msg.Type {
				case tea.KeyEsc:
					u.state = ListView
					return u, cmd
				case tea.KeyEnter:
					if u.textInput.Value() == "" {
						return u, tea.Printf("download path cannot be empty")
					}
					v := u.textInput.Value()
					downloadUrl, err := u.scrapper.DownloadFromTag(u.catalogId)
					if err != nil {
						u.logger.Fatal("error downloading book")
					}
					pathUrl, err := url.Parse(v)
					if err != nil {
						u.logger.Fatal("download path is invalid")
					}
					u.downloadClient = http.NewGetterClient(downloadUrl, pathUrl)
					u.state = Downloading
					return u, u.spinner.Tick
				default:
					u.textInput, cmd = u.textInput.Update(msg)
					return u, cmd
				}
			}
		}
	case Downloading:
		{
			switch u.downloadState {
			case DownloadInit:
				go func() {
					// todo: add another channel for progress tracker here
					u.downloadErrors <- u.downloadClient.Get()
				}()
				u.downloadState = DownloadStarted
				return u, u.spinner.Tick
			case DownloadStarted:
				select {
				case err := <-u.downloadErrors:
					if err != nil {
						u.logger.Fatal("error downloading")
					}
					u.state = Done
					u.downloadState = DownloadFinished
					return u, cmd
				default:
					switch msg := msg.(type) {
					case spinner.TickMsg:
						u.spinner, cmd = u.spinner.Update(msg)
						return u, cmd
					}
				}
			}
		}
	}
	return u, nil
}

func (u *UiModel) View() string {
	if u.err != nil {
		u.logger.Fatal(u.err.Error())
	}
	switch u.state {
	case Start:
		return fmt.Sprintf(
			"%s\n%s\n", "Enter a search term for a novel (title, author or both)", u.textInput.View(),
		)
	case Loading:
		return fmt.Sprintf("%s Loading books...\n", u.spinner.View())
	case ListView:
		return u.table.View() + "\n" + u.table.HelpView()
	case Confirmation:
		return fmt.Sprintf(
			"%s\n%s\n", "Please confirm the download path", u.textInput.View(),
		)
	case Downloading:
		return fmt.Sprintf("%s Downloading %s...\n", u.spinner.View(), *u.searchQuery)
	case Done:
		return "✔️ Download done!"
	default:
		return "This is a weird state...\n"
	}
}
