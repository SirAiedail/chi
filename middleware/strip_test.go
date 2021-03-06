package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SirAiedail/chi"
)

func TestStripSlashes(t *testing.T) {
	r := chi.NewRouter()

	// This middleware must be mounted at the top level of the router, not at the end-handler
	// because then it'll be too late and will end up in a 404
	r.Use(StripSlashes)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		return chi.Error{Code: http.StatusNotFound}
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		w.Write([]byte("root"))
		return nil
	})

	r.Route("/accounts/{accountID}", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
			accountID := chi.URLParam(r, "accountID")
			w.Write([]byte(accountID))
			return nil
		})
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/", nil); body != "root" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "//", nil); body != "root" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/accounts/admin", nil); body != "admin" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/accounts/admin/", nil); body != "admin" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "GET", "/nothing-here", nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf(body)
	}
}

func TestStripSlashesInRoute(t *testing.T) {
	r := chi.NewRouter()

	r.NotFound(func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		return chi.Error{Code: http.StatusNotFound}
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		w.Write([]byte("hi"))
		return nil
	})

	r.Route("/accounts/{accountID}", func(r chi.Router) {
		r.Use(StripSlashes)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
			w.Write([]byte("accounts index"))
			return nil
		})
		r.Get("/query", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
			accountID := chi.URLParam(r, "accountID")
			w.Write([]byte(accountID))
			return nil
		})
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "hi" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "GET", "/hi/", nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf(body)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin", nil); resp != "accounts index" {
		t.Fatalf(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin/", nil); resp != "accounts index" {
		t.Fatalf(resp)
	}
	if _, resp := testRequest(t, ts, "GET", "/accounts/admin/query", nil); resp != "admin" {
		t.Fatalf(resp)
	}
	if _, body := testRequest(t, ts, "GET", "/accounts/admin/query/", nil); body != "admin" {
		t.Fatalf(body)
	}
}

func TestRedirectSlashes(t *testing.T) {
	r := chi.NewRouter()

	// This middleware must be mounted at the top level of the router, not at the end-handler
	// because then it'll be too late and will end up in a 404
	r.Use(RedirectSlashes)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		return chi.Error{Code: http.StatusNotFound}
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		w.Write([]byte("root"))
		return nil
	})

	r.Route("/accounts/{accountID}", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
			accountID := chi.URLParam(r, "accountID")
			w.Write([]byte(accountID))
			return nil
		})
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if resp, body := testRequest(t, ts, "GET", "/", nil); body != "root" && resp.StatusCode != 200 {
		t.Fatalf(body)
	}

	// NOTE: the testRequest client will follow the redirection..
	if resp, body := testRequest(t, ts, "GET", "//", nil); body != "root" && resp.StatusCode != 200 {
		t.Fatalf(body)
	}

	if resp, body := testRequest(t, ts, "GET", "/accounts/admin", nil); body != "admin" && resp.StatusCode != 200 {
		t.Fatalf(body)
	}

	// NOTE: the testRequest client will follow the redirection..
	if resp, body := testRequest(t, ts, "GET", "/accounts/admin/", nil); body != "admin" && resp.StatusCode != 200 {
		t.Fatalf(body)
	}

	if resp, body := testRequest(t, ts, "GET", "/nothing-here", nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf(body)
	}

	// Ensure redirect Location url is correct
	{
		resp, body := testRequestNoRedirect(t, ts, "GET", "/accounts/someuser/", nil)
		if resp.StatusCode != 301 {
			t.Fatalf(body)
		}
		if resp.Header.Get("Location") != "/accounts/someuser" {
			t.Fatalf("invalid redirection, should be /accounts/someuser")
		}
	}

	// Ensure query params are kept in tact upon redirecting a slash
	{
		resp, body := testRequestNoRedirect(t, ts, "GET", "/accounts/someuser/?a=1&b=2", nil)
		if resp.StatusCode != 301 {
			t.Fatalf(body)
		}
		if resp.Header.Get("Location") != "/accounts/someuser?a=1&b=2" {
			t.Fatalf("invalid redirection, should be /accounts/someuser?a=1&b=2")
		}

	}
}
