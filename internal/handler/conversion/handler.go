package conversion

import (
	"context"
	"converter/internal/model"
	service "converter/internal/service/conversion"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"os"
)

type Handler struct {
	service *service.Service
	slots   chan struct{}
}

func NewHandler(service *service.Service, concurrent int) *Handler {
	return &Handler{service: service, slots: make(chan struct{}, concurrent)}
}
func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("/api/health", h.Health)
	mux.HandleFunc("/api/formats", h.Formats)
	mux.HandleFunc("/api/convert", h.Convert)
}
func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func method(w http.ResponseWriter, r *http.Request, allowed string) bool {
	if r.Method == allowed {
		return true
	}
	w.Header().Set("Allow", allowed)
	writeError(w, 405, "Метод не поддерживается")
	return false
}
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func (h *Handler) Formats(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"formats": h.service.Formats(), "maxFileSize": h.service.MaxFileSize(), "conversionTimeoutSeconds": int(h.service.Timeout().Seconds())})
}
func (h *Handler) Convert(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		writeError(w, 429, "Сервис занят. Повторите попытку чуть позже.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.service.MaxFileSize()+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, 413, "Файл превышает допустимый размер")
		} else {
			writeError(w, 400, "Не удалось прочитать загрузку")
		}
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "Добавьте файл")
		return
	}
	defer file.Close()
	if header.Size > h.service.MaxFileSize() {
		writeError(w, 413, fmt.Sprintf("Максимальный размер — %d МБ", h.service.MaxFileSize()>>20))
		return
	}
	result, err := h.service.Convert(r.Context(), header.Filename, r.FormValue("format"), file)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrUnsupported):
			writeError(w, 400, "Это преобразование не поддерживается")
		case errors.Is(err, model.ErrTooLarge):
			writeError(w, 413, "Файл превышает допустимый размер")
		case errors.Is(err, model.ErrInvalid):
			writeError(w, 400, "Файл пуст или некорректен")
		case errors.Is(err, model.ErrNoAudio):
			writeError(w, 422, "В файле нет аудиодорожки. Извлечь MP3 из видео без звука невозможно.")
		case errors.Is(err, model.ErrNoMedia):
			writeError(w, 422, "В файле не найдены видео- или аудиодорожки.")
		case errors.Is(err, context.DeadlineExceeded):
			slog.Warn("conversion timed out", "error", err)
			writeError(w, 422, fmt.Sprintf("Конвертация не успела завершиться за %d секунд. Попробуйте более короткое видео или увеличьте лимит времени обработки на сервере.", int(h.service.Timeout().Seconds())))
		case errors.Is(err, context.Canceled):
			return
		case errors.Is(err, model.ErrConversion):
			slog.Warn("conversion failed", "error", err)
			writeError(w, 422, "Не удалось преобразовать файл. Он может быть повреждён, защищён паролем или слишком сложным.")
		default:
			slog.Error("conversion error", "error", err)
			writeError(w, 500, "Внутренняя ошибка обработки файла")
		}
		return
	}
	defer result.Cleanup()
	output, err := os.Open(result.Path)
	if err != nil {
		writeError(w, 500, "Не удалось открыть результат")
		return
	}
	defer output.Close()
	stat, err := output.Stat()
	if err != nil {
		writeError(w, 500, "Не удалось прочитать результат")
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": result.Name}))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, result.Name, stat.ModTime(), output)
}
