package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/igorcafe/anyflix/config"
	"github.com/igorcafe/anyflix/httpx"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	metaAPI := meta.DefaultAPI()
	opensubtitles := opensubs.DefaultAPI()

	torrentSource := source.SourceMux{
		Addons: cfg.Addons,
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
	_ = cacheDir

	routesMux := http.NewServeMux()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		wx := &httpx.ResponseWriter{ResponseWriter: w}
		w = wx
		slog.Debug(fmt.Sprintf("[INC] - %s %s", r.Method, r.URL.Path))
		routesMux.ServeHTTP(w, r)
		slog.Debug(fmt.Sprintf("[%d] - %s %s", wx.Status(), r.Method, r.URL.Path))
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

	routesMux.HandleFunc("GET /api/meta/{type}/details/{id}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		if kind != "movie" && kind != "series" {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Msg:    "invalid content type " + kind,
				Status: http.StatusBadRequest,
			})
			return
		}

		id := r.PathValue("id")

		res, err := metaAPI.Get(kind, id)
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err: err,
				Msg: "find metadata",
			})
			return
		}

		httpx.JSON(w, res)
	})

	routesMux.HandleFunc("GET /api/meta/{type}/search/{query}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		if kind != "movie" && kind != "series" {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Msg:    "invalid content type " + kind,
				Status: http.StatusBadRequest,
			})
			return
		}

		query := r.PathValue("query")

		res, err := metaAPI.Search(kind, query)
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err: err,
				Msg: "find metadata",
			})
			return
		}

		httpx.JSON(w, res)
	})

	routesMux.HandleFunc("GET /api/streams/{type}/{imdbID}", func(w http.ResponseWriter, r *http.Request) {
		imdbID := r.PathValue("imdbID") // TODO
		kind := r.PathValue("type")     // TODO

		streams, err := torrentSource.Find(kind, imdbID)
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err: err,
				Msg: "find streams",
			})
			return
		}

		httpx.JSON(w, streams)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/stream", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err:    err,
				Msg:    "invalid fileIdx",
				Status: http.StatusBadRequest,
			})
			return
		}
		torrentService.StreamFileHTTP(w, r, infoHash, fileIdx)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/download", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err:    err,
				Msg:    "invalid fileIdx",
				Status: http.StatusBadRequest,
			})
			return
		}
		torrentService.DownloadFile(infoHash, fileIdx)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/stat", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err:    err,
				Msg:    "invalid fileIdx",
				Status: http.StatusBadRequest,
			})
			return
		}

		stat := torrentService.Stat(infoHash, fileIdx)

		httpx.JSON(w, stat)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/drop", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		torrentService.Drop(infoHash)
	})

	routesMux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/hash", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err:    err,
				Msg:    "invalid fileIdx",
				Status: http.StatusBadRequest,
			})
			return
		}

		hash, err := torrentService.FileHash(infoHash, fileIdx)
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err: err,
				Msg: "get file hash",
			})
			return
		}

		res := map[string]string{
			"hash": hash,
		}

		httpx.JSON(w, res)
	})

	routesMux.HandleFunc("GET /api/opensubs/{type}/{imdbID}/{fileHash}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		imdbID := r.PathValue("imdbID")
		fileHash := r.PathValue("fileHash")

		subs, err := opensubtitles.Search(kind, imdbID, fileHash)
		if err != nil {
			httpx.ErrorJSON(w, httpx.ErrorJSONParams{
				Err: err,
				Msg: "search subtitles",
			})
			return
		}

		httpx.JSON(w, subs)
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
