package conversion

import (
	"bytes"
	"converter/internal/infrastructure/converter"
	"converter/internal/repository/files"
	service "converter/internal/service/conversion"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func request(t *testing.T, name, target string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
	mw.WriteField("format", target)
	mw.Close()
	req := httptest.NewRequest("POST", "/api/convert", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	s := testHandler(t, 2)
	router(s).ServeHTTP(w, req)
	return w
}
func TestCSVToJSON(t *testing.T) {
	w := request(t, "тест.csv", "json", []byte("name,age\nИван,25\n"))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var rows []map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Иван" {
		t.Fatal(rows)
	}
	if w.Header().Get("Content-Disposition") == "" {
		t.Fatal("missing download header")
	}
}
func TestJSONToCSV(t *testing.T) {
	w := request(t, "data.json", "csv", []byte(`[{"b":"hello, world","a":9007199254740993}]`))
	if w.Code != 200 || w.Body.String() != "a,b\n9007199254740993,\"hello, world\"\n" {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestImage(t *testing.T) {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 9)))
	w := request(t, "image.png", "jpg", b.Bytes())
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
	if err != nil || format != "jpeg" || cfg.Width != 8 || cfg.Height != 9 {
		t.Fatal(cfg, format, err)
	}
}
func TestInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name, target, data string
		code               int
	}{{"x.exe", "png", "garbage", 400}, {"x.png", "jpg", "garbage", 422}, {"x.csv", "json", "a,a\n1,2", 422}, {"x.json", "csv", `[{"x":{}}]`, 422}, {"x.json", "csv", `[{"x":1}] trailing`, 422}, {"x.png", "../../evil", "", 400}} {
		t.Run(tc.name+tc.target+tc.data, func(t *testing.T) {
			w := request(t, tc.name, tc.target, []byte(tc.data))
			if w.Code != tc.code {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}
func TestMethodAndBusy(t *testing.T) {
	s := testHandler(t, 1)
	w := httptest.NewRecorder()
	router(s).ServeHTTP(w, httptest.NewRequest("GET", "/api/convert", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
	s.slots <- struct{}{}
	w = httptest.NewRecorder()
	router(s).ServeHTTP(w, httptest.NewRequest("POST", "/api/convert", nil))
	if w.Code != 429 {
		t.Fatal(w.Code)
	}
}
func TestFormats(t *testing.T) {
	s := testHandler(t, 1)
	w := httptest.NewRecorder()
	router(s).ServeHTTP(w, httptest.NewRequest("GET", "/api/formats", nil))
	data, _ := io.ReadAll(w.Result().Body)
	if w.Code != 200 || !json.Valid(data) {
		t.Fatal(string(data))
	}
}

func testHandler(t *testing.T, count int) *Handler {
	t.Helper()
	repo, err := files.NewRepository(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewService(repo, converter.NewRunner(), converter.Routes(), 50<<20, 2*time.Minute)
	return NewHandler(svc, count)
}
func router(h *Handler) http.Handler { mux := http.NewServeMux(); RegisterRoutes(mux, h); return mux }
