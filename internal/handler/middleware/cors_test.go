package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	for _, tc := range []struct {
		name, origin, method string
		code                 int
		called               bool
	}{
		{"allowed browser", "http://localhost:3080", "GET", 200, true},
		{"preflight", "http://localhost:3080", "OPTIONS", 204, false},
		{"blocked browser", "https://untrusted.example", "POST", 403, false},
		{"direct API client", "", "GET", 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := CORS([]string{"http://localhost:3080"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) }))
			request := httptest.NewRequest(tc.method, "/api/formats", nil)
			request.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, request)
			if w.Code != tc.code || called != tc.called {
				t.Fatalf("code=%d called=%v", w.Code, called)
			}
			if tc.origin == "http://localhost:3080" {
				if w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
					t.Fatal("missing allowed origin")
				}
				if w.Header().Get("Access-Control-Expose-Headers") != "Content-Disposition" {
					t.Fatal("download filename is hidden from frontend")
				}
			}
		})
	}
}
