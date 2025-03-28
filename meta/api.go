package meta

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"

	"github.com/igorcafe/anyflix/filler"
)

type API struct {
	BaseURL string
}

func DefaultAPI() API {
	return API{
		"https://v3-cinemeta.strem.io",
	}
}

type getMetaResponse struct {
	Meta Meta `json:"meta"`
}

type searchMetaResponse struct {
	Metas []Meta `json:"metas"`
}

type Meta struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	ReleaseInfo string   `json:"releaseInfo"`
	Description string   `json:"description"`
	Runtime     string   `json:"runtime"`
	IMDBRating  string   `json:"imdbRating"`
	Poster      string   `json:"poster"`
	Background  string   `json:"background"`
	Logo        string   `json:"logo"`
	Videos      []Video  `json:"videos"`
	Genre       []string `json:"genre"`
	Director    []string `json:"director"`
	Writer      []string `json:"writer"`
}

type Video struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Season int    `json:"season"`
	Number int    `json:"number"`
	Type   string `json:"type"`
}

func (s API) Get(kind, id string) (Meta, error) {
	var res getMetaResponse
	url := s.BaseURL + "/meta/" + kind + "/" + id + ".json"

	slog.Debug("meta.API.Get", "kind", kind, "id", id, "url", url)
	resp, err := http.Get(url)
	if err != nil {
		slog.Error("search meta",
			"url", url,
			"err", err)
		return Meta{}, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		slog.Error("parse meta json",
			"err", err)
		return Meta{}, err
	}

	res.Meta.Videos = slices.DeleteFunc(res.Meta.Videos, func(v Video) bool {
		return v.Season == 0
	})

	if kind == "series" {
		fillers, err := filler.SearchShow(res.Meta.Name)
		if err != nil {
			slog.Error("search fillers", "err", err)
		} else {
			for i, video := range res.Meta.Videos {
				if len(fillers.Episodes) < i+1 {
					break
				}

				video.Type = fillers.Episodes[i].Type
				res.Meta.Videos[i] = video
			}
		}
	}

	return res.Meta, nil
}

func (s API) Search(kind string, query string) ([]Meta, error) {
	var res searchMetaResponse
	url := s.BaseURL + "/catalog/" + kind + "/top/search=" + query + ".json"

	resp, err := http.Get(url)
	if err != nil {
		slog.Error("search meta",
			"url", url,
			"err", err)
		return nil, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		slog.Error("parse meta json",
			"err", err)
		return nil, err
	}

	return res.Metas, nil
}
