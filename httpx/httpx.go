package httpx

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

func JSON(w http.ResponseWriter, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		ErrorJSON(w, ErrorJSONParams{
			Err: err,
			Msg: "failed to serialize response",
		})
		return
	}

	_, err = w.Write(b)
	if err != nil {
		slog.Error("write response", "err", err)
	}
}

type ErrorJSONParams struct {
	Err    error
	Msg    string
	Status int
}

func ErrorJSON(w http.ResponseWriter, params ErrorJSONParams) {
	var finalMsg string

	if params.Err != nil {
		slog.Error(params.Msg, "err", params.Err)
		finalMsg = fmt.Sprintf("%v: %v", params.Msg, params.Err)
	} else {
		finalMsg = params.Msg
	}

	b, _ := json.MarshalIndent(map[string]any{
		"params.error": true,
		"message":      finalMsg,
	}, "", "  ")

	status := params.Status
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

type ResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *ResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *ResponseWriter) Status() int {
	if w.status == 0 {
		return 200
	}
	return w.status
}
