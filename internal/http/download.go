package http

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/go-getter"
)

type SupportedExtensions string

var (
	Epub SupportedExtensions = "epub"
)

func NewGetterClient(downloadUrl *url.URL, destinationUrl *url.URL) *getter.Client {
	return &getter.Client{
		Ctx:  context.Background(),
		Dir:  false,
		Dst:  destinationUrl.Path,
		Mode: getter.ClientModeFile,
		Getters: map[string]getter.Getter{
			"http": &getter.HttpGetter{},
		},
		Src: downloadUrl.String(),
	}
}

func MakeDownloadPathFromListing(baseUrl string, details SearchListing, extension SupportedExtensions) (*url.URL, error) {
	invalidRegex := regexp.MustCompile(`[\/<>:"\\|\?\*]+`)
	folder := invalidRegex.ReplaceAllString(strings.ToValidUTF8(details.Author, ""), "")
	fileName := invalidRegex.ReplaceAllString(strings.ToValidUTF8(details.Title, ""), "")
	u := fmt.Sprintf("%s/%s - %s/%s.%s", baseUrl, folder, fileName, fileName, extension)
	finalUrl, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	return finalUrl, nil
}
