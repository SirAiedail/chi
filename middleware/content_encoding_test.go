package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SirAiedail/chi"
)

func TestContentEncodingMiddleware(t *testing.T) {
	t.Parallel()

	// support for:
	// Content-Encoding: gzip
	// Content-Encoding: deflate
	// Content-Encoding: gzip, deflate
	// Content-Encoding: deflate, gzip
	middleware := AllowContentEncoding("deflate", "gzip")

	tests := []struct {
		name           string
		encodings      []string
		expectedStatus int
	}{
		{
			name:           "Support no encoding",
			encodings:      []string{},
			expectedStatus: 200,
		},
		{
			name:           "Support gzip encoding",
			encodings:      []string{"gzip"},
			expectedStatus: 200,
		},
		{
			name:           "No support for br encoding",
			encodings:      []string{"br"},
			expectedStatus: 415,
		},
		{
			name:           "Support for gzip and deflate encoding",
			encodings:      []string{"gzip", "deflate"},
			expectedStatus: 200,
		},
		{
			name:           "Support for deflate and gzip encoding",
			encodings:      []string{"deflate", "gzip"},
			expectedStatus: 200,
		},
		{
			name:           "No support for deflate and br encoding",
			encodings:      []string{"deflate", "br"},
			expectedStatus: 415,
		},
	}

	for _, tt := range tests {
		var tt = tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := []byte("This is my content. There are many like this but this one is mine")
			r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
			for _, encoding := range tt.encodings {
				r.Header.Set("Content-Encoding", encoding)
			}

			w := httptest.NewRecorder()
			router := chi.NewRouter()
			router.Use(middleware)
			router.Post("/", func(w http.ResponseWriter, r *http.Request) chi.HandlerError { return nil })

			err := router.ServeHTTP(w, r)
			res := w.Result()
			if tt.expectedStatus >= 400 && err != nil && err.StatusCode() != tt.expectedStatus {
				t.Errorf("error is incorrect, got %d, weant %d", err.StatusCode(), tt.expectedStatus)
			} else if tt.expectedStatus < 400 && err != nil {
				t.Errorf("response is incorrect, got error, want response")
			} else if err == nil && res.StatusCode != tt.expectedStatus {
				t.Errorf("response is incorrect, got %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}
