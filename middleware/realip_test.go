package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SirAiedail/chi"
)

func TestXRealIP(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add("X-Real-IP", "100.100.100.100")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(RealIP)

	realIP := ""
	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		realIP = r.RemoteAddr
		w.Write([]byte("Hello World"))
		return nil
	})
	err := r.ServeHTTP(w, req)
	if err != nil {
		t.Fatal(err)
	}

	if w.Code != 200 {
		t.Fatal("Response StatusCode should be 200")
	}

	if realIP != "100.100.100.100" {
		t.Fatal("Test get real IP error.")
	}
}

func TestXForwardForIP(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Add("X-Forwarded-For", "100.100.100.100")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(RealIP)

	realIP := ""
	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		realIP = r.RemoteAddr
		w.Write([]byte("Hello World"))
		return nil
	})
	err := r.ServeHTTP(w, req)
	if err != nil {
		t.Fatal(err)
	}

	if w.Code != 200 {
		t.Fatal("Response StatusCode should be 200")
	}

	if realIP != "100.100.100.100" {
		t.Fatal("Test get real IP error.")
	}
}
