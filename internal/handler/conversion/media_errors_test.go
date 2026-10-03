package conversion

import (
	"bytes"
	"context"
	"converter/internal/model"
	"converter/internal/repository/files"
	service "converter/internal/service/conversion"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type failingMediaConverter struct{ err error }

func (f failingMediaConverter) Convert(context.Context, string, string, string, string, string, string) error {
	return f.err
}
func TestMediaErrorsReachClient(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		message string
	}{
		{"missing audio", model.ErrNoAudio, "нет аудиодорожки"},
		{"missing media", model.ErrNoMedia, "не найдены"},
		{"timeout", context.DeadlineExceeded, "за 120 секунд"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, err := files.NewRepository(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			svc := service.NewService(repo, failingMediaConverter{tc.err}, []model.Route{{From: "mp4", To: []string{"mp3"}, Engine: "ffmpeg"}}, 1024, 120*time.Second)
			h := NewHandler(svc, 1)
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			file, err := writer.CreateFormFile("file", "video.mp4")
			if err != nil {
				t.Fatal(err)
			}
			file.Write([]byte("test"))
			writer.WriteField("format", "mp3")
			writer.Close()
			req := httptest.NewRequest("POST", "/api/convert", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()
			router(h).ServeHTTP(response, req)
			if response.Code != 422 {
				t.Fatal(response.Code, response.Body.String())
			}
			var data map[string]string
			if err = json.Unmarshal(response.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(data["error"], tc.message) {
				t.Fatal(data)
			}
		})
	}
}
