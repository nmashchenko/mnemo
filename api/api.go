package api

import (
	"context"
	"encoding/json"
	"log"
	"mnemo/clog"
	"mnemo/config"
	"mnemo/deps"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type API struct {
	config  *config.Config
	deps    *deps.Dependencies
	server  *http.Server
	log     clog.ICustomLog
	version string
}

type ResponseJSON struct {
	Status  int               `json:"status"`
	Message string            `json:"message"`
	Values  map[string]string `json:"values,omitempty"`
	Errors  string            `json:"errors,omitempty"`
}

func New(cfg *config.Config, d *deps.Dependencies, version string) (*API, error) {
	if cfg == nil {
		return nil, errors.New("cfg cannot be nil")
	}

	if d == nil {
		return nil, errors.New("deps cannot be nil")
	}

	server := &http.Server{
		Addr: cfg.APIListenAddress,
	}

	a := &API{
		config:  cfg,
		deps:    d,
		server:  server,
		version: version,
		log:     d.Log.With(zap.String("pkg", "api")),
	}

	// Run shutdown listener
	go a.runShutdownListener()

	return a, nil

}

func (a *API) runShutdownListener() {
	<-a.deps.ShutdownCtx.Done()

	// Give server 5s to shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		a.log.Error("Error shutting down API server", zap.Error(err))
	}
}

func (a *API) Run() error {
	logger := a.log.With(zap.String("method", "Run"))

	router := httprouter.New()

	a.server.Handler = router

	router.HandlerFunc(http.MethodGet, "/health-check", a.healthCheckHandler)
	router.HandlerFunc(http.MethodGet, "/version", a.versionHandler)
	router.HandlerFunc(http.MethodPost, "/api/v1/qr", a.createQRCodeHandler)
	router.HandlerFunc(http.MethodGet, "/api/v1/join", a.joinHubHandler)

	// Maybe enable profiling
	if a.config.EnablePprof {
		router.Handler(http.MethodGet, "/debug/pprof/*item", http.DefaultServeMux)
	}

	logger.Info("API server running", zap.String("listenAddress", a.config.APIListenAddress))

	return a.server.ListenAndServe()
}

// WriteJSON is a helper function for writing JSON responses
func WriteJSON(rw http.ResponseWriter, payload interface{}, status int) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ERROR: unable to marshal JSON during WriteJSON "+
			"(payload: '%s'; status: '%d'): %s\n", payload, status, err)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)

	if _, err := rw.Write(data); err != nil {
		log.Printf("ERROR: unable to write resp in WriteJSON: %s\n", err)
		return
	}
}
