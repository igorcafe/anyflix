package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/igorcafe/anyflix/meta"
	"github.com/igorcafe/anyflix/opensubs"
	"github.com/igorcafe/anyflix/source"
	"github.com/igorcafe/anyflix/torrent"
)

//go:embed www/*
var www embed.FS

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	slog.Info("loading config")
	config, err := ConfigLoad()
	if err != nil {
		log.Fatal(err)
	}

	metaAPI := meta.DefaultAPI()
	opensubtitles := opensubs.DefaultAPI()

	addon := config.Addons[0]

	torrentSource := source.TorrentSource{
		BaseURL: addon.BaseURL(),
	}

	slog.Info("starting torrent service")
	torrentService, err := torrent.DefaultService()
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("started torrent service")

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatal(err)
	}

	routesMux := http.NewServeMux()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		slog.Debug(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		routesMux.ServeHTTP(w, r)
	})

	www, err := fs.Sub(www, "www")
	if err != nil {
		log.Fatal(err)
	}

	host := "localhost"
	port := 2025
	baseURL := fmt.Sprintf("http://%s:%d", host, port)

	routesMux.Handle("GET /", http.FileServerFS(www))

	// use it instead for faster developing
	// routesMux.Handle("GET /", http.FileServer(http.Dir("./www")))
	// _ = www

	routesMux.HandleFunc("GET /watch/{type}/{imdbID}/{infoHash}/{fileIdx}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		infoHash := r.PathValue("infoHash")
		imdbID := r.PathValue("imdbID")

		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err:    err,
				msg:    "invalid fileIdx",
				status: http.StatusBadRequest,
			})
			return
		}

		url := fmt.Sprintf(
			"%s/api/torrent/%s/%d/stream",
			baseURL,
			infoHash,
			fileIdx,
		)

		hash, err := torrentService.FileHash(infoHash, fileIdx)
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err: err,
				msg: "failed to get file hash",
			})
			return
		}

		subs, err := opensubtitles.Search(kind, imdbID, hash)
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err: err,
				msg: "failed to search subtitles",
			})
			return
		}

		subs = slices.DeleteFunc(subs, func(sub opensubs.Sub) bool {
			return !slices.Contains(config.SubLangs, sub.Lang)
		})

		subsDir := filepath.Join(cacheDir, "subs")
		_ = os.MkdirAll(subsDir, 0700)

		subPaths, err := downloadSubtitles(subsDir, subs)
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err: err,
				msg: "download subtitles",
			})
			return
		}

		buf := &bytes.Buffer{}
		err = template.
			Must(template.New("").Parse(config.PlayerCmd)).
			Execute(buf, map[string]any{
				"URL":  url,
				"Subs": subPaths,
			})

		args := strings.Fields(buf.String())
		slog.Info(fmt.Sprint(args))
		cmd := exec.CommandContext(r.Context(), args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	})

	routesMux.HandleFunc("GET /api/meta/{type}/details/{id}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		if kind != "movie" && kind != "series" {
			httpErrorJSON(w, httpErrorJSONParams{
				msg:    "invalid content type " + kind,
				status: http.StatusBadRequest,
			})
			return
		}

		id := r.PathValue("id")

		res, err := metaAPI.Get(kind, id)
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err: err,
				msg: "find metadata",
			})
			return
		}

		httpJSON(w, res)
	})

	routesMux.HandleFunc("GET /api/meta/{type}/search/{query}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		if kind != "movie" && kind != "series" {
			httpErrorJSON(w, httpErrorJSONParams{
				msg:    "invalid content type " + kind,
				status: http.StatusBadRequest,
			})
			return
		}

		query := r.PathValue("query")

		res, err := metaAPI.Search(kind, query)
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err: err,
				msg: "find metadata",
			})
			return
		}

		httpJSON(w, res)
	})

	routesMux.HandleFunc("GET /api/streams/{type}/{imdbID}", func(w http.ResponseWriter, r *http.Request) {
		imdbID := r.PathValue("imdbID") // TODO
		kind := r.PathValue("type")     // TODO

		streams, err := torrentSource.Find(kind, imdbID)
		if err != nil {
			slog.Error("failed to find streams", "err", err)
			return
		}

		httpJSON(w, streams)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/stream", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err:    err,
				msg:    "invalid fileIdx",
				status: http.StatusBadRequest,
			})
			return
		}
		torrentService.StreamFileHTTP(w, r, infoHash, fileIdx)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/download", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err:    err,
				msg:    "invalid fileIdx",
				status: http.StatusBadRequest,
			})
			return
		}
		torrentService.DownloadFile(infoHash, fileIdx)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/stat", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err:    err,
				msg:    "invalid fileIdx",
				status: http.StatusBadRequest,
			})
			return
		}

		stat := torrentService.Stat(infoHash, fileIdx)

		httpJSON(w, stat)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/drop", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		torrentService.Drop(infoHash)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/hash", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err:    err,
				msg:    "invalid fileIdx",
				status: http.StatusBadRequest,
			})
			return
		}

		hash, err := torrentService.FileHash(infoHash, fileIdx)
		if err != nil {
			httpErrorJSON(w, httpErrorJSONParams{
				err: err,
				msg: "get file hash",
			})
			return
		}

		res := map[string]string{
			"hash": hash,
		}

		httpJSON(w, res)
	})

	routesMux.HandleFunc("GET /api/opensubs/{type}/{imdbID}/{fileHash}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		imdbID := r.PathValue("imdbID")
		fileHash := r.PathValue("fileHash")

		subs, err := opensubtitles.Search(kind, imdbID, fileHash)
		if err != nil {
			slog.Error("failed to search subtitles",
				"err", err)
			return
		}

		httpJSON(w, subs)
	})
	//mux.HandleFunc("GET /api/opensubs/{id}", subsService.handleFindSubByID)

	go func() {
		time.Sleep(time.Second)
		_ = exec.Command("xdg-open", baseURL).Run()
	}()

	slog.Info("starting anyflix at " + baseURL)
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), mux)
	log.Panic(err)
}

func downloadSubtitles(dir string, subs []opensubs.Sub) ([]string, error) {
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

func httpJSON(w http.ResponseWriter, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		httpErrorJSON(w, httpErrorJSONParams{
			err: err,
			msg: "failed to serialize response",
		})
		return
	}

	_, err = w.Write(b)
	if err != nil {
		slog.Error("write response", "err", err)
	}
}

type httpErrorJSONParams struct {
	err    error
	msg    string
	status int
}

func httpErrorJSON(w http.ResponseWriter, params httpErrorJSONParams) {
	var finalMsg string

	if params.err != nil {
		slog.Error(params.msg, "err", params.err)
		finalMsg = fmt.Sprintf("%v: %v", params.msg, params.err)
	} else {
		finalMsg = params.msg
	}

	b, _ := json.MarshalIndent(map[string]any{
		"params.error": true,
		"message":      finalMsg,
	}, "", "  ")

	status := params.status
	if status == 0 {
		status = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_, err := w.Write(b)
	if err != nil {
		slog.Error("write error response", "err", err)
	}
}
