package torrent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/types/infohash"
)

type Service struct {
	client *torrent.Client
}

func DefaultService() (Service, error) {
	svc := Service{}

	cache, err := os.UserCacheDir()
	if err != nil {
		return svc, err
	}

	config := torrent.NewDefaultClientConfig()
	config.DataDir = filepath.Join(cache, "anyflix")

	err = os.MkdirAll(config.DataDir, os.ModePerm)
	if err != nil {
		return svc, err
	}

	client, err := torrent.NewClient(config)
	if err != nil {
		return svc, err
	}

	svc.client = client
	return svc, nil
}

func (h Service) DownloadFile(infoHash string, fileIdx int) {
	torrent, _ := h.client.AddTorrentInfoHash(infohash.FromHexString(infoHash))
	<-torrent.GotInfo()

	file := torrent.Files()[fileIdx]
	file.Download()
}

type Stat struct {
	BytesComplete    int64 `json:"bytesComplete"`
	BytesTotal       int64 `json:"bytesTotal"`
	TotalPeers       int   `json:"totalPeers"`
	PendingPeers     int   `json:"pendingPeers"`
	ActivePeers      int   `json:"activePeers"`
	ConnectedSeeders int   `json:"connectedSeeders"`
	HalfOpenPeers    int   `json:"halfOpenPeers"`
	PiecesComplete   int   `json:"piecesComplete"`
}

func (h Service) Stat(infoHash string, fileIdx int) Stat {
	torrent, _ := h.client.AddTorrentInfoHash(infohash.FromHexString(infoHash))
	<-torrent.GotInfo()

	file := torrent.Files()[fileIdx]
	states := file.State()

	stats := torrent.Stats()
	stat := Stat{
		BytesComplete:    0,
		BytesTotal:       file.Length(),
		TotalPeers:       stats.TotalPeers,
		PendingPeers:     stats.PendingPeers,
		ActivePeers:      stats.ActivePeers,
		ConnectedSeeders: stats.ConnectedSeeders,
		HalfOpenPeers:    stats.HalfOpenPeers,
		PiecesComplete:   stats.PiecesComplete,
	}

	for _, state := range states {
		if state.Completion.Complete {
			stat.BytesComplete += state.Bytes
		}
	}

	return stat
}

func (h Service) StreamFileHTTP(w http.ResponseWriter, r *http.Request, infoHash string, fileIdx int) {
	torrent, _ := h.client.AddTorrentInfoHash(infohash.FromHexString(infoHash))
	<-torrent.GotInfo()

	slog.Debug("torrent info ready", "infoHash", infoHash)

	// TODO: let it panic?
	file := torrent.Files()[fileIdx]

	ranges := strings.SplitN(
		strings.TrimPrefix(r.Header.Get("Range"), "bytes="),
		"-",
		2,
	)

	var start, end int64
	if len(ranges) == 2 {
		start, _ = strconv.ParseInt(ranges[0], 10, 64)
	}
	const chunkSize = 10 * 1024 * 1024
	end = start + chunkSize
	end = min(end, file.Length())

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", "video/mp4") // TODO
	w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.Length()))
	w.WriteHeader(http.StatusPartialContent)

	reader := file.NewReader()
	if _, err := reader.Seek(start, io.SeekStart); err != nil {
		slog.Error("failed to seek",
			"start", start,
			"end", end,
			"err", err)
		return
	}

	slog.Debug("will start streaming chunk", "start", start, "end", end)
	if _, err := io.CopyN(w, reader, chunkSize); err != nil {
		slog.Error("failed to stream chunk",
			"start", start,
			"end", end,
			"err", err)
		return
	}
}

func (h Service) FileHash(infoHash string, fileIdx int) (string, error) {
	slog.Debug("torrentSevice.getFileHash", "infoHash", infoHash, "fileIdx", fileIdx)

	torrent, _ := h.client.AddTorrentInfoHash(infohash.FromHexString(infoHash))
	<-torrent.GotInfo()

	if len(torrent.Files()) == 0 {
		return "", errors.New("invalid torrent")
	}

	if fileIdx >= len(torrent.Files()) {
		return "", errors.New("invalid fileIdx")
	}

	hash := torrent.Piece(fileIdx).Info().Hash().HexString()
	return hash, nil
}

func (h Service) handleGetFileHash(w http.ResponseWriter, r *http.Request) {
	infoHash := r.PathValue("infoHash")

	fileIdx, err := strconv.Atoi(r.PathValue("fileIdx"))
	if err != nil {
		http.Error(w, "invalid fileIdx", http.StatusBadRequest)
		return
	}

	hash, err := h.FileHash(infoHash, fileIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res := getTorrentFileHashResponse{
		Hash: hash,
	}

	err = json.NewEncoder(w).Encode(res) // TODO
}

type getTorrentFileHashResponse struct {
	Hash string `json:"hash"`
}
