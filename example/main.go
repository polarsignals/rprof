package main

import (
	"net/http"

	"github.com/polarsignals/rprof"
)

func main() {
	http.Handle("/debug/rprof", rprof.Handler())
	http.ListenAndServe(":8080", nil)
}
