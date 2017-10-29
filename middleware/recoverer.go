package middleware

// The original work was derived from Goji's middleware, source:
// https://github.com/zenazn/goji/tree/master/web/middleware

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/SirAiedail/chi"
)

// Recoverer is a middleware that recovers from panics, logs the panic (and a
// backtrace), and returns a HTTP 500 (Internal Server Error) status if
// possible. Recoverer prints a request ID if one is provided.
//
// Alternatively, look at https://github.com/pressly/lg middleware pkgs.
func Recoverer(next chi.Handler) chi.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		err := next.ServeHTTP(w, r)
		if err != nil {
			return err
		} else if rvr := recover(); rvr != nil {
			logEntry := GetLogEntry(r)
			if logEntry != nil {
				logEntry.Panic(rvr, debug.Stack())
			} else {
				fmt.Fprintf(os.Stderr, "Panic: %+v\n", rvr)
				debug.PrintStack()
			}
			return chi.Error{Code: http.StatusInternalServerError}
		} else {
			return nil
		}
	}

	return chi.HandlerFunc(fn)
}
