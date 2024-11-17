package http

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/bubbles/table"
)

type Language string

// supported languages
const (
	LangEnglish Language = "English"
)

type Format string

// supported formats
const (
	FormatEpub Format = "epub"
)

var IsbnSearchUrl = "https://isbnsearch.org/isbn/"

type LibgenScrapper struct {
	baseUrl         *url.URL
	downloadBaseUrl *url.URL
	downloadPath    *url.URL
	language        Language
	format          Format
}

type SearchListing struct {
	Page        int
	Author      string
	Title       string
	Size        string
	CatalogId   string
	IsbnUrl     *url.URL
	DownloadUrl *url.URL
}

// GetSearchListingColumns gets tea's []table.Column for []SearchListing
func GetSearchListingColumns() []table.Column {
	return []table.Column{
		{Title: "Author", Width: 20},
		{Title: "Title", Width: 75},
		{Title: "Size", Width: 25},
	}
}

// GetSearchListingTableRows converts []SearchListing to []table.Row for tea
func GetSearchListingTableRows(rows []SearchListing) []table.Row {
	var stringRows []table.Row
	for _, v := range rows {
		entry := []string{v.Author, v.Title, v.Size}
		stringRows = append(stringRows, entry)
	}
	return stringRows
}

// GetSearchListingByTableRow returns the search listing for a selected table.Row because tea only allows pure string values for a table.
// Given a string selection, we need to find the right value in our slice of objects
func GetSearchListingByTableRow(row table.Row, dataset *[]SearchListing) (*SearchListing, error) {
	// table.Row{Author, Title, Size}
	if len(row) < 3 {
		return nil, errors.New("invalid table rows")
	}
	for _, v := range *dataset {
		if v.Author == row[0] && v.Title == row[1] && v.Size == row[2] {
			return &v, nil
		}
	}
	return nil, errors.New("search listing selection invalid")

}

func NewScrapper(baseUrl string, downloadBaseUrl string, downloadPath string) (*LibgenScrapper, error) {
	if baseUrl == "" {
		return nil, errors.New("base url is empty")
	}
	if downloadBaseUrl == "" {
		return nil, errors.New("download base url is empty")
	}
	if downloadPath == "" {
		return nil, errors.New("download path is empty")
	}
	d, err := url.Parse(downloadBaseUrl)
	if err != nil {
		return nil, fmt.Errorf("error reading download base url: %s", err)
	}
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("error reading base url: %s", err)
	}
	f, err := url.ParseRequestURI(downloadPath)
	if err != nil {
		return nil, fmt.Errorf("error reading download path to save file: %s", err)
	}
	return &LibgenScrapper{
		baseUrl:         u,
		downloadBaseUrl: d,
		downloadPath:    f,
		language:        LangEnglish,
		format:          FormatEpub,
	}, nil
}

func (l *LibgenScrapper) GetDownloadPath() string {
	return l.downloadPath.Path
}

// SearchByTerm returns a SearchListing by looking up a search term at the LibgenScrapper's baseUrl.
// The parsing logic grabs the author and manually traverses the DOM from that point
func (l *LibgenScrapper) SearchByTerm(term string, pageNumber int) ([]SearchListing, error) {
	catalogIdRegex := regexp.MustCompile(`\/fiction\/(?P<catalogId>[\dA-Z]*)`)
	isbnRegex := regexp.MustCompile(`ISBN: (?P<isbn>\d*)`)
	var results []SearchListing
	addToResults := func(s SearchListing) {
		results = append(results, s)
	}
	if term == "" {
		return nil, errors.New("search term is empty")
	}
	term = url.QueryEscape(strings.TrimSpace(term))
	baseLibUrl := fmt.Sprintf("%s?q=%s&criteria=&language=%s&format=%s&page=%s",
		l.baseUrl.String(), term, l.language,
		l.format, strconv.Itoa(pageNumber+1),
	)
	res, err := http.Get(baseLibUrl)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}
	doc.Find("ul[class=catalog_authors]").Each(func(i int, s *goquery.Selection) {
		entry := SearchListing{}
		entry.Author = strings.TrimSpace(s.Text())

		titleNode := s.Parent().Siblings().Eq(1).Find("a")
		entry.Title = strings.TrimSpace(titleNode.Text())
		href, exists := s.Parent().Siblings().Eq(1).Find("a").Attr("href")
		if !exists {
			return
		}
		catalogMatches := catalogIdRegex.FindStringSubmatch(href)
		catalogIdIndex := catalogIdRegex.SubexpIndex("catalogId")
		if catalogIdIndex == -1 || len(catalogMatches) < 2 {
			return
		}
		entry.CatalogId = catalogMatches[catalogIdIndex]

		isbnMatches := isbnRegex.FindStringSubmatch(
			strings.TrimSpace(
				titleNode.Parent().Siblings().First().Text(),
			),
		)
		isbnIndex := isbnRegex.SubexpIndex("isbn")
		if isbnIndex != -1 && len(isbnMatches) > 1 {
			isbnText := isbnMatches[isbnIndex]
			if isbnText != "" {
				u, err := url.Parse(fmt.Sprintf("%s%s", IsbnSearchUrl, isbnText))
				if err == nil {
					entry.IsbnUrl = u
				}
			}
		}

		sizeNode := s.Parent().Siblings().Eq(3).Text()
		sizeNodeData := strings.Split(sizeNode, "/")
		if len(sizeNodeData) > 1 {
			entry.Size = strings.TrimSpace(sizeNodeData[1])
		}
		addToResults(entry)
	})
	return results, nil
}

func (l *LibgenScrapper) DownloadFromTag(id string) (*url.URL, error) {
	baseDownloadListingUrl := fmt.Sprintf("%s/%s", l.downloadBaseUrl.String(), id)
	res, err := http.Get(baseDownloadListingUrl)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}
	downloadNode := doc.Find("div[id=download]").First()
	href, exists := downloadNode.Find("h2").First().Find("a").First().Attr("href")
	if !exists {
		return nil, fmt.Errorf("download link not found in scrapper")
	}
	d, err := url.Parse(href)
	d.Scheme = "http"
	if err != nil {
		return nil, fmt.Errorf("error reading download url: %s", err)
	}
	return d, nil
}
