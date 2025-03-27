package source

import (
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
)

type TorrentSource struct {
	BaseURL string
}

type torrentioFindResponse struct {
	Streams []Stream `json:"streams"`
}

type Stream struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	InfoHash string `json:"infoHash"`
	FileIdx  int    `json:"fileIdx"`
}

func (api TorrentSource) Find(kind, imdbID string) ([]Stream, error) {
	var res torrentioFindResponse

	url := api.BaseURL + "/stream/" + kind + "/" + imdbID + ".json"
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
		// TODO log
		return nil, err
	}
	return res.Streams, nil
}
