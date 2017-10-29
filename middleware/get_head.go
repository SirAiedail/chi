package middleware

import (
	"net/http"

	"github.com/SirAiedail/chi"
)

func GetHead(next chi.Handler) chi.Handler {
	return chi.HandlerFunc(func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		if r.Method == "HEAD" {
			rctx := chi.RouteContext(r.Context())
			routePath := rctx.RoutePath
			if routePath == "" {
				if r.URL.RawPath != "" {
					routePath = r.URL.RawPath
				} else {
					routePath = r.URL.Path
				}
			}

			// Temporary routing context to look-ahead before routing the request
			tctx := chi.NewRouteContext()

			// Attempt to find a HEAD handler for the routing path, if not found, traverse
			// the router as through its a GET route, but proceed with the request
			// with the HEAD method.
			if !rctx.Routes.Match(tctx, "HEAD", routePath) {
				rctx.RouteMethod = "GET"
				rctx.RoutePath = routePath
				return next.ServeHTTP(w, r)
			}
		}

		return next.ServeHTTP(w, r)
	})
}
