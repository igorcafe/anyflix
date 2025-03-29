package opensubs

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
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

func Download(dir string, subs ...Sub) ([]string, error) {
	paths := []string{}

	for _, sub := range subs {
		func() {
			filePath := filepath.Join(dir, path.Base(sub.URL))
			_, err := os.Stat(filePath)
			if err == nil {
				slog.Info("subtitle already downloaded", "url", sub.URL)
				paths = append(paths, filePath)
				return
			}

			resp, err := http.Get(sub.URL)
			if err != nil {
				slog.Error("failed to download subtitle", "url", sub.URL, "err", err)
				return
			}
			defer resp.Body.Close()

			f, err := os.Create(filePath)
			if err != nil {
				slog.Error("failed to download subtitle", "url", sub.URL, "err", err)
				return
			}
			defer f.Close()

			_, err = io.Copy(f, resp.Body)
			if err != nil {
				slog.Error("failed to download subtitle", "url", sub.URL, "err", err)
				return
			}

			slog.Debug("saved subtitle", "filePath", filePath, "url", sub.URL)
			paths = append(paths, filePath)
		}()
	}

	return paths, nil
}
