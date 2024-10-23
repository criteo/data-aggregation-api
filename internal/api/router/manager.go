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
	"github.com/julienschmidt/httprouter"
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

	router := httprouter.New()

	router.GET("/metrics", prometheusMetrics(promhttp.Handler()))
	router.GET("/api/version", getVersion)
	router.GET("/api/health", healthCheck)
	router.GET("/v1/devices/:hostname/afk_enabled", withAuth.Wrap(m.getAFKEnabled))
	router.GET("/v1/devices/:hostname/openconfig", withAuth.Wrap(m.getDeviceOpenConfig))
	router.GET("/v1/devices/:hostname/ietfconfig", withAuth.Wrap(m.getDeviceIETFConfig))
	router.GET("/v1/devices/:hostname/config", withAuth.Wrap(m.getDeviceConfig))
	router.GET("/v1/report/last", withAuth.Wrap(m.getLastReport))
	router.GET("/v1/report/last/complete", withAuth.Wrap(m.getLastCompleteReport))
	router.GET("/v1/report/last/successful", withAuth.Wrap(m.getLastSuccessfulReport))
	router.POST("/v1/build/trigger", withAuth.Wrap(m.triggerBuild))

	if enablepprof {
		router.HandlerFunc(http.MethodGet, "/debug/pprof/", pprof.Index)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/allocs", pprof.Index)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/goroutine", pprof.Index)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/heap", pprof.Index)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/profile", pprof.Profile)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/trace", pprof.Trace)
		router.HandlerFunc(http.MethodGet, "/debug/pprof/symbol", pprof.Symbol)
	}

	listenSocket := fmt.Sprint(address, ":", port)
	log.Info().Msgf("Start webserver - listening on %s", listenSocket)

	httpServer := http.Server{Addr: listenSocket, Handler: router}

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
