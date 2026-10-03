package converter

import (
	"bytes"
	"context"
	"converter/internal/model"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type mediaStream struct {
	Index       int    `json:"index"`
	Type        string `json:"codec_type"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
}

func mediaStreams(ctx context.Context, input string) ([]mediaStream, error) {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-show_entries", "stream=index,codec_type:stream_disposition=attached_pic", "-of", "json", input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ffprobe: %w", ctx.Err())
		}
		return nil, fmt.Errorf("ffprobe: %w: %.2000s", err, stderr.String())
	}
	var info struct {
		Streams []mediaStream `json:"streams"`
	}
	if err = json.Unmarshal(output, &info); err != nil {
		return nil, err
	}
	return info.Streams, nil
}

func convertMedia(ctx context.Context, input, output, target string) error {
	// Limit decoder and filter threads as well as the output encoder threads.
	args := []string{"-nostdin", "-v", "error", "-y", "-protocol_whitelist", "file,pipe", "-threads", "2", "-filter_threads", "2", "-i", input}
	if target == "mp3" || target == "webm" {
		streams, err := mediaStreams(ctx, input)
		if err != nil {
			return err
		}
		audio, video := -1, -1
		for _, stream := range streams {
			if stream.Type == "audio" && audio < 0 {
				audio = stream.Index
			}
			if stream.Type == "video" && stream.Disposition.AttachedPic == 0 && video < 0 {
				video = stream.Index
			}
		}
		if target == "mp3" {
			if audio < 0 {
				return model.ErrNoAudio
			}
			args = append(args, "-map", "0:"+strconv.Itoa(audio), "-vn", "-sn", "-dn", "-c:a", "libmp3lame", "-q:a", "2", "-ac", "2", "-ar", "44100")
		} else {
			if audio < 0 && video < 0 {
				return model.ErrNoMedia
			}
			if video >= 0 {
				args = append(args, "-map", "0:"+strconv.Itoa(video), "-c:v", "libvpx-vp9", "-deadline", "realtime", "-cpu-used", "6", "-row-mt", "1", "-lag-in-frames", "0", "-crf", "32", "-b:v", "0", "-pix_fmt", "yuv420p")
			}
			if audio >= 0 {
				args = append(args, "-map", "0:"+strconv.Itoa(audio), "-c:a", "libopus", "-b:a", "128k", "-ac", "2", "-ar", "48000")
			}
			args = append(args, "-sn", "-dn")
		}
	}
	args = append(args, "-threads", "2", output)
	return command(ctx, "ffmpeg", args...)
}
