package converter

import (
	"converter/internal/model"
	"os/exec"
	"strings"
)

func available(name string) bool { _, err := exec.LookPath(name); return err == nil }
func Routes() []model.Route {
	var out []model.Route
	add := func(from, to, group, engine string) {
		if engine != "native" && !available(engine) {
			return
		}
		for _, f := range strings.Fields(from) {
			targets := []string{}
			for _, t := range strings.Fields(to) {
				if f != t {
					targets = append(targets, t)
				}
			}
			out = append(out, model.Route{From: f, To: targets, Group: group, Engine: engine})
		}
	}
	add("png jpg jpeg gif", "png jpg gif", "Изображения", "native")
	add("webp bmp tiff tif avif", "png jpg gif webp bmp tiff avif", "Изображения", "convert")
	if available("convert") {
		for i := range out {
			if out[i].Engine == "native" {
				out[i].To = append(out[i].To, "webp", "bmp", "tiff", "avif")
			}
		}
	}
	add("csv", "json", "Данные", "native")
	add("json", "csv", "Данные", "native")
	add("doc docx odt rtf", "pdf docx odt rtf txt", "Документы", "libreoffice")
	add("xls xlsx ods", "pdf xlsx ods csv", "Таблицы", "libreoffice")
	add("ppt pptx odp", "pdf pptx odp", "Презентации", "libreoffice")
	add("txt", "pdf docx odt", "Документы", "libreoffice")
	add("md", "html docx odt", "Документы", "pandoc")
	add("pdf", "txt", "PDF", "pdftotext")
	add("pdf", "png jpg", "PDF", "pdftoppm")
	add("mp3 wav ogg flac m4a aac", "mp3 wav ogg flac m4a", "Аудио", "ffmpeg")
	add("mp4 mov mkv webm avi", "mp4 webm mp3 wav", "Видео", "ffmpeg")
	return out
}
