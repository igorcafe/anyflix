package opensubs

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type API struct {
	BaseURL string
}

func DefaultAPI() API {
	return API{
		"https://opensubtitles-v3.strem.io",
	}
}

type searchResponse struct {
	Subtitles []Sub `json:"subtitles"`
}

type Sub struct {
	URL      string `json:"url"`
	Lang     string `json:"lang"`
	Encoding string `json:"SubEncoding"`
}

func (h API) Search(kind, imdbID, fileHash string) ([]Sub, error) {
	slog.Debug("opensubsService.search", "kind", kind, "imdbID", imdbID, "fileHash", fileHash)

	var subs searchResponse
	url := h.BaseURL + "/subtitles/" + kind + "/" + imdbID + "videoHash=" + fileHash + ".json"

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, errors.New(resp.Status)
	}

	slog.Info("handleSearch", "method", "GET", "url", url, "status", resp.Status)

	err = json.NewDecoder(resp.Body).Decode(&subs)
	return subs.Subtitles, err
}
