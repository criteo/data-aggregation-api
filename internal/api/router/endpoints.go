package router

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/app"
	"github.com/criteo/data-aggregation-api/internal/convertor/device"
)

const contentType = "Content-Type"
const applicationJSON = "application/json"
const hostnameKey = "hostname"
const wildcard = "*"

func healthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(contentType, applicationJSON)
	_, _ = fmt.Fprintf(w, `{"status": "ok"}`)
}

func getVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(contentType, applicationJSON)
	_, _ = fmt.Fprintf(w, `{"version": "%s", "build_time": "%s", "build_user": "%s"}`, app.Info.Version, app.Info.BuildTime, app.Info.BuildUser)
}

func prometheusMetrics(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	}
}

// getAFKEnabled endpoint returns all AFK enabled devices.
// They are supposed to be managed by AFK, meaning the configuration should be applied periodically.
func (m *Manager) getAFKEnabled(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, applicationJSON)
	hostname := r.PathValue(hostnameKey)

	if hostname == wildcard {
		out, err := m.devices.ListAFKEnabledDevicesJSON()
		if err != nil {
			log.Error().Err(err).Send()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(out)
		return
	}

	out, err := m.devices.IsAFKEnabledJSON(hostname)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("{}"))
			return
		}

		log.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(out)
}

// getDeviceOpenConfig endpoint returns OpenConfig JSON for one or all devices.
func (m *Manager) getDeviceOpenConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, applicationJSON)
	hostname := r.PathValue(hostnameKey)
	if hostname == wildcard {
		cfg, err := m.devices.GetAllDevicesOpenConfigJSON()
		if err != nil {
			log.Error().Err(err).Send()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(cfg)
		return
	}

	cfg, err := m.devices.GetDeviceOpenConfigJSON(hostname)
	if err != nil {
		log.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(cfg)
}

// getDeviceIETFConfig endpoint returns Ietf JSON for one or all devices.
func (m *Manager) getDeviceIETFConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, applicationJSON)
	hostname := r.PathValue(hostnameKey)
	if hostname == wildcard {
		cfg, err := m.devices.GetAllDevicesIETFConfigJSON()
		if err != nil {
			log.Error().Err(err).Send()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(cfg)
		return
	}

	cfg, err := m.devices.GetDeviceIETFConfigJSON(hostname)
	if err != nil {
		log.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(cfg)
}

// getDeviceConfig endpoint returns Ietf & openconfig JSON for one or all devices.
func (m *Manager) getDeviceConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, applicationJSON)
	hostname := r.PathValue(hostnameKey)
	if hostname == wildcard {
		cfg, err := m.devices.GetAllDevicesConfigJSON()
		if err != nil {
			log.Error().Err(err).Send()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(cfg)
		return
	}

	cfg, err := m.devices.GetDeviceConfigJSON(hostname)
	if err != nil {
		log.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(cfg)
}

// getLastReport returns the last or current report.
func (m *Manager) getLastReport(w http.ResponseWriter, _ *http.Request) {
	out, err := m.reports.GetLastJSON()
	if err != nil {
		log.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(contentType, applicationJSON)
	_, _ = w.Write(out)
}

// getLastCompleteReport returns the previous build report.
func (m *Manager) getLastCompleteReport(w http.ResponseWriter, _ *http.Request) {
	out, err := m.reports.GetLastCompleteJSON()
	if err != nil {
		log.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(contentType, applicationJSON)
	_, _ = w.Write(out)
}

// getLastSuccessfulReport returns the previous successful build report.
func (m *Manager) getLastSuccessfulReport(w http.ResponseWriter, _ *http.Request) {
	out, err := m.reports.GetLastSuccessfulJSON()
	if err != nil {
		log.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(contentType, applicationJSON)
	_, _ = w.Write(out)
}

// triggerBuild enables the user to trigger a new build.
//
// It only accepts one build request at a time.
func (m *Manager) triggerBuild(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(contentType, applicationJSON)
	select {
	case m.newBuildRequest <- struct{}{}:
		_, _ = w.Write([]byte("{\"message\": \"new build request received\""))
	default:
		_, _ = w.Write([]byte("{\"message\": \"a build request is already pending\""))
	}
}
