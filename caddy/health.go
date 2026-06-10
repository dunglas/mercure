package caddy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/mercure"
)

const (
	checkReady = "ready"
	checkLive  = "live"
)

var (
	errMethodNotAllowed  = errors.New("method not allowed")
	errUnknownHealthPath = errors.New("unknown health endpoint")
	errHubNotFound       = errors.New("hub not found")
)

func init() { //nolint:gochecknoinits
	caddy.RegisterModule(&Health{})
}

// Health is a Caddy admin API module that exposes transport health check endpoints.
type Health struct{}

// CaddyModule returns the Caddy module information.
func (*Health) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "admin.api.mercure_health",
		New: func() caddy.Module { return new(Health) },
	}
}

// Routes returns the admin routes for the health module.
func (h *Health) Routes() []caddy.AdminRoute {
	return []caddy.AdminRoute{
		{
			Pattern: "/mercure/health/",
			Handler: caddy.AdminHandlerFunc(h.handleHealth),
		},
	}
}

type healthResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func (h *Health) handleHealth(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return caddy.APIError{
			HTTPStatus: http.StatusMethodNotAllowed,
			Err:        errMethodNotAllowed,
		}
	}

	checkType, hubName, err := parseHealthPath(r.URL.Path)
	if err != nil {
		return caddy.APIError{
			HTTPStatus: http.StatusNotFound,
			Err:        err,
		}
	}

	w.Header().Set("Content-Type", "application/json")

	err = h.checkTransports(r, checkType, hubName)
	if errors.Is(err, errHubNotFound) {
		return caddy.APIError{
			HTTPStatus: http.StatusNotFound,
			Err:        err,
		}
	}

	if err != nil {
		// Log the detailed error server-side; return a generic message to
		// avoid exposing internal connection details if the admin API is
		// reachable beyond localhost.
		caddy.Log().Named("admin.api.mercure_health").Error(
			fmt.Sprintf("transport health check failed for check_type=%q hub=%q: %v", checkType, hubName, err),
		)

		w.WriteHeader(http.StatusServiceUnavailable)

		return json.NewEncoder(w).Encode(healthResponse{ //nolint:wrapcheck
			Status: "error",
			Error:  "transport health check failed",
		})
	}

	return json.NewEncoder(w).Encode(healthResponse{Status: "ok"}) //nolint:wrapcheck
}

func parseHealthPath(urlPath string) (checkType, hubName string, err error) {
	path := strings.TrimPrefix(urlPath, "/mercure/health/")
	path = strings.TrimSuffix(path, "/")

	switch {
	case path == checkReady:
		return checkReady, "", nil
	case path == checkLive:
		return checkLive, "", nil
	case strings.HasSuffix(path, "/"+checkReady):
		return checkReady, strings.TrimSuffix(path, "/"+checkReady), nil
	case strings.HasSuffix(path, "/"+checkLive):
		return checkLive, strings.TrimSuffix(path, "/"+checkLive), nil
	default:
		return "", "", errUnknownHealthPath
	}
}

func (h *Health) checkTransports(r *http.Request, checkType, hubName string) error {
	infos := h.snapshotHubs()

	var matched bool

	for _, info := range infos {
		if hubName != "" && info.name != hubName {
			continue
		}

		matched = true

		checker, ok := info.transport.(mercure.TransportHealthChecker)
		if !ok {
			continue
		}

		var err error
		if checkType == checkReady {
			err = checker.Ready(r.Context())
		} else {
			err = checker.Live(r.Context())
		}

		if err != nil {
			return fmt.Errorf("transport %q: %w", info.name, err)
		}
	}

	if hubName != "" && !matched {
		return fmt.Errorf("%w: %q", errHubNotFound, hubName)
	}

	return nil
}

func (h *Health) snapshotHubs() []*hubInfo {
	hubsMu.Lock()
	defer hubsMu.Unlock()

	infos := make([]*hubInfo, 0, len(hubs))
	for _, info := range hubs {
		infos = append(infos, info)
	}

	return infos
}

// Interface guards.
var _ caddy.AdminRouter = (*Health)(nil)
