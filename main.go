package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/igorcafe/anyflix/meta"
	"github.com/igorcafe/anyflix/opensubs"
	"github.com/igorcafe/anyflix/source"
	"github.com/igorcafe/anyflix/torrent"
)

const cmdTmpl = `mpv
{{ .StreamURL }}
{{ range .Subtitles }}
--sub-file={{ .URL }}
{{ end }}
`

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	metaAPI := meta.DefaultAPI()

	opensubtitles := opensubs.DefaultAPI()
	torrentSource := source.DefaultTorrentIOAPI()
	torrentService, err := torrent.DefaultService()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./www/home.html")
	})

	mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		res, err := metaAPI.Search(q.Get("type"), q.Get("query"))
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		tmpl := template.Must(template.ParseFiles("./www/search.html"))
		err = tmpl.Execute(w, res)
		if err != nil {
			slog.Error("", "err", err)
			return
		}
	})

	mux.HandleFunc("GET /details/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		kind := r.PathValue("type")

		metadata, err := metaAPI.Get(kind, id)
		if err != nil {
			slog.Error("", "err", err)
			return
		}

		tmpl := template.Must(template.ParseFiles("./www/details.html"))

		err = tmpl.Execute(w, struct {
			ID   string
			Meta meta.Meta
		}{
			ID:   id,
			Meta: metadata,
		})
		if err != nil {
			slog.Error("", "err", err)
			return
		}
	})

	mux.HandleFunc("GET /streams/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		kind := r.PathValue("type")
		tmpl := template.Must(template.ParseFiles("./www/streams.html"))

		streams, err := torrentSource.Find(kind, id)
		err = tmpl.Execute(w, struct {
			ID      string
			Kind    string
			Streams []source.Stream
		}{
			ID:      id,
			Kind:    kind,
			Streams: streams,
		})
		if err != nil {
			slog.Error("", "err", err)
			return
		}
	})

	mux.HandleFunc("GET /watch/{type}/{imdbID}/{infoHash}/{fileIdx}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		infoHash := r.PathValue("infoHash")
		imdbID := r.PathValue("imdbID")

		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			http.Error(w, "invalid fileIdx", http.StatusBadRequest)
			return
		}

		url := fmt.Sprintf(
			"http://localhost:3000/api/torrent/%s/%d/stream",
			infoHash,
			fileIdx,
		)

		hash, err := torrentService.FileHash(infoHash, fileIdx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		subs, err := opensubtitles.Search(kind, imdbID, hash)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("subs: %#+v", subs)

		buf := &bytes.Buffer{}
		err = template.
			Must(template.New("").Parse(cmdTmpl)).
			Execute(buf, struct {
				Subtitles []opensubs.Subtitile
				StreamURL string
			}{
				StreamURL: url,
				Subtitles: nil,
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

	mux.HandleFunc("GET /api/meta/{type}/search/{query}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		if kind != "movie" && kind != "series" {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		query := r.PathValue("query")

		res, err := metaAPI.Search(kind, query)
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(res)
		if err != nil {
			slog.Error("parse meta to json",
				"err", err)
			return
		}
	})

	mux.HandleFunc("GET /api/streams/{type}/{imdbID}", func(w http.ResponseWriter, r *http.Request) {
		imdbID := r.PathValue("imdbID") // TODO
		kind := r.PathValue("type")     // TODO

		streams, err := torrentSource.Find(kind, imdbID)
		if err != nil {
			panic(err)
		}

		err = json.NewEncoder(w).Encode(streams) // TODO
	})

	mux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/stream", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			// TODO
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		torrentService.StreamFileHTTP(w, r, infoHash, fileIdx)
	})

	mux.HandleFunc("GET /api/torrent/{infoHash}/{fileIdx}/hash", func(w http.ResponseWriter, r *http.Request) {
		infoHash := r.PathValue("infoHash")
		fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
		if err != nil {
			http.Error(w, "invalid fileIdx", http.StatusBadRequest)
			return
		}

		hash, err := torrentService.FileHash(infoHash, fileIdx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		res := map[string]string{
			"hash": hash,
		}

		err = json.NewEncoder(w).Encode(res) // TODO
	})

	mux.HandleFunc("GET /api/opensubs/{type}/{imdbID}/{fileHash}", func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("type")
		imdbID := r.PathValue("imdbID")
		fileHash := r.PathValue("fileHash")

		subs, err := opensubtitles.Search(kind, imdbID, fileHash)
		if err != nil {
			slog.Error("failed to search subtitles",
				"err", err)
			return
		}

		err = json.NewEncoder(w).Encode(subs)
		if err != nil {
			slog.Error("parse subs to json",
				"err", err)
			return
		}
	})
	//mux.HandleFunc("GET /api/opensubs/{id}", subsService.handleFindSubByID)

	slog.Info("starting anyflix at http://localhost:3000")
	err = http.ListenAndServe(":3000", mux)
	log.Panic(err)
}
