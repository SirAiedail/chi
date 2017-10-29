package middleware

import (
	"context"
	"net/http"

	"github.com/SirAiedail/chi"
)

// WithValue is a middleware that sets a given key/value in a context chain.
func WithValue(key interface{}, val interface{}) func(next chi.Handler) chi.Handler {
	return func(next chi.Handler) chi.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
			r = r.WithContext(context.WithValue(r.Context(), key, val))
			return next.ServeHTTP(w, r)
		}
		return chi.HandlerFunc(fn)
	}
}
