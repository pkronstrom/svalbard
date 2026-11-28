package videotext

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
)

const (
	image        = "svalbard-video-parakeet:2.0.3"
	modelURL     = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2"
	modelSHA256  = "1495e91b59d7fc53ddca91f3431758ec99f8fa253fdb0e1e851be0f72b14b0f9"
	modelVersion = "parakeet-tdt-v3-int8"
)

var modelFiles = [...]string{"encoder.int8.onnx", "decoder.int8.onnx", "joiner.int8.onnx", "tokens.txt"}

//go:embed transcribe.Dockerfile
var dockerfile string

//go:embed transcribe.py
var transcribeScript []byte

type Runner struct {
	docker   string
	modelDir string
	progress io.Writer
}

// NewRunner provisions the optional tools and host cache only when a video
// actually needs speech recognition.
func NewRunner(ctx context.Context, progress io.Writer) (*Runner, error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return nil, fmt.Errorf("video transcription needs Docker; start Docker or use --video-index captions")
	}
	if info, err := exec.CommandContext(ctx, docker, "info", "--format", "{{.ServerVersion}}").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("video transcription needs a running Docker daemon: %s: %w", strings.TrimSpace(string(info)), err)
	}
	if err := exec.CommandContext(ctx, docker, "image", "inspect", image).Run(); err != nil {
		fmt.Fprintln(progress, "Building the optional FFmpeg and Parakeet image")
		build := exec.CommandContext(ctx, docker, "build", "--tag", image, "-")
		build.Stdin, build.Stdout, build.Stderr = strings.NewReader(dockerfile), progress, progress
		if err := build.Run(); err != nil {
			return nil, fmt.Errorf("building %s: %w", image, err)
		}
	}
	model, err := cachedModel(ctx, progress)
	if err != nil {
		return nil, err
	}
	return &Runner{docker: docker, modelDir: model, progress: progress}, nil
}

// Transcribe runs FFmpeg and Parakeet with a single temporary video and a
// read-only model mount. Archival media and model weights stay on the host.
func (r *Runner) Transcribe(ctx context.Context, copyMedia func(io.Writer) error) ([]Cue, error) {
	scratch, err := os.MkdirTemp("", "svalbard-video-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	scratch, err = filepath.EvalSymlinks(scratch)
	if err != nil {
		return nil, err
	}
	video, err := os.Create(filepath.Join(scratch, "media"))
	if err != nil {
		return nil, err
	}
	if err := errors.Join(copyMedia(video), video.Close()); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(scratch, "transcribe.py"), transcribeScript, 0o600); err != nil {
		return nil, err
	}

	cidPath := filepath.Join(scratch, "cid")
	cmd := exec.CommandContext(ctx, r.docker, "run", "--rm", "--network", "none",
		"--cidfile", cidPath,
		"--mount", "type=bind,source="+scratch+",target=/job,readonly",
		"--mount", "type=bind,source="+r.modelDir+",target=/model,readonly",
		image, "python", "/job/transcribe.py", "/job/media",
	)
	cmd.Stderr = r.progress
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	finished := false
	defer func() {
		// A killed CLI can leave its container running.
		if cid, err := os.ReadFile(cidPath); err == nil && !finished {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = exec.CommandContext(cleanup, r.docker, "rm", "-f", strings.TrimSpace(string(cid))).Run()
		}
	}()
	var cues []Cue
	decoder := json.NewDecoder(stdout)
	for {
		var cue Cue
		if err := decoder.Decode(&cue); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, fmt.Errorf("reading Parakeet transcript: %w", err)
		}
		if cue.StartMS >= 0 && strings.TrimSpace(cue.Text) != "" {
			cues = append(cues, cue)
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("transcribing video: %w", err)
	}
	finished = true
	return cues, nil
}

func cachedModel(ctx context.Context, w io.Writer) (string, error) {
	if override := os.Getenv("SVALBARD_VIDEO_MODEL_DIR"); override != "" {
		dir, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		if !hasModel(dir) {
			return "", fmt.Errorf("SVALBARD_VIDEO_MODEL_DIR does not contain the Parakeet v3 int8 files: %s", dir)
		}
		return filepath.EvalSymlinks(dir)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(cache, "svalbard", "video")
	dir := filepath.Join(base, modelVersion)
	if hasModel(dir) {
		return dir, nil
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(base, ".model-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	fmt.Fprintln(w, "Downloading Parakeet v3 int8 to the host cache (465 MiB)")
	archive := filepath.Join(stage, "model.tar.bz2")
	if _, err := downloader.Download(ctx, modelURL, archive, modelSHA256); err != nil {
		return "", fmt.Errorf("downloading Parakeet: %w", err)
	}
	extracted := filepath.Join(stage, modelVersion)
	if err := os.Mkdir(extracted, 0o755); err != nil {
		return "", err
	}
	if err := unpackModel(ctx, archive, extracted); err != nil {
		return "", err
	}
	if !hasModel(extracted) {
		return "", fmt.Errorf("downloaded Parakeet archive is incomplete")
	}
	if err := os.Rename(extracted, dir); err != nil && !hasModel(dir) {
		return "", fmt.Errorf("publishing the cached Parakeet model: %w", err)
	}
	return dir, nil
}

func hasModel(dir string) bool {
	for _, file := range modelFiles {
		if st, err := os.Stat(filepath.Join(dir, file)); err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
			return false
		}
	}
	return true
}

func unpackModel(ctx context.Context, archive, dir string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := tar.NewReader(bzip2.NewReader(file))
	allowed := make(map[string]bool, len(modelFiles))
	for _, name := range modelFiles {
		allowed[name] = true
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Base(header.Name)
		if !allowed[name] || header.Typeflag != tar.TypeReg {
			continue
		}
		dst, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(dst, reader)
		if err := errors.Join(copyErr, dst.Close()); err != nil {
			return err
		}
	}
}
