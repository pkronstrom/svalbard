package toolkit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	previewsDir         = "previews"
	previewTransform    = "jpeg-480x300-v1"
	previewWidth        = 480
	previewHeight       = 300
	previewTargetBytes  = 80 << 10
	previewMaxBytes     = 128 << 10
	previewMaxBodyBytes = 8 << 20
	previewMaxPixels    = 20_000_000
)

type recipePreview struct {
	Summary string        `json:"summary,omitempty"`
	Image   *previewImage `json:"image,omitempty"`
}

type previewImage struct {
	File string `json:"file"`
	Alt  string `json:"alt,omitempty"`
}

var previewClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !validPreviewURL(req.URL) ||
			(via[0].URL.Scheme == "https" && req.URL.Scheme != "https") {
			return errors.New("unsupported image redirect")
		}
		return nil
	},
}

func prepareRecipePreview(ctx context.Context, dir string, recipe catalog.Item, previous *recipePreview, notice func(string)) *recipePreview {
	result := &recipePreview{Summary: strings.TrimSpace(recipe.Preview.Summary)}
	if source := recipe.Preview.Image; source != nil && strings.TrimSpace(source.URL) != "" {
		file, err := prepareThumbnail(ctx, dir, strings.TrimSpace(source.URL))
		if err == nil {
			result.Image = &previewImage{File: file, Alt: strings.TrimSpace(source.Alt)}
		} else {
			if previous != nil && previous.Image != nil && hasThumbnail(dir, previous.Image.File) {
				result.Image = previous.Image
			}
			if notice != nil {
				notice(fmt.Sprintf("Thumbnail for %s: %v", recipe.ID, err))
			}
		}
	}
	if result.Summary == "" && result.Image == nil {
		return nil
	}
	return result
}

func prepareThumbnail(ctx context.Context, dir, source string) (string, error) {
	hash := sha256.Sum256([]byte(previewTransform + "\x00" + source))
	name := hex.EncodeToString(hash[:]) + ".jpg"
	if hasThumbnail(dir, name) {
		return name, nil
	}

	download, err := downloadPreview(ctx, source)
	if err != nil {
		return "", err
	}
	thumbnail, err := encodeThumbnail(download)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if !directoryIsReal(dir) {
		return "", errors.New("thumbnail directory is not a real directory")
	}
	tmp, err := os.CreateTemp(dir, ".preview-*.jpg")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(thumbnail)
	if err := errors.Join(writeErr, tmp.Close()); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		return "", err
	}
	return name, nil
}

func hasThumbnail(dir, name string) bool {
	if len(name) != sha256.Size*2+4 || filepath.Ext(name) != ".jpg" || filepath.Base(name) != name || !directoryIsReal(dir) {
		return false
	}
	if _, err := hex.DecodeString(strings.TrimSuffix(name, ".jpg")); err != nil {
		return false
	}
	if info, err := os.Lstat(filepath.Join(dir, name)); err == nil {
		return info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= previewMaxBytes
	}
	return false
}

func directoryIsReal(dir string) bool {
	info, err := os.Lstat(dir)
	return err == nil && info.IsDir()
}

func validPreviewURL(address *url.URL) bool {
	if address.Hostname() == "" || address.User != nil {
		return false
	}
	return address.Scheme == "https" ||
		(address.Scheme == "http" && net.ParseIP(address.Hostname()).IsLoopback())
}

func downloadPreview(ctx context.Context, source string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil || !validPreviewURL(req.URL) {
		return nil, errors.New("expected an HTTPS image URL")
	}
	resp, err := previewClient.Do(req)
	if err != nil {
		// The request error's URL may include private query parameters.
		var requestError *url.Error
		if errors.As(err, &requestError) {
			err = requestError.Err
		}
		return nil, fmt.Errorf("fetching image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching image: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > previewMaxBodyBytes {
		return nil, errors.New("source image exceeds 8 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, previewMaxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}
	if len(data) > previewMaxBodyBytes {
		return nil, errors.New("source image exceeds 8 MiB")
	}
	return data, nil
}

func encodeThumbnail(data []byte) ([]byte, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("expected a JPEG, PNG, GIF, or WebP image")
	}
	width, height := config.Width, config.Height
	if width <= 0 || height <= 0 || width > previewMaxPixels/height {
		return nil, errors.New("source image exceeds 20 megapixels")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	if width > previewWidth || height > previewHeight {
		if int64(width)*previewHeight > int64(height)*previewWidth {
			height, width = max(1, int(int64(height)*previewWidth/int64(width))), previewWidth
		} else {
			width, height = max(1, int(int64(width)*previewHeight/int64(height))), previewHeight
		}
	}

	scaled := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(scaled, scaled.Bounds(), image.White, image.Point{}, draw.Src)
	draw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), source, source.Bounds(), draw.Over, nil)
	var encoded bytes.Buffer
	for _, quality := range []int{84, 76, 65, 52, 40} {
		encoded.Reset()
		if err := jpeg.Encode(&encoded, scaled, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("encoding thumbnail: %w", err)
		}
		if encoded.Len() <= previewTargetBytes {
			return encoded.Bytes(), nil
		}
	}
	if encoded.Len() > previewMaxBytes {
		return nil, errors.New("thumbnail exceeds 128 KiB")
	}
	return encoded.Bytes(), nil
}
