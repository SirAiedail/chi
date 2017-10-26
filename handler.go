package chi

import "net/http"

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request) HandlerError
}

type HandlerFunc func(http.ResponseWriter, *http.Request) HandlerError

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) HandlerError {
	return f(w, r)
}

func (f HandlerFunc) ToHTTPFunc() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := f(w, r)
		if err != nil {
			panic("chi: an unhandled error occurred")
		}
	})
}

func FromHTTPHandler(h http.Handler) Handler {
	return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		h.ServeHTTP(w, r)
		return nil
	})
}

type HandlerError interface {
	error
	Code() int
}

type Error struct {
	code int
	err  error
}

func (e Error) Code() int {
	return e.code
}

func (e Error) Error() string {
	return e.err.Error()
}
