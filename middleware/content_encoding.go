package middleware

import (
	"net/http"
	"strings"

	"github.com/SirAiedail/chi"
)

// AllowContentEncoding enforces a whitelist of request Content-Encoding otherwise responds
// with a 415 Unsupported Media Type status.
func AllowContentEncoding(contentEncoding ...string) func(next chi.Handler) chi.Handler {
	allowedEncodings := make(map[string]struct{}, len(contentEncoding))
	for _, encoding := range contentEncoding {
		allowedEncodings[strings.TrimSpace(strings.ToLower(encoding))] = struct{}{}
	}

	return func(next chi.Handler) chi.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
			requestEncodings := r.Header["Content-Encoding"]
			// skip check for empty content body or no Content-Encoding
			if r.ContentLength == 0 {
				return next.ServeHTTP(w, r)
			}
			// All encodings in the request must be allowed
			for _, encoding := range requestEncodings {
				if _, ok := allowedEncodings[strings.TrimSpace(strings.ToLower(encoding))]; !ok {
					return chi.Error{Code: http.StatusUnsupportedMediaType}
				}
			}

			return next.ServeHTTP(w, r)
		}

		return chi.HandlerFunc(fn)
	}
}
