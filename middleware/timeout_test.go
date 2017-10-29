package middleware

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SirAiedail/chi"
)

func TestNoTimeout(t *testing.T) {
	r := chi.NewRouter()

	r.Use(Timeout(time.Second * 3))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		w.WriteHeader(http.StatusOK)
		time.Sleep(time.Second * 1) // Expensive operation.
		w.Write(testContent)
		return nil
	})

	server := httptest.NewServer(r.ToHTTPHandler())
	defer server.Close()

	client := http.Client{
		Timeout: time.Second * 5, // Maximum waiting time.
	}

	res, err := client.Get(server.URL)
	assertNoError(t, err)

	assertEqual(t, http.StatusOK, res.StatusCode)
	buf, err := ioutil.ReadAll(res.Body)
	assertNoError(t, err)
	assertEqual(t, testContent, buf)
}

func TestRouteTimeout(t *testing.T) {
	r := chi.NewRouter()

	r.Use(Timeout(time.Second * 1))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		ctx := r.Context()

		select {
		case <-ctx.Done():
			// Canceled by timeout
			return nil
		case <-time.After(time.Second * 3):
			// simulating some hard work
		}
		w.Write(testContent)
		w.WriteHeader(http.StatusOK)

		return nil
	})

	server := httptest.NewServer(r.ToHTTPHandler())
	defer server.Close()

	client := http.Client{
		Timeout: time.Second * 5, // Maximum waiting time.
	}

	res, err := client.Get(server.URL)
	assertNoError(t, err)
	assertEqual(t, http.StatusGatewayTimeout, res.StatusCode)
}

func TestCustomTimeoutError(t *testing.T) {
	r := chi.NewRouter()

	r.Use(Timeout(time.Second * 1))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		ctx := r.Context()

		select {
		case <-ctx.Done():
			// Canceled by timeout
			return chi.Error{Code: http.StatusInternalServerError}
		case <-time.After(time.Second * 3):
			// simulating some hard work
		}
		w.Write(testContent)
		w.WriteHeader(http.StatusOK)

		return nil
	})

	server := httptest.NewServer(r.ToHTTPHandler())
	defer server.Close()

	client := http.Client{
		Timeout: time.Second * 5, // Maximum waiting time.
	}

	res, err := client.Get(server.URL)
	assertNoError(t, err)
	assertEqual(t, http.StatusInternalServerError, res.StatusCode)
}

func TestClientTimeout(t *testing.T) {
	r := chi.NewRouter()

	r.Use(Timeout(time.Second * 3))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError {
		ctx := r.Context()

		select {
		case <-ctx.Done():
			// Canceled by timeout
			return nil
		case <-time.After(time.Second * 5):
			// simulating some hard work
		}
		w.Write(testContent)
		w.WriteHeader(http.StatusOK)

		return nil
	})

	server := httptest.NewServer(r.ToHTTPHandler())
	defer server.Close()

	client := http.Client{
		Timeout: time.Second * 1, // Maximum waiting time.
	}

	_, err := client.Get(server.URL)
	assertError(t, err)
}
