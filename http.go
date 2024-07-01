package rprof

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"
)

// ProfHandler is an HTTP handler that starts the profiler for a given duration.
type ProfHandler struct {
	p *Rprof
}

// Handler returns a new ProfHandler that uses the default profiler.
func Handler() *ProfHandler {
	return &ProfHandler{p: profiler}
}

// NewHandler returns a new ProfHandler that uses the given profiler.
func NewHandler(p *Rprof) *ProfHandler {
	return &ProfHandler{p: p}
}

// ServeHTTP starts the profiler for the given duration and writes the profile to the response.
// Implements http.Handler.
func (h *ProfHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Default to 10 seconds.
	seconds := 10
	if r.FormValue("seconds") != "" {
		var err error
		// If given, parse the duration.
		seconds, err = strconv.Atoi(r.FormValue("seconds"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Start the profiler.
	if err := h.p.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wait for the duration for samples to accumulate.
	time.Sleep(time.Duration(seconds) * time.Second)

	// Stop the profiler, which returns the profile.
	prof, err := h.p.Stop()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Marshal the proto message, compress it, and write it to the response.
	content, err := proto.Marshal(prof)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	buf := bytes.NewBuffer(nil)

	gz := gzip.NewWriter(buf)
	defer gz.Close()

	if _, err := gz.Write(content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}
