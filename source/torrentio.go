package source

import (
	"encoding/json"
	"errors"
	"net/http"
)

type TorrentIOAPI struct {
	BaseURL string
}

func DefaultTorrentIOAPI() TorrentIOAPI {
	return TorrentIOAPI{
		"https://torrentio.strem.fun",
	}
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

func (api TorrentIOAPI) Find(kind, imdbID string) ([]Stream, error) {
	var res torrentioFindResponse

	resp, err := http.Get(api.BaseURL + "/stream/" + kind + "/" + imdbID + ".json")
	if err != nil {
		// TODO log
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// TODO
		return nil, errors.New("")
	}

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		// TODO log
		return nil, err
	}
	return res.Streams, nil
}
