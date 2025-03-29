package source

import (
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/igorcafe/anyflix/config"
)

type Source struct {
	BaseURL string
}

type FindSourceResponse struct {
	Streams []Stream `json:"streams"`
}

type Stream struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	InfoHash string `json:"infoHash"`
	FileIdx  int    `json:"fileIdx"`
}

func (api Source) Find(kind, imdbID string) ([]Stream, error) {
	var res FindSourceResponse

	url := ManifestToBaseURL(api.BaseURL) + "/stream/" + kind + "/" + imdbID + ".json"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	ua := "Mozilla/5.0 (X11; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0"
	req.Header.Set("User-Agent", ua)

	slog.Info("TorrentSource.Find", "url", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// TODO
		return nil, errors.New(resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res.Streams, nil
}

type SourceMux struct {
	Addons []config.Addon
}

// TODO: concurrency
func (mux SourceMux) Find(kind, imdbID string) ([]Stream, error) {
	var streams []Stream

	for _, addon := range mux.Addons {
		url := addon.Manifest
		torrentSrc := Source{BaseURL: url}
		_streams, err := torrentSrc.Find(kind, imdbID)
		if err != nil {
			return nil, err
		}

		streams = append(streams, _streams...)
	}

	return streams, nil
}

func ManifestToBaseURL(manifest string) string {
	return strings.TrimSuffix(manifest, "/"+path.Base(manifest))
}
