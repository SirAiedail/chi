package chi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMuxBasic(t *testing.T) {
	var count uint64
	countermw := func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			count++
			return next.ServeHTTP(w, r)
		})
	}

	usermw := func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKey{"user"}, "peter")
			r = r.WithContext(ctx)
			return next.ServeHTTP(w, r)
		})
	}

	exmw := func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := context.WithValue(r.Context(), ctxKey{"ex"}, "a")
			r = r.WithContext(ctx)
			return next.ServeHTTP(w, r)
		})
	}

	logbuf := bytes.NewBufferString("")
	logmsg := "logmw test"
	logmw := func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			logbuf.WriteString(logmsg)
			return next.ServeHTTP(w, r)
		})
	}

	cxindex := func(w http.ResponseWriter, r *http.Request) HandlerError {
		ctx := r.Context()
		user := ctx.Value(ctxKey{"user"}).(string)
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("hi %s", user)))
		return nil
	}

	ping := func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.WriteHeader(200)
		w.Write([]byte("."))
		return nil
	}

	headPing := func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Header().Set("X-Ping", "1")
		w.WriteHeader(200)
		return nil
	}

	createPing := func(w http.ResponseWriter, r *http.Request) HandlerError {
		// create ....
		w.WriteHeader(201)
		return nil
	}

	pingAll := func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.WriteHeader(200)
		w.Write([]byte("ping all"))
		return nil
	}

	pingAll2 := func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.WriteHeader(200)
		w.Write([]byte("ping all2"))
		return nil
	}

	pingOne := func(w http.ResponseWriter, r *http.Request) HandlerError {
		idParam := URLParam(r, "id")
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("ping one id: %s", idParam)))
		return nil
	}

	pingWoop := func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.WriteHeader(200)
		w.Write([]byte("woop." + URLParam(r, "iidd")))
		return nil
	}

	catchAll := func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.WriteHeader(200)
		w.Write([]byte("catchall"))
		return nil
	}

	m := NewRouter()
	m.Use(countermw)
	m.Use(usermw)
	m.Use(exmw)
	m.Use(logmw)
	m.Get("/", cxindex)
	m.Method("GET", "/ping", HandlerFunc(ping))
	m.MethodFunc("GET", "/pingall", pingAll)
	m.MethodFunc("get", "/ping/all", pingAll)
	m.Get("/ping/all2", pingAll2)

	m.Head("/ping", headPing)
	m.Post("/ping", createPing)
	m.Get("/ping/{id}", pingWoop)
	m.Get("/ping/{id}", pingOne) // expected to overwrite to pingOne handler
	m.Get("/ping/{iidd}/woop", pingWoop)
	m.HandleFunc("/admin/*", catchAll)
	// m.Post("/admin/*", catchAll)

	ts := httptest.NewServer(m.ToHTTPHandler())
	defer ts.Close()

	// GET /
	if _, body := testRequest(t, ts, "GET", "/", nil); body != "hi peter" {
		t.Fatalf(body)
	}
	tlogmsg, _ := logbuf.ReadString(0)
	if tlogmsg != logmsg {
		t.Error("expecting log message from middleware:", logmsg)
	}

	// GET /ping
	if _, body := testRequest(t, ts, "GET", "/ping", nil); body != "." {
		t.Fatalf(body)
	}

	// GET /pingall
	if _, body := testRequest(t, ts, "GET", "/pingall", nil); body != "ping all" {
		t.Fatalf(body)
	}

	// GET /ping/all
	if _, body := testRequest(t, ts, "GET", "/ping/all", nil); body != "ping all" {
		t.Fatalf(body)
	}

	// GET /ping/all2
	if _, body := testRequest(t, ts, "GET", "/ping/all2", nil); body != "ping all2" {
		t.Fatalf(body)
	}

	// GET /ping/123
	if _, body := testRequest(t, ts, "GET", "/ping/123", nil); body != "ping one id: 123" {
		t.Fatalf(body)
	}

	// GET /ping/allan
	if _, body := testRequest(t, ts, "GET", "/ping/allan", nil); body != "ping one id: allan" {
		t.Fatalf(body)
	}

	// GET /ping/1/woop
	if _, body := testRequest(t, ts, "GET", "/ping/1/woop", nil); body != "woop.1" {
		t.Fatalf(body)
	}

	// HEAD /ping
	resp, err := http.Head(ts.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Error("head failed, should be 200")
	}
	if resp.Header.Get("X-Ping") == "" {
		t.Error("expecting X-Ping header")
	}

	// GET /admin/catch-this
	if _, body := testRequest(t, ts, "GET", "/admin/catch-thazzzzz", nil); body != "catchall" {
		t.Fatalf(body)
	}

	// POST /admin/catch-this
	resp, err = http.Post(ts.URL+"/admin/casdfsadfs", "text/plain", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Error("POST failed, should be 200")
	}

	if string(body) != "catchall" {
		t.Error("expecting response body: 'catchall'")
	}

	// Custom http method DIE /ping/1/woop
	if resp, body := testRequest(t, ts, "DIE", "/ping/1/woop", nil); body != fmt.Sprintln(http.StatusText(405)) || resp.StatusCode != 405 {
		t.Fatalf(fmt.Sprintf("expecting 405 status and status text, got %d '%s'", resp.StatusCode, body))
	}
}

func TestMuxMounts(t *testing.T) {
	r := NewRouter()

	r.Get("/{hash}", func(w http.ResponseWriter, r *http.Request) HandlerError {
		v := URLParam(r, "hash")
		w.Write([]byte(fmt.Sprintf("/%s", v)))
		return nil
	})

	r.Route("/{hash}/share", func(r Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
			v := URLParam(r, "hash")
			w.Write([]byte(fmt.Sprintf("/%s/share", v)))
			return nil
		})
		r.Get("/{network}", func(w http.ResponseWriter, r *http.Request) HandlerError {
			v := URLParam(r, "hash")
			n := URLParam(r, "network")
			w.Write([]byte(fmt.Sprintf("/%s/share/%s", v, n)))
			return nil
		})
	})

	m := NewRouter()
	m.Mount("/sharing", r)

	ts := httptest.NewServer(m.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/sharing/aBc", nil); body != "/aBc" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/sharing/aBc/share", nil); body != "/aBc/share" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/sharing/aBc/share/twitter", nil); body != "/aBc/share/twitter" {
		t.Fatalf(body)
	}
}

func TestMuxPlain(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("bye"))
		return nil
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return Error{Code: 404}
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "GET", "/", nil); resp.StatusCode != 404 || body != fmt.Sprintln(http.StatusText(404)) {
		t.Fatalf("expected response 404, got '%d': '%s'", resp.StatusCode, body)
	}
}

func TestMuxEmptyRoutes(t *testing.T) {
	mux := NewRouter()

	apiRouter := NewRouter()
	// oops, we forgot to declare any route handlers

	mux.Handle("/api*", apiRouter)

	if _, body, err := testHandler(t, mux, "GET", "/", nil); err.StatusCode() != 404 {
		t.Fatalf(body)
	}

	if _, body, err := testHandler(t, mux, "GET", "/api", nil); err.StatusCode() != 500 {
		t.Fatalf(body)
	}

	if _, body, err := testHandler(t, mux, "GET", "/api/abc", nil); err.StatusCode() != 500 {
		t.Fatalf(body)
	}
}

// Test a mux that routes a trailing slash, see also middleware/strip_test.go
// for an example of using a middleware to handle trailing slashes.
func TestMuxTrailingSlash(t *testing.T) {
	r := NewRouter()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return Error{Code: 404}
	})

	subRoutes := NewRouter()
	indexHandler := func(w http.ResponseWriter, r *http.Request) HandlerError {
		accountID := URLParam(r, "accountID")
		w.Write([]byte(accountID))
		return nil
	}
	subRoutes.Get("/", indexHandler)

	r.Mount("/accounts/{accountID}", subRoutes)
	r.Get("/accounts/{accountID}/", indexHandler)

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/accounts/admin", nil); body != "admin" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/accounts/admin/", nil); body != "admin" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "GET", "/nothing-here", nil); resp.StatusCode != 404 {
		t.Fatalf(body)
	}
}

func TestMuxNestedNotFound(t *testing.T) {
	r := NewRouter()

	r.Use(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"mw"}, "mw"))
			return next.ServeHTTP(w, r)
		})
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("bye"))
		return nil
	})

	r.With(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"with"}, "with"))
			return next.ServeHTTP(w, r)
		})
	}).NotFound(func(w http.ResponseWriter, r *http.Request) HandlerError {
		chkMw := r.Context().Value(ctxKey{"mw"}).(string)
		chkWith := r.Context().Value(ctxKey{"with"}).(string)
		return Error{
			Code: 404,
			Err:  errors.New(fmt.Sprintf("root 404 %s %s", chkMw, chkWith)),
		}
	})

	sr1 := NewRouter()

	sr1.Get("/sub", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("sub"))
		return nil
	})
	sr1.Group(func(sr1 Router) {
		sr1.Use(func(next Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{"mw2"}, "mw2"))
				return next.ServeHTTP(w, r)
			})
		})
		sr1.NotFound(func(w http.ResponseWriter, r *http.Request) HandlerError {
			chkMw2 := r.Context().Value(ctxKey{"mw2"}).(string)
			return Error{
				Code: 404,
				Err:  errors.New(fmt.Sprintf("sub 404 %s", chkMw2)),
			}
		})
	})

	sr2 := NewRouter()
	sr2.Get("/sub", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("sub2"))
		return nil
	})

	r.Mount("/admin1", sr1)
	r.Mount("/admin2", sr2)

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "GET", "/nothing-here", nil); resp.StatusCode != 404 || body != "root 404 mw with\n" {
		t.Fatalf("expected status Code 404 with body 'root 404 mw with\n', got %d with '%s'", resp.StatusCode, body)
	}
	if _, body := testRequest(t, ts, "GET", "/admin1/sub", nil); body != "sub" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "GET", "/admin1/nope", nil); resp.StatusCode != 404 || body != "sub 404 mw2\n" {
		t.Fatalf("expected status Code 404 with body 'sub 404 mw2\n', got %d with '%s'", resp.StatusCode, body)
	}
	if _, body := testRequest(t, ts, "GET", "/admin2/sub", nil); body != "sub2" {
		t.Fatalf(body)
	}

	// Not found pages should bubble up to the root.
	if resp, body := testRequest(t, ts, "GET", "/admin2/nope", nil); resp.StatusCode != 404 || body != "root 404 mw with\n" {
		t.Fatalf("expected status Code 404 with body 'root 404 mw with\n', got %d with '%s'", resp.StatusCode, body)
	}
}

func TestMuxNestedMethodNotAllowed(t *testing.T) {
	r := NewRouter()
	r.Get("/root", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("root"))
		return nil
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return Error{Code: 405}
	})

	sr1 := NewRouter()
	sr1.Get("/sub1", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("sub1"))
		return nil
	})
	sr1.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return Error{Code: 405}
	})

	sr2 := NewRouter()
	sr2.Get("/sub2", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("sub2"))
		return nil
	})

	r.Mount("/prefix1", sr1)
	r.Mount("/prefix2", sr2)

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/root", nil); body != "root" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "PUT", "/root", nil); resp.StatusCode != 405 {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/prefix1/sub1", nil); body != "sub1" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "PUT", "/prefix1/sub1", nil); resp.StatusCode != 405 {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/prefix2/sub2", nil); body != "sub2" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "PUT", "/prefix2/sub2", nil); resp.StatusCode != 405 {
		t.Fatalf(body)
	}
}

func TestMuxComplicatedNotFound(t *testing.T) {
	// sub router with groups
	sub := NewRouter()
	sub.Route("/resource", func(r Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
			w.Write([]byte("private get"))
			return nil
		})
	})

	// Root router with groups
	r := NewRouter()
	r.Get("/auth", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("auth get"))
		return nil
	})
	r.Route("/public", func(r Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
			w.Write([]byte("public get"))
			return nil
		})
	})
	r.Mount("/private", sub)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("custom not-found"))
		return nil
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	// check that we didn't broke correct routes
	if _, body := testRequest(t, ts, "GET", "/auth", nil); body != "auth get" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/public", nil); body != "public get" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/private/resource", nil); body != "private get" {
		t.Fatalf(body)
	}
	// check custom not-found on all levels
	if _, body := testRequest(t, ts, "GET", "/nope", nil); body != "custom not-found" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/public/nope", nil); body != "custom not-found" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/private/nope", nil); body != "custom not-found" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/private/resource/nope", nil); body != "custom not-found" {
		t.Fatalf(body)
	}
	// check custom not-found on trailing slash routes
	if _, body := testRequest(t, ts, "GET", "/auth/", nil); body != "custom not-found" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/public/", nil); body != "custom not-found" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/private/", nil); body != "custom not-found" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/private/resource/", nil); body != "custom not-found" {
		t.Fatalf(body)
	}
}

func TestMuxWith(t *testing.T) {
	var cmwInit1, cmwHandler1 uint64
	var cmwInit2, cmwHandler2 uint64
	mw1 := func(next Handler) Handler {
		cmwInit1++
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			cmwHandler1++
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"inline1"}, "yes"))
			return next.ServeHTTP(w, r)
		})
	}
	mw2 := func(next Handler) Handler {
		cmwInit2++
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			cmwHandler2++
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"inline2"}, "yes"))
			return next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("bye"))
		return nil
	})
	r.With(mw1).With(mw2).Get("/inline", func(w http.ResponseWriter, r *http.Request) HandlerError {
		v1 := r.Context().Value(ctxKey{"inline1"}).(string)
		v2 := r.Context().Value(ctxKey{"inline2"}).(string)
		w.Write([]byte(fmt.Sprintf("inline %s %s", v1, v2)))
		return nil
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/inline", nil); body != "inline yes yes" {
		t.Fatalf(body)
	}
	if cmwInit1 != 1 {
		t.Fatalf("expecting cmwInit1 to be 1, got %d", cmwInit1)
	}
	if cmwHandler1 != 1 {
		t.Fatalf("expecting cmwHandler1 to be 1, got %d", cmwHandler1)
	}
	if cmwInit2 != 1 {
		t.Fatalf("expecting cmwInit2 to be 1, got %d", cmwInit2)
	}
	if cmwHandler2 != 1 {
		t.Fatalf("expecting cmwHandler2 to be 1, got %d", cmwHandler2)
	}
}

func TestRouterFromMuxWith(t *testing.T) {
	t.Parallel()

	r := NewRouter()

	with := r.With(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			return next.ServeHTTP(w, r)
		})
	})

	with.Get("/with_middleware", func(w http.ResponseWriter, r *http.Request) HandlerError {
		return nil
	})

	ts := httptest.NewServer(with.ToHTTPHandler())
	defer ts.Close()

	// Without the fix this test was committed with, this causes a panic.
	testRequest(t, ts, http.MethodGet, "/with_middleware", nil)
}

func TestMuxMiddlewareStack(t *testing.T) {
	var stdmwInit, stdmwHandler uint64
	stdmw := func(next Handler) Handler {
		stdmwInit++
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			stdmwHandler++
			return next.ServeHTTP(w, r)
		})
	}
	_ = stdmw

	var ctxmwInit, ctxmwHandler uint64
	ctxmw := func(next Handler) Handler {
		ctxmwInit++
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctxmwHandler++
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKey{"count.ctxmwHandler"}, ctxmwHandler)
			r = r.WithContext(ctx)
			return next.ServeHTTP(w, r)
		})
	}

	var inCtxmwInit, inCtxmwHandler uint64
	inCtxmw := func(next Handler) Handler {
		inCtxmwInit++
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			inCtxmwHandler++
			return next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Use(stdmw)
	r.Use(ctxmw)
	r.Use(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			if r.URL.Path == "/ping" {
				w.Write([]byte("pong"))
				return nil
			}
			return next.ServeHTTP(w, r)
		})
	})

	var handlerCount uint64

	r.With(inCtxmw).Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
		handlerCount++
		ctx := r.Context()
		ctxmwHandlerCount := ctx.Value(ctxKey{"count.ctxmwHandler"}).(uint64)
		w.Write([]byte(fmt.Sprintf("inits:%d reqs:%d ctxValue:%d", ctxmwInit, handlerCount, ctxmwHandlerCount)))
		return nil
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("wooot"))
		return nil
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	testRequest(t, ts, "GET", "/", nil)
	testRequest(t, ts, "GET", "/", nil)
	var body string
	_, body = testRequest(t, ts, "GET", "/", nil)
	if body != "inits:1 reqs:3 ctxValue:3" {
		t.Fatalf("got: '%s'", body)
	}

	_, body = testRequest(t, ts, "GET", "/ping", nil)
	if body != "pong" {
		t.Fatalf("got: '%s'", body)
	}
}

func TestMuxRouteGroups(t *testing.T) {
	var stdmwInit, stdmwHandler uint64

	stdmw := func(next Handler) Handler {
		stdmwInit++
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			stdmwHandler++
			return next.ServeHTTP(w, r)
		})
	}

	var stdmwInit2, stdmwHandler2 uint64
	stdmw2 := func(next Handler) Handler {
		stdmwInit2++
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			stdmwHandler2++
			return next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Group(func(r Router) {
		r.Use(stdmw)
		r.Get("/group", func(w http.ResponseWriter, r *http.Request) HandlerError {
			w.Write([]byte("root group"))
			return nil
		})
	})
	r.Group(func(r Router) {
		r.Use(stdmw2)
		r.Get("/group2", func(w http.ResponseWriter, r *http.Request) HandlerError {
			w.Write([]byte("root group2"))
			return nil
		})
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	// GET /group
	_, body := testRequest(t, ts, "GET", "/group", nil)
	if body != "root group" {
		t.Fatalf("got: '%s'", body)
	}
	if stdmwInit != 1 || stdmwHandler != 1 {
		t.Logf("stdmw counters failed, should be 1:1, got %d:%d", stdmwInit, stdmwHandler)
	}

	// GET /group2
	_, body = testRequest(t, ts, "GET", "/group2", nil)
	if body != "root group2" {
		t.Fatalf("got: '%s'", body)
	}
	if stdmwInit2 != 1 || stdmwHandler2 != 1 {
		t.Fatalf("stdmw2 counters failed, should be 1:1, got %d:%d", stdmwInit2, stdmwHandler2)
	}
}

func TestMuxBig(t *testing.T) {
	r := bigMux()

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	var body, expected string

	_, body = testRequest(t, ts, "GET", "/favicon.ico", nil)
	if body != "fav" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/4/view", nil)
	if body != "/hubs/4/view reqid:1 session:anonymous" {
		t.Fatalf("got '%v'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/4/view/index.html", nil)
	if body != "/hubs/4/view/index.html reqid:1 session:anonymous" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "POST", "/hubs/ethereumhub/view/index.html", nil)
	if body != "/hubs/ethereumhub/view/index.html reqid:1 session:anonymous" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/", nil)
	if body != "/ reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/suggestions", nil)
	if body != "/suggestions reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/woot/444/hiiii", nil)
	if body != "/woot/444/hiiii" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123", nil)
	expected = "/hubs/123 reqid:1 session:elvis"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/touch", nil)
	if body != "/hubs/123/touch reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/webhooks", nil)
	if body != "/hubs/123/webhooks reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/posts", nil)
	if body != "/hubs/123/posts reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders", nil)
	if body != fmt.Sprintln(http.StatusText(404)) {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/", nil)
	if body != "/folders/ reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/public", nil)
	if body != "/folders/public reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/nothing", nil)
	if body != fmt.Sprintln(http.StatusText(404)) {
		t.Fatalf("got '%s'", body)
	}
}

func bigMux() Router {
	var r, sr1, sr2, sr3, sr4, sr5, sr6 *Mux
	r = NewRouter()
	r.Use(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := context.WithValue(r.Context(), ctxKey{"requestID"}, "1")
			return next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Use(func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			return next.ServeHTTP(w, r)
		})
	})
	r.Group(func(r Router) {
		r.Use(func(next Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
				ctx := context.WithValue(r.Context(), ctxKey{"session.user"}, "anonymous")
				return next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) HandlerError {
			w.Write([]byte("fav"))
			return nil
		})
		r.Get("/hubs/{hubID}/view", func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view reqid:%s session:%s", URLParam(r, "hubID"),
				ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
			return nil
		})
		r.Get("/hubs/{hubID}/view/*", func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view/%s reqid:%s session:%s", URLParamFromCtx(ctx, "hubID"),
				URLParam(r, "*"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
			return nil
		})
		r.Post("/hubs/{hubSlug}/view/*", func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view/%s reqid:%s session:%s", URLParamFromCtx(ctx, "hubSlug"),
				URLParam(r, "*"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
			return nil
		})
	})
	r.Group(func(r Router) {
		r.Use(func(next Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
				ctx := context.WithValue(r.Context(), ctxKey{"session.user"}, "elvis")
				return next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := r.Context()
			s := fmt.Sprintf("/ reqid:%s session:%s", ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
			return nil
		})
		r.Get("/suggestions", func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := r.Context()
			s := fmt.Sprintf("/suggestions reqid:%s session:%s", ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
			return nil
		})

		r.Get("/woot/{wootID}/*", func(w http.ResponseWriter, r *http.Request) HandlerError {
			s := fmt.Sprintf("/woot/%s/%s", URLParam(r, "wootID"), URLParam(r, "*"))
			w.Write([]byte(s))
			return nil
		})

		r.Route("/hubs", func(r Router) {
			sr1 = r.(*Mux)
			r.Route("/{hubID}", func(r Router) {
				sr2 = r.(*Mux)
				r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s reqid:%s session:%s",
						URLParam(r, "hubID"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
					return nil
				})
				r.Get("/touch", func(w http.ResponseWriter, r *http.Request) HandlerError {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s/touch reqid:%s session:%s", URLParam(r, "hubID"),
						ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
					return nil
				})

				sr3 = NewRouter()
				sr3.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s/webhooks reqid:%s session:%s", URLParam(r, "hubID"),
						ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
					return nil
				})
				sr3.Route("/{webhookID}", func(r Router) {
					sr4 = r.(*Mux)
					r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
						ctx := r.Context()
						s := fmt.Sprintf("/hubs/%s/webhooks/%s reqid:%s session:%s", URLParam(r, "hubID"),
							URLParam(r, "webhookID"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
						w.Write([]byte(s))
						return nil
					})
				})

				r.Mount("/webhooks", Chain(func(next Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
						return next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{"hook"}, true)))
					})
				}).Handler(sr3))

				r.Route("/posts", func(r Router) {
					sr5 = r.(*Mux)
					r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
						ctx := r.Context()
						s := fmt.Sprintf("/hubs/%s/posts reqid:%s session:%s", URLParam(r, "hubID"),
							ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
						w.Write([]byte(s))
						return nil
					})
				})
			})
		})

		r.Route("/folders/", func(r Router) {
			sr6 = r.(*Mux)
			r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
				ctx := r.Context()
				s := fmt.Sprintf("/folders/ reqid:%s session:%s",
					ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
				w.Write([]byte(s))
				return nil
			})
			r.Get("/public", func(w http.ResponseWriter, r *http.Request) HandlerError {
				ctx := r.Context()
				s := fmt.Sprintf("/folders/public reqid:%s session:%s",
					ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
				w.Write([]byte(s))
				return nil
			})
		})
	})

	return r
}

func TestMuxSubroutesBasic(t *testing.T) {
	hIndex := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("index"))
		return nil
	})
	hArticlesList := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("articles-list"))
		return nil
	})
	hSearchArticles := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("search-articles"))
		return nil
	})
	hGetArticle := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte(fmt.Sprintf("get-article:%s", URLParam(r, "id"))))
		return nil
	})
	hSyncArticle := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte(fmt.Sprintf("sync-article:%s", URLParam(r, "id"))))
		return nil
	})

	r := NewRouter()
	var rr1, rr2 *Mux
	r.Get("/", hIndex)
	r.Route("/articles", func(r Router) {
		rr1 = r.(*Mux)
		r.Get("/", hArticlesList)
		r.Get("/search", hSearchArticles)
		r.Route("/{id}", func(r Router) {
			rr2 = r.(*Mux)
			r.Get("/", hGetArticle)
			r.Get("/sync", hSyncArticle)
		})
	})

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, r.tree, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, rr1.tree, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, rr2.tree, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	var body, expected string

	_, body = testRequest(t, ts, "GET", "/", nil)
	expected = "index"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/articles", nil)
	expected = "articles-list"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/articles/search", nil)
	expected = "search-articles"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/articles/123", nil)
	expected = "get-article:123"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/articles/123/sync", nil)
	expected = "sync-article:123"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
}

func TestMuxSubroutes(t *testing.T) {
	hHubView1 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("hub1"))
		return nil
	})
	hHubView2 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("hub2"))
		return nil
	})
	hHubView3 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("hub3"))
		return nil
	})
	hAccountView1 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("account1"))
		return nil
	})
	hAccountView2 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("account2"))
		return nil
	})

	r := NewRouter()
	r.Get("/hubs/{hubID}/view", hHubView1)
	r.Get("/hubs/{hubID}/view/*", hHubView2)

	sr := NewRouter()
	sr.Get("/", hHubView3)
	r.Mount("/hubs/{hubID}/users", sr)

	sr3 := NewRouter()
	sr3.Get("/", hAccountView1)
	sr3.Get("/hi", hAccountView2)

	var sr2 *Mux
	r.Route("/accounts/{accountID}", func(r Router) {
		sr2 = r.(*Mux)
		// r.Get("/", hAccountView1)
		r.Mount("/", sr3)
	})

	// This is the same as the r.Route() call mounted on sr2
	// sr2 := NewRouter()
	// sr2.Mount("/", sr3)
	// r.Mount("/accounts/{accountID}", sr2)

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	var body, expected string
	var resp *http.Response

	_, body = testRequest(t, ts, "GET", "/hubs/123/view", nil)
	expected = "hub1"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/view/index.html", nil)
	expected = "hub2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/users", nil)
	expected = "hub3"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	resp, body = testRequest(t, ts, "GET", "/hubs/123/users/", nil)
	expected = fmt.Sprintln(http.StatusText(404))
	if resp.StatusCode != 404 || body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/accounts/44", nil)
	expected = "account1"
	if body != expected {
		t.Fatalf("request:%s expected:%s got:%s", "GET /accounts/44", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/accounts/44/hi", nil)
	expected = "account2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	// Test that we're building the routingPatterns properly
	router := r.ToHTTPHandler()
	req, _ := http.NewRequest("GET", "/accounts/44/hi", nil)

	rctx := NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body = string(w.Body.Bytes())
	expected = "account2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	routePatterns := rctx.RoutePatterns
	if len(rctx.RoutePatterns) != 3 {
		t.Fatalf("expected 3 routing patterns, got:%d", len(rctx.RoutePatterns))
	}
	expected = "/accounts/{accountID}/*"
	if routePatterns[0] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[0])
	}
	expected = "/*"
	if routePatterns[1] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[1])
	}
	expected = "/hi"
	if routePatterns[2] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[2])
	}

}

func TestSingleHandler(t *testing.T) {
	h := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		name := URLParam(r, "name")
		w.Write([]byte("hi " + name))
		return nil
	})

	r, _ := http.NewRequest("GET", "/", nil)
	rctx := NewRouteContext()
	r = r.WithContext(context.WithValue(r.Context(), RouteCtxKey, rctx))
	rctx.URLParams.Add("name", "joe")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	body := string(w.Body.Bytes())
	expected := "hi joe"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
}

// TODO: a Router wrapper test..
//
// type ACLMux struct {
// 	*Mux
// 	XX string
// }
//
// func NewACLMux() *ACLMux {
// 	return &ACLMux{Mux: NewRouter(), XX: "hihi"}
// }
//
// // TODO: this should be supported...
// func TestWoot(t *testing.T) {
// 	var r Router = NewRouter()
//
// 	var r2 Router = NewACLMux() //NewRouter()
// 	r2.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
// 		w.Write([]byte("hi"))
// 	})
//
// 	r.Mount("/", r2)
// }

func TestServeHTTPExistingContext(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) HandlerError {
		s, _ := r.Context().Value(ctxKey{"testCtx"}).(string)
		w.Write([]byte(s))
		return nil
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) HandlerError {
		s, _ := r.Context().Value(ctxKey{"testCtx"}).(string)
		return Error{
			Code: 404,
			Err:  errors.New(s),
		}
	})

	testcases := []struct {
		Method         string
		Path           string
		Ctx            context.Context
		ExpectedStatus int
		ExpectedBody   string
	}{
		{
			Method:         "GET",
			Path:           "/hi",
			Ctx:            context.WithValue(context.Background(), ctxKey{"testCtx"}, "hi ctx"),
			ExpectedStatus: 200,
			ExpectedBody:   "hi ctx",
		},
		{
			Method:         "GET",
			Path:           "/hello",
			Ctx:            context.WithValue(context.Background(), ctxKey{"testCtx"}, "nothing here ctx"),
			ExpectedStatus: 404,
			ExpectedBody:   "nothing here ctx",
		},
	}

	for _, tc := range testcases {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest(tc.Method, tc.Path, nil)
		if err != nil {
			t.Fatalf("%v", err)
		}
		req = req.WithContext(tc.Ctx)

		var code int
		var body string
		herr := r.ServeHTTP(resp, req)
		if herr != nil {
			code = herr.StatusCode()
			body = herr.Error()
		} else {
			code = resp.Code
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("%v", err)
			} else {
				body = string(b)
			}
		}

		if code != tc.ExpectedStatus {
			t.Fatalf("%v != %v", tc.ExpectedStatus, resp.Code)
		}
		if body != tc.ExpectedBody {
			t.Fatalf("%s != %s", tc.ExpectedBody, body)
		}
	}
}

func TestNestedGroups(t *testing.T) {
	handlerPrintCounter := func(w http.ResponseWriter, r *http.Request) HandlerError {
		counter, _ := r.Context().Value(ctxKey{"counter"}).(int)
		w.Write([]byte(fmt.Sprintf("%v", counter)))
		return nil
	}

	mwIncreaseCounter := func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			ctx := r.Context()
			counter, _ := ctx.Value(ctxKey{"counter"}).(int)
			counter++
			ctx = context.WithValue(ctx, ctxKey{"counter"}, counter)
			return next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Each route represents value of its counter (number of applied middlewares).
	r := NewRouter() // counter == 0
	r.Get("/0", handlerPrintCounter)
	r.Group(func(r Router) {
		r.Use(mwIncreaseCounter) // counter == 1
		r.Get("/1", handlerPrintCounter)

		// r.Handle(GET, "/2", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
		r.With(mwIncreaseCounter).Get("/2", handlerPrintCounter)

		r.Group(func(r Router) {
			r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 3
			r.Get("/3", handlerPrintCounter)
		})
		r.Route("/", func(r Router) {
			r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 3

			// r.Handle(GET, "/4", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
			r.With(mwIncreaseCounter).Get("/4", handlerPrintCounter)

			r.Group(func(r Router) {
				r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 5
				r.Get("/5", handlerPrintCounter)
				// r.Handle(GET, "/6", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
				r.With(mwIncreaseCounter).Get("/6", handlerPrintCounter)

			})
		})
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	for _, route := range []string{"0", "1", "2", "3", "4", "5", "6"} {
		if _, body := testRequest(t, ts, "GET", "/"+route, nil); body != route {
			t.Errorf("expected %v, got %v", route, body)
		}
	}
}

func TestMiddlewarePanicOnLateUse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("hello\n"))
		return nil
	}

	mw := func(next Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
			return next.ServeHTTP(w, r)
		})
	}

	defer func() {
		if recover() == nil {
			t.Error("expected panic()")
		}
	}()

	r := NewRouter()
	r.Get("/", handler)
	r.Use(mw) // Too late to apply middleware, we're expecting panic().
}

func TestMountingExistingPath(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) HandlerError {
		return nil
	}

	defer func() {
		if recover() == nil {
			t.Error("expected panic()")
		}
	}()

	r := NewRouter()
	r.Get("/", handler)
	r.Mount("/hi", HandlerFunc(handler))
	r.Mount("/hi", HandlerFunc(handler))
}

func TestMountingSimilarPattern(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("bye"))
		return nil
	})

	r2 := NewRouter()
	r2.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("foobar"))
		return nil
	})

	r3 := NewRouter()
	r3.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Write([]byte("foo"))
		return nil
	})

	r.Mount("/foobar", r2)
	r.Mount("/foo", r3)

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatalf(body)
	}
}

func TestMuxMissingParams(t *testing.T) {
	r := NewRouter()
	r.Get(`/user/{userId:\d+}`, func(w http.ResponseWriter, r *http.Request) HandlerError {
		userID := URLParam(r, "userId")
		w.Write([]byte(fmt.Sprintf("userId = '%s'", userID)))
		return nil
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return Error{Code: 404}
	})

	ts := httptest.NewServer(r.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/user/123", nil); body != "userId = '123'" {
		t.Fatalf(body)
	}
	if resp, body := testRequest(t, ts, "GET", "/user/", nil); resp.StatusCode != 404 {
		t.Fatalf(body)
	}
}

func TestMuxContextIsThreadSafe(t *testing.T) {
	router := NewRouter()
	router.Get("/{id}", func(w http.ResponseWriter, r *http.Request) HandlerError {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Millisecond)
		defer cancel()

		<-ctx.Done()
		return nil
	})

	wg := sync.WaitGroup{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10000; j++ {
				w := httptest.NewRecorder()
				r, err := http.NewRequest("GET", "/ok", nil)
				if err != nil {
					t.Fatal(err)
				}

				ctx, cancel := context.WithCancel(r.Context())
				r = r.WithContext(ctx)

				go func() {
					cancel()
				}()
				router.ServeHTTP(w, r)
			}
		}()
	}
	wg.Wait()
}

func TestEscapedURLParams(t *testing.T) {
	m := NewRouter()
	m.Get("/api/{identifier}/{region}/{size}/{rotation}/*", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.WriteHeader(200)
		rctx := RouteContext(r.Context())
		if rctx == nil {
			t.Error("no context")
			return nil
		}
		identifier := URLParam(r, "identifier")
		if identifier != "http:%2f%2fexample.com%2fimage.png" {
			t.Errorf("identifier path parameter incorrect %s", identifier)
			return nil
		}
		region := URLParam(r, "region")
		if region != "full" {
			t.Errorf("region path parameter incorrect %s", region)
			return nil
		}
		size := URLParam(r, "size")
		if size != "max" {
			t.Errorf("size path parameter incorrect %s", size)
			return nil
		}
		rotation := URLParam(r, "rotation")
		if rotation != "0" {
			t.Errorf("rotation path parameter incorrect %s", rotation)
			return nil
		}
		w.Write([]byte("success"))
		return nil
	})

	ts := httptest.NewServer(m.ToHTTPHandler())
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/api/http:%2f%2fexample.com%2fimage.png/full/max/0/color.png", nil); body != "success" {
		t.Fatalf(body)
	}
}

func TestMuxMatch(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) HandlerError {
		w.Header().Set("X-Test", "yes")
		w.Write([]byte("bye"))
		return nil
	})
	r.Route("/articles", func(r Router) {
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) HandlerError {
			id := URLParam(r, "id")
			w.Header().Set("X-Article", id)
			w.Write([]byte("article:" + id))
			return nil
		})
	})
	r.Route("/users", func(r Router) {
		r.Head("/{id}", func(w http.ResponseWriter, r *http.Request) HandlerError {
			w.Header().Set("X-User", "-")
			w.Write([]byte("user"))
			return nil
		})
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) HandlerError {
			id := URLParam(r, "id")
			w.Header().Set("X-User", id)
			w.Write([]byte("user:" + id))
			return nil
		})
	})

	tctx := NewRouteContext()

	tctx.Reset()
	if r.Match(tctx, "GET", "/users/1") == false {
		t.Fatal("expecting to find match for route:", "GET", "/users/1")
	}

	tctx.Reset()
	if r.Match(tctx, "HEAD", "/articles/10") == true {
		t.Fatal("not expecting to find match for route:", "HEAD", "/articles/10")
	}
}

func TestServerBaseContext(t *testing.T) {
	r := NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) HandlerError {
		baseYes := r.Context().Value(ctxKey{"base"}).(string)
		if _, ok := r.Context().Value(http.ServerContextKey).(*http.Server); !ok {
			panic("missing server context")
		}
		if _, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); !ok {
			panic("missing local addr context")
		}
		w.Write([]byte(baseYes))
		return nil
	})

	// Setup http Server with a base context
	ctx := context.WithValue(context.Background(), ctxKey{"base"}, "yes")
	ts := httptest.NewServer(ServerBaseContext(ctx, r.ToHTTPHandler()))
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/", nil); body != "yes" {
		t.Fatalf(body)
	}
}

func testRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}
	defer resp.Body.Close()

	return resp, string(respBody)
}

func testHandler(t *testing.T, h Handler, method, path string, body io.Reader) (*http.Response, string, HandlerError) {
	r, _ := http.NewRequest(method, path, body)
	w := httptest.NewRecorder()
	err := h.ServeHTTP(w, r)
	return w.Result(), string(w.Body.Bytes()), err
}

type testFileSystem struct {
	open func(name string) (http.File, error)
}

func (fs *testFileSystem) Open(name string) (http.File, error) {
	return fs.open(name)
}

type testFile struct {
	name     string
	contents []byte
}

func (tf *testFile) Close() error {
	return nil
}

func (tf *testFile) Read(p []byte) (n int, err error) {
	copy(p, tf.contents)
	return len(p), nil
}

func (tf *testFile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (tf *testFile) Readdir(count int) ([]os.FileInfo, error) {
	stat, _ := tf.Stat()
	return []os.FileInfo{stat}, nil
}

func (tf *testFile) Stat() (os.FileInfo, error) {
	return &testFileInfo{tf.name, int64(len(tf.contents))}, nil
}

type testFileInfo struct {
	name string
	size int64
}

func (tfi *testFileInfo) Name() string       { return tfi.name }
func (tfi *testFileInfo) Size() int64        { return tfi.size }
func (tfi *testFileInfo) Mode() os.FileMode  { return 0755 }
func (tfi *testFileInfo) ModTime() time.Time { return time.Now() }
func (tfi *testFileInfo) IsDir() bool        { return false }
func (tfi *testFileInfo) Sys() interface{}   { return nil }

type ctxKey struct {
	name string
}

func (k ctxKey) String() string {
	return "context value " + k.name
}

func BenchmarkMux(b *testing.B) {
	h1 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return nil
	})
	h2 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return nil
	})
	h3 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return nil
	})
	h4 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return nil
	})
	h5 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return nil
	})
	h6 := HandlerFunc(func(w http.ResponseWriter, r *http.Request) HandlerError {
		return nil
	})

	mx := NewRouter()
	mx.Get("/", h1)
	mx.Get("/hi", h2)
	mx.Get("/sup/{id}/and/{this}", h3)

	mx.Route("/sharing/{hash}", func(mx Router) {
		mx.Get("/", h4)          // subrouter-1
		mx.Get("/{network}", h5) // subrouter-1
		mx.Get("/twitter", h5)
		mx.Route("/direct", func(mx Router) {
			mx.Get("/", h6) // subrouter-2
		})
	})

	routes := []string{
		"/",
		"/sup/123/and/this",
		"/sharing/aBc",         // subrouter-1
		"/sharing/aBc/twitter", // subrouter-1
		"/sharing/aBc/direct",  // subrouter-2
	}

	for _, path := range routes {
		b.Run("route:"+path, func(b *testing.B) {
			w := httptest.NewRecorder()
			r, _ := http.NewRequest("GET", path, nil)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				mx.ServeHTTP(w, r)
			}
		})
	}
}
