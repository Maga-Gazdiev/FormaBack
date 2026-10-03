package converter

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

type Runner struct{}

func NewRunner() *Runner { return &Runner{} }

func command(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s: %w", name, ctx.Err())
		}
		return fmt.Errorf("%s: %w: %.2000s", name, err, logs.String())
	}
	return nil
}
func (r *Runner) Convert(ctx context.Context, engine, input, output, from, to, dir string) error {
	switch engine {
	case "native":
		data, err := os.ReadFile(input)
		if err != nil {
			return err
		}
		if from == "csv" {
			records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
			if err != nil {
				return err
			}
			if len(records) == 0 {
				return errors.New("empty csv")
			}
			seen := map[string]bool{}
			for _, h := range records[0] {
				if h == "" || seen[h] {
					return errors.New("headers must be unique and nonempty")
				}
				seen[h] = true
			}
			rows := []map[string]string{}
			for _, r := range records[1:] {
				row := map[string]string{}
				for i, key := range records[0] {
					row[key] = r[i]
				}
				rows = append(rows, row)
			}
			b, err := json.MarshalIndent(rows, "", "  ")
			if err != nil {
				return err
			}
			return os.WriteFile(output, b, 0600)
		}
		if from == "json" {
			var rows []map[string]any
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.UseNumber()
			if err := dec.Decode(&rows); err != nil {
				return err
			}
			var extra any
			if dec.Decode(&extra) != io.EOF {
				return errors.New("trailing json")
			}
			if len(rows) == 0 {
				return errors.New("expected nonempty array of objects")
			}
			keys := map[string]bool{}
			for _, row := range rows {
				for k, v := range row {
					switch v.(type) {
					case map[string]any, []any:
						return errors.New("nested values unsupported")
					}
					keys[k] = true
				}
			}
			headers := []string{}
			for k := range keys {
				headers = append(headers, k)
			}
			sort.Strings(headers)
			var b bytes.Buffer
			cw := csv.NewWriter(&b)
			cw.Write(headers)
			for _, row := range rows {
				record := make([]string, len(headers))
				for i, k := range headers {
					if row[k] != nil {
						record[i] = fmt.Sprint(row[k])
					}
				}
				cw.Write(record)
			}
			cw.Flush()
			if err := cw.Error(); err != nil {
				return err
			}
			return os.WriteFile(output, b.Bytes(), 0600)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return err
		}
		if int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
			return errors.New("image exceeds 40 megapixels")
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return err
		}
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
		switch to {
		case "png":
			return png.Encode(f, img)
		case "gif":
			return gif.Encode(f, img, nil)
		case "jpg":
			bg := image.NewRGBA(img.Bounds())
			draw.Draw(bg, bg.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
			draw.Draw(bg, bg.Bounds(), img, img.Bounds().Min, draw.Over)
			return jpeg.Encode(f, bg, &jpeg.Options{Quality: 90})
		}
	case "convert":
		return command(ctx, "convert", "-limit", "memory", "128MiB", "-limit", "map", "256MiB", input+"[0]", output)
	case "libreoffice":
		outdir := filepath.Join(dir, "office")
		if err := os.Mkdir(outdir, 0700); err != nil {
			return err
		}
		if err := command(ctx, "libreoffice", "-env:UserInstallation=file://"+filepath.Join(dir, "profile"), "--headless", "--convert-to", to, "--outdir", outdir, input); err != nil {
			return err
		}
		return os.Rename(filepath.Join(outdir, "input."+to), output)
	case "pandoc":
		return command(ctx, "pandoc", input, "--standalone", "-o", output)
	case "pdftotext":
		return command(ctx, "pdftotext", "-layout", input, output)
	case "pdftoppm":
		flag := "-png"
		if to == "jpg" {
			flag = "-jpeg"
		}
		prefix := filepath.Join(dir, "page")
		if err := command(ctx, "pdftoppm", flag, "-scale-to", "2000", input, prefix); err != nil {
			return err
		}
		pages, err := filepath.Glob(prefix + "-*." + to)
		if err != nil {
			return err
		}
		if len(pages) == 0 {
			return errors.New("no pages")
		}
		if len(pages) == 1 {
			return os.Rename(pages[0], output)
		}
		f, err := os.Create(output + ".zip")
		if err != nil {
			return err
		}
		defer f.Close()
		z := zip.NewWriter(f)
		for _, p := range pages {
			entry, err := z.Create(filepath.Base(p))
			if err != nil {
				return err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if _, err = entry.Write(b); err != nil {
				return err
			}
		}
		return z.Close()
	case "ffmpeg":
		return convertMedia(ctx, input, output, to)
	}
	return errors.New("unsupported conversion")
}
