package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	testresources "github.com/codeready-toolchain/argocd-mcp-server/test/resources"
	flag "github.com/spf13/pflag"
)

func main() {

	var listen string
	var token string
	var debug bool
	flag.StringVar(&listen, "listen", "", "listen address")
	flag.StringVar(&token, "token", "", "token")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()

	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)
	if debug {
		lvl.Set(slog.LevelDebug)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	}))
	logger.Debug("debug mode enabled")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/applications", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", token):
			logger.Debug("unauthorized request")
			w.WriteHeader(http.StatusUnauthorized)
			return
		case r.URL.Query().Get("name") == "example":
			logger.Debug("serving example application")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(testresources.ExampleApplicationStr))
			return
		case r.URL.Query().Get("name") == "example-error":
			logger.Debug("serving example error application")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		logger.Debug("serving mock applications")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testresources.ApplicationsStr))
	})

	srv := &http.Server{
		Addr:         listen,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	logger.Info("serving argocd mock", "debug", debug, "listen", listen, "token", token)
	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
