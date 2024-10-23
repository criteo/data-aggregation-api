package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/api/auth"
	"github.com/criteo/data-aggregation-api/internal/config"
	"github.com/criteo/data-aggregation-api/internal/convertor/device"
	"github.com/criteo/data-aggregation-api/internal/report"
)

const shutdownTimeout = 5 * time.Second

type DevicesRepository interface {
	Set(devices map[string]*device.Device)
	ListAFKEnabledDevicesJSON() ([]byte, error)
	IsAFKEnabledJSON(hostname string) ([]byte, error)
	GetAllDevicesOpenConfigJSON() ([]byte, error)
	GetDeviceOpenConfigJSON(hostname string) ([]byte, error)
	GetAllDevicesIETFConfigJSON() ([]byte, error)
	GetDeviceIETFConfigJSON(hostname string) ([]byte, error)
	GetAllDevicesConfigJSON() ([]byte, error)
	GetDeviceConfigJSON(hostname string) ([]byte, error)
}

type Manager struct {
	devices         DevicesRepository
	reports         *report.Repository
	newBuildRequest chan<- struct{}
}

// NewManager creates and initializes a new API manager.
func NewManager(deviceRepo DevicesRepository, reports *report.Repository, restartRequest chan<- struct{}) *Manager {
	return &Manager{devices: deviceRepo, reports: reports, newBuildRequest: restartRequest}
}

// ListenAndServe starts to serve Web API requests.
func (m *Manager) ListenAndServe(ctx context.Context, address string, port int, enablepprof bool) error {
	defer func() {
		close(m.newBuildRequest)
		log.Warn().Msg("Shutdown.")
	}()

	withAuth, err := auth.NewBasicAuth(ctx, config.Cfg.Authentication)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /metrics", prometheusMetrics(promhttp.Handler()))
	mux.HandleFunc("GET /api/version", getVersion)
	mux.HandleFunc("GET /api/health", healthCheck)
	mux.HandleFunc("GET /v1/devices/{hostname}/afk_enabled", withAuth.Wrap(m.getAFKEnabled))
	mux.HandleFunc("GET /v1/devices/{hostname}/openconfig", withAuth.Wrap(m.getDeviceOpenConfig))
	mux.HandleFunc("GET /v1/devices/{hostname}/ietfconfig", withAuth.Wrap(m.getDeviceIETFConfig))
	mux.HandleFunc("GET /v1/devices/{hostname}/config", withAuth.Wrap(m.getDeviceConfig))
	mux.HandleFunc("GET /v1/report/last", withAuth.Wrap(m.getLastReport))
	mux.HandleFunc("GET /v1/report/last/complete", withAuth.Wrap(m.getLastCompleteReport))
	mux.HandleFunc("GET /v1/report/last/successful", withAuth.Wrap(m.getLastSuccessfulReport))
	mux.HandleFunc("POST /v1/build/trigger", withAuth.Wrap(m.triggerBuild))

	if enablepprof {
		mux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/allocs", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/goroutine", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/heap", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
		mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	}

	listenSocket := fmt.Sprint(address, ":", port)
	log.Info().Msgf("Start webserver - listening on %s", listenSocket)

	httpServer := http.Server{Addr: listenSocket, Handler: mux}

	// TODO: handle http failure! with a channel
	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("stopped to listen and serve")
		}
	}()

	<-ctx.Done()
	ctxCancel, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(ctxCancel); err != nil {
		log.Error().Err(err).Send()
	}

	return nil
}
