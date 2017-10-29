package middleware

import (
	"expvar"
	"net/http"
	"net/http/pprof"

	"github.com/SirAiedail/chi"
)

// Profiler is a convenient subrouter used for mounting net/http/pprof. ie.
//
//  func MyService() chi.Handler {
//    r := chi.NewRouter()
//    // ..middlewares
//    r.Mount("/debug", middleware.Profiler())
//    // ..routes
//    return r
//  }
func Profiler() chi.Handler {
	r := chi.NewRouter()
	r.Use(NoCache)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		http.Redirect(w, r, r.RequestURI+"/pprof/", 301)
		return nil
	})
	r.HandleFunc("/pprof", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		http.Redirect(w, r, r.RequestURI+"/", 301)
		return nil
	})

	r.HandleFunc("/pprof/*", chi.FromHTTPHandlerFunc(pprof.Index))
	r.HandleFunc("/pprof/cmdline", chi.FromHTTPHandlerFunc(pprof.Cmdline))
	r.HandleFunc("/pprof/profile", chi.FromHTTPHandlerFunc(pprof.Profile))
	r.HandleFunc("/pprof/symbol", chi.FromHTTPHandlerFunc(pprof.Symbol))
	r.HandleFunc("/pprof/trace", chi.FromHTTPHandlerFunc(pprof.Trace))
	r.Handle("/vars", chi.FromHTTPHandler(expvar.Handler()))

	return r
}
