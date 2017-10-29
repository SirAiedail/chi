package middleware

import (
	"net/http"
	"strings"

	"github.com/SirAiedail/chi"
)

// Heartbeat endpoint middleware useful to setting up a path like
// `/ping` that load balancers or uptime testing external services
// can make a request before hitting any routes. It's also convenient
// to place this above ACL middlewares as well.
func Heartbeat(endpoint string) func(chi.Handler) chi.Handler {
	f := func(h chi.Handler) chi.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
			if r.Method == "GET" && strings.EqualFold(r.URL.Path, endpoint) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("."))
				return nil
			}
			return h.ServeHTTP(w, r)
		}
		return chi.HandlerFunc(fn)
	}
	return f
}
