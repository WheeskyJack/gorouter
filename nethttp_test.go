package gorouter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/vardius/gorouter/v4/context"
)

func TestInterface(t *testing.T) {
	var _ http.Handler = New()
}

func TestHandle(t *testing.T) {
	t.Parallel()

	handler := &mockHandler{}
	router := New().(*router)
	router.Handle(http.MethodPost, "/x/y", handler)

	checkIfHasRootRoute(t, router, http.MethodPost)

	err := mockServeHTTP(router, http.MethodPost, "/x/y")
	if err != nil {
		t.Fatal(err)
	}

	if handler.served != true {
		t.Error("Handler has not been served")
	}
}

func TestOPTIONSHeaders(t *testing.T) {
	handler := &mockHandler{}
	router := New().(*router)

	router.GET("/x/y", handler)
	router.POST("/x/y", handler)

	checkIfHasRootRoute(t, router, http.MethodGet)

	w := httptest.NewRecorder()

	// test all tree "*" paths
	req, err := http.NewRequest(http.MethodOptions, "*", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if allow := w.Header().Get("Allow"); !strings.Contains(allow, "POST") || !strings.Contains(allow, "GET") || !strings.Contains(allow, "OPTIONS") {
		t.Errorf("Allow header incorrect value: %s", allow)
	}

	// test specific path
	req, err = http.NewRequest(http.MethodOptions, "/x/y", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if allow := w.Header().Get("Allow"); !strings.Contains(allow, "POST") || !strings.Contains(allow, "GET") || !strings.Contains(allow, "OPTIONS") {
		t.Errorf("Allow header incorrect value: %s", allow)
	}
}

func TestMethods(t *testing.T) {
	t.Parallel()

	for _, method := range []string{
		http.MethodPost,
		http.MethodGet,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodConnect,
		http.MethodTrace,
		http.MethodOptions,
	} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			handler := &mockHandler{}
			router := New().(*router)

			reflect.ValueOf(router).MethodByName(method).Call([]reflect.Value{reflect.ValueOf("/x/y"), reflect.ValueOf(handler)})

			checkIfHasRootRoute(t, router, method)

			err := mockServeHTTP(router, method, "/x/y")
			if err != nil {
				t.Fatal(err)
			}

			if handler.served != true {
				t.Error("Handler has not been served")
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	t.Parallel()

	handler := &mockHandler{}
	router := New().(*router)
	router.GET("/x", handler)
	router.GET("/x/y", handler)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/x/x", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("NotFound error, actual code: %d", w.Code)
	}

	router.NotFound(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("test")); err != nil {
			t.Fatal(err)
		}
	}))

	if router.notFound == nil {
		t.Error("NotFound handler error")
	}

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Body.String() != "test" {
		t.Error("Not found handler wasn't invoked")
	}
}

func TestNotAllowed(t *testing.T) {
	t.Parallel()

	handler := &mockHandler{}
	router := New().(*router)
	router.GET("/x/y", handler)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/x/y", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Error("NotAllowed doesnt work")
	}

	router.NotAllowed(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("test")); err != nil {
			t.Fatal(err)
		}
	}))

	if router.notAllowed == nil {
		t.Error("NotAllowed handler error")
	}

	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Body.String() != "test" {
		t.Error("NotAllowed handler wasn't invoked")
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest(http.MethodPost, "*", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Body.String() != "test" {
		t.Error("NotAllowed handler wasn't invoked")
	}
}

func TestParam(t *testing.T) {
	t.Parallel()

	router := New().(*router)

	served := false
	router.GET("/x/{param}", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		served = true

		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if params.Value("param") != "y" {
			t.Errorf("Wrong params value. Expected 'y', actual '%s'", params.Value("param"))
		}
	}))

	err := mockServeHTTP(router, http.MethodGet, "/x/y")
	if err != nil {
		t.Fatal(err)
	}

	if served != true {
		t.Error("Handler has not been served")
	}
}

func TestRegexpParam(t *testing.T) {
	t.Parallel()

	router := New().(*router)

	served := false
	router.GET("/x/{param:r([a-z]+)go}", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		served = true

		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if params.Value("param") != "rxgo" {
			t.Errorf("Wrong params value. Expected 'rxgo', actual '%s'", params.Value("param"))
		}
	}))

	err := mockServeHTTP(router, http.MethodGet, "/x/rxgo")
	if err != nil {
		t.Fatal(err)
	}

	if served != true {
		t.Error("Handler has not been served")
	}
}

func TestEmptyParam(t *testing.T) {
	t.Parallel()

	panicked := false
	defer func() {
		if rcv := recover(); rcv != nil {
			panicked = true
		}
	}()

	handler := &mockHandler{}
	router := New().(*router)

	router.GET("/x/{}", handler)

	if panicked != true {
		t.Error("Router should panic for empty wildcard path")
	}
}

func TestServeFiles(t *testing.T) {
	t.Parallel()

	mfs := &mockFileSystem{}
	router := New().(*router)

	router.ServeFiles(mfs, "static", true)

	if router.fileServer == nil {
		t.Error("File serve handler error")
	}

	w := httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodGet, "/favicon.ico", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Error("File should not exist")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Router should panic for empty path")
		}
	}()

	router.ServeFiles(mfs, "", true)
}

func TestNilMiddleware(t *testing.T) {
	t.Parallel()

	router := New().(*router)

	router.GET("/x/{param}", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("test")); err != nil {
			t.Fatal(err)
		}
	}))

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/x/y", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Body.String() != "test" {
		t.Error("Nil middleware works")
	}
}

func TestPanicMiddleware(t *testing.T) {
	t.Parallel()

	panicked := false
	panicMiddleware := func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rcv := recover(); rcv != nil {
					panicked = true
				}
			}()

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}

	router := New(panicMiddleware).(*router)

	router.GET("/x/{param}", http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic recover")
	}))

	err := mockServeHTTP(router, http.MethodGet, "/x/y")
	if err != nil {
		t.Fatal(err)
	}

	if panicked != true {
		t.Error("Panic has not been handled")
	}
}

func TestNodeApplyMiddleware(t *testing.T) {
	t.Parallel()

	router := New().(*router)

	router.GET("/x/{param}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if _, err := w.Write([]byte(params.Value("param"))); err != nil {
			t.Fatal(err)
		}
	}))

	router.USE(http.MethodGet, "/x/{param}", mockMiddleware("m1"))
	router.USE(http.MethodGet, "/x/x", mockMiddleware("m2"))

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/x/y", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Body.String() != "m1y" {
		t.Errorf("Use middleware error %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest(http.MethodGet, "/x/x", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Body.String() != "m1m2x" {
		t.Errorf("Use middleware error %s", w.Body.String())
	}
}

func TestTreeOrphanMiddlewareOrder(t *testing.T) {
	t.Parallel()

	router := New().(*router)

	router.GET("/x/{param}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("handler")); err != nil {
			t.Fatal(err)
		}
	}))

	// Method global middleware
	router.USE(http.MethodGet, "/", mockMiddleware("m1->"))
	router.USE(http.MethodGet, "/", mockMiddleware("m2->"))
	// Path middleware
	router.USE(http.MethodGet, "/x", mockMiddleware("mx1->"))
	router.USE(http.MethodGet, "/x", mockMiddleware("mx2->"))
	router.USE(http.MethodGet, "/x/y", mockMiddleware("mxy1->"))
	router.USE(http.MethodGet, "/x/y", mockMiddleware("mxy2->"))
	router.USE(http.MethodGet, "/x/{param}", mockMiddleware("mparam1->"))
	router.USE(http.MethodGet, "/x/{param}", mockMiddleware("mparam2->"))
	router.USE(http.MethodGet, "/x/y", mockMiddleware("mxy3->"))
	router.USE(http.MethodGet, "/x/y", mockMiddleware("mxy4->"))

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/x/y", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Body.String() != "m1->m2->mx1->mx2->mxy1->mxy2->mparam1->mparam2->mxy3->mxy4->handler" {
		t.Errorf("Use middleware error %s", w.Body.String())
	}
}

func TestNodeApplyMiddlewareStatic(t *testing.T) {
	t.Parallel()

	router := New().(*router)

	router.GET("/x/{param}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}))

	router.USE(http.MethodGet, "/x/x", mockMiddleware("m1"))

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/x/x", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Body.String() != "m1x" {
		t.Errorf("Use middleware error %s", w.Body.String())
	}
}

func TestNodeApplyMiddlewareInvalidNodeReference(t *testing.T) {
	t.Parallel()

	router := New().(*router)

	router.GET("/x/{param}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if _, err := w.Write([]byte(params.Value("param"))); err != nil {
			t.Fatal(err)
		}
	}))

	router.USE(http.MethodGet, "/x/x", mockMiddleware("m1"))

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/x/y", nil)
	if err != nil {
		t.Fatal(err)
	}

	router.ServeHTTP(w, req)

	if w.Body.String() != "y" {
		t.Errorf("Use middleware error %s", w.Body.String())
	}
}

func TestChainCalls(t *testing.T) {
	t.Parallel()

	router := New().(*router)

	served := false
	router.GET("/users/{user:[a-z0-9]+}/starred", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		served = true

		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if params.Value("user") != "x" {
			t.Errorf("Wrong params value. Expected 'x', actual '%s'", params.Value("user"))
		}
	}))

	router.GET("/applications/{client_id}/tokens", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		served = true

		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if params.Value("client_id") != "client_id" {
			t.Errorf("Wrong params value. Expected 'client_id', actual '%s'", params.Value("client_id"))
		}
	}))

	router.GET("/applications/{client_id}/tokens/{access_token}", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		served = true

		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if params.Value("client_id") != "client_id" {
			t.Errorf("Wrong params value. Expected 'client_id', actual '%s'", params.Value("client_id"))
		}

		if params.Value("access_token") != "access_token" {
			t.Errorf("Wrong params value. Expected 'access_token', actual '%s'", params.Value("access_token"))
		}
	}))

	router.GET("/users/{user}/received_events", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		served = true

		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if params.Value("user") != "user1" {
			t.Errorf("Wrong params value. Expected 'user1', actual '%s'", params.Value("user"))
		}
	}))

	router.GET("/users/{user}/received_events/public", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		served = true

		params, ok := context.Parameters(r.Context())
		if !ok {
			t.Fatal("Error while reading param")
		}

		if params.Value("user") != "user2" {
			t.Errorf("Wrong params value. Expected 'user2', actual '%s'", params.Value("user"))
		}
	}))

	// //FIRST CALL
	err := mockServeHTTP(router, http.MethodGet, "/users/x/starred")
	if err != nil {
		t.Fatal(err)
	}

	if !served {
		t.Fatal("First not served")
	}

	// SECOND CALL
	served = false
	err = mockServeHTTP(router, http.MethodGet, "/applications/client_id/tokens")
	if err != nil {
		t.Fatal(err)
	}

	if !served {
		t.Fatal("Second not served")
	}

	// THIRD CALL
	served = false
	err = mockServeHTTP(router, http.MethodGet, "/applications/client_id/tokens/access_token")
	if err != nil {
		t.Fatal(err)
	}

	if !served {
		t.Fatal("Third not served")
	}

	// FOURTH CALL
	served = false
	err = mockServeHTTP(router, http.MethodGet, "/users/user1/received_events")
	if err != nil {
		t.Fatal(err)
	}

	if !served {
		t.Fatal("Fourth not served")
	}

	// FIFTH CALL
	served = false
	err = mockServeHTTP(router, http.MethodGet, "/users/user2/received_events/public")
	if err != nil {
		t.Fatal(err)
	}

	if !served {
		t.Fatal("Fifth not served")
	}
}

func TestMountSubRouter(t *testing.T) {
	t.Parallel()

	mainRouter := New(
		mockMiddleware("[rg1]"),
		mockMiddleware("[rg2]"),
	).(*router)

	subRouter := New(
		mockMiddleware("[sg1]"),
		mockMiddleware("[sg2]"),
	).(*router)

	subRouter.GET("/y", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("[s]")); err != nil {
			t.Fatal(err)
		}
	}))

	mainRouter.Mount("/{param}", subRouter)

	mainRouter.USE(http.MethodGet, "/{param}", mockMiddleware("[r1]"))
	mainRouter.USE(http.MethodGet, "/{param}", mockMiddleware("[r2]"))

	subRouter.USE(http.MethodGet, "/y", mockMiddleware("[s1]"))
	subRouter.USE(http.MethodGet, "/y", mockMiddleware("[s2]"))

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/x/y", nil)
	if err != nil {
		t.Fatal(err)
	}

	mainRouter.ServeHTTP(w, req)

	if w.Body.String() != "[rg1][rg2][r1][r2][sg1][sg2][s1][s2][s]" {
		t.Errorf("Router mount subrouter middleware error: %s", w.Body.String())
	}
}

func TestMountSubRouter_second(t *testing.T) {
	t.Parallel()

	mainRouter := New().(*router)
	subRouter := New().(*router)

	subRouter.GET("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("/")); err != nil {
			t.Fatal(err)
		}
	}))
	subRouter.GET("/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("/id")); err != nil {
			t.Fatal(err)
		}
	}))

	mainRouter.Mount("/v1", subRouter)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1", nil)
	if err != nil {
		t.Fatal(err)
	}

	mainRouter.ServeHTTP(w, req)

	if w.Body.String() != "/" {
		t.Errorf("Router mount subrouter middleware error: %s", w.Body.String())
	}
}

func TestMountSubRouter_third(t *testing.T) {
	t.Parallel()

	subRouter := New().(*router)

	subRouter.GET("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "GET[/]") }))
	subRouter.GET("/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "GET[/{id}]") }))
	subRouter.GET("/me", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "GET[/me]") }))
	subRouter.USE(http.MethodGet, "/me", func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, "USE[/me]")
			h.ServeHTTP(w, r)
		})
	})
	subRouter.POST("/google/callback", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "POST[/google/callback]") }))
	subRouter.POST("/facebook/callback", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "POST[/facebook/callback]") }))
	subRouter.POST("/dispatch/{command}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "POST[/dispatch/{command}]") }))
	subRouter.USE(http.MethodPost, "/dispatch/some-else", func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, "USE[/dispatch/some-else]")
			h.ServeHTTP(w, r)
		})
	})

	mainRouter := New().(*router)

	mainRouter.Mount("/v1", subRouter)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1", nil)
	if err != nil {
		t.Fatal(err)
	}

	mainRouter.ServeHTTP(w, req)

	if w.Body.String() != "GET[/]" {
		t.Errorf("subrouter route did not match: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest(http.MethodPost, "/v1/dispatch/some-else", nil)
	if err != nil {
		t.Fatal(err)
	}

	mainRouter.ServeHTTP(w, req)

	if w.Body.String() != "USE[/dispatch/some-else]POST[/dispatch/{command}]" {
		t.Errorf("subrouter route did not match: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest(http.MethodGet, "/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}

	mainRouter.ServeHTTP(w, req)

	if w.Body.String() != "USE[/me]GET[/me]" {
		t.Errorf("subrouter route did not match: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest(http.MethodGet, "/v1", nil)
	if err != nil {
		t.Fatal(err)
	}

	mainRouter.ServeHTTP(w, req)

	if w.Body.String() != "GET[/]" {
		t.Errorf("subrouter route did not match: %s", w.Body.String())
	}
}
