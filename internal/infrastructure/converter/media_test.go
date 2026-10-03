package converter

import (
	"context"
	"converter/internal/model"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMP4Conversions(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " is not installed")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, tc := range []struct {
		name, layout string
		silent       bool
	}{
		{name: "stereo", layout: "stereo"},
		{name: "surround", layout: "5.1"},
		{name: "silent", silent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input.mp4")
			args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=12"}
			if !tc.silent {
				args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout="+tc.layout+":sample_rate=32000")
			}
			args = append(args, "-t", "0.5", "-c:v", "libx264", "-threads", "2", "-c:a", "aac", input)
			if err := command(ctx, "ffmpeg", args...); err != nil {
				t.Fatal(err)
			}
			for _, target := range []string{"mp3", "webm"} {
				output := filepath.Join(dir, "result."+target)
				err := convertMedia(ctx, input, output, target)
				if tc.silent && target == "mp3" {
					if !errors.Is(err, model.ErrNoAudio) {
						t.Fatalf("expected missing audio, got %v", err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				data, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "stream=codec_name,codec_type,channels,sample_rate", "-of", "json", output).Output()
				if err != nil {
					t.Fatal(err)
				}
				var info struct {
					Streams []struct {
						Codec    string `json:"codec_name"`
						Type     string `json:"codec_type"`
						Channels int    `json:"channels"`
						Rate     string `json:"sample_rate"`
					}
				}
				if err = json.Unmarshal(data, &info); err != nil {
					t.Fatal(err)
				}
				audioCount, videoCount := 0, 0
				for _, stream := range info.Streams {
					switch stream.Type {
					case "audio":
						audioCount++
						want := "mp3"
						rate := "44100"
						if target == "webm" {
							want = "opus"
							rate = "48000"
						}
						if stream.Codec != want || stream.Channels != 2 || stream.Rate != rate {
							t.Fatalf("unexpected audio: %+v", stream)
						}
					case "video":
						videoCount++
						if target != "webm" || stream.Codec != "vp9" {
							t.Fatalf("unexpected video: %+v", stream)
						}
					default:
						t.Fatalf("unexpected stream: %+v", stream)
					}
				}
				if target == "mp3" && (audioCount != 1 || videoCount != 0) {
					t.Fatal("MP3 stream mismatch")
				}
				if target == "webm" && (videoCount != 1 || (!tc.silent && audioCount != 1) || (tc.silent && audioCount != 0)) {
					t.Fatal("WebM stream mismatch")
				}
			}
		})
	}
}
func TestMediaCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := convertMedia(ctx, "missing.mp4", "unused.webm", "webm"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
