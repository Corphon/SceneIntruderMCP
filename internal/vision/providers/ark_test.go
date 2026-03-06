package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Corphon/SceneIntruderMCP/internal/vision"
)

func TestArkImagesProvider_GenerateImage_DownloadsURL(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotBody map[string]interface{}

	imageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	}))
	defer imageSrv.Close()

	arkSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"` + imageSrv.URL + `","size":"1024x1024"}]}`))
	}))
	defer arkSrv.Close()

	p := NewArkImagesProvider(arkSrv.URL+"/api/v3", "test-key")
	p.GenerationPath = "/images/generations"
	p.ModelKeyOverrides = map[string]string{"doubao-seedream-4.5": "doubao-seedream-4-5-251128"}
	p.SizeOverride = "1024x1024"

	img, err := p.GenerateImage(context.Background(), "a cat", vision.VisionGenerateOptions{Model: "doubao-seedream-4.5"})
	if err != nil {
		t.Fatalf("GenerateImage error: %v", err)
	}
	if img == nil || len(img.Data) == 0 {
		t.Fatalf("expected non-empty image")
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("expected auth header, got=%q", gotAuth)
	}
	if gotPath != "/api/v3/images/generations" {
		t.Fatalf("expected path /api/v3/images/generations, got=%q", gotPath)
	}
	if gotBody["prompt"] != "a cat" {
		t.Fatalf("expected prompt, got=%v", gotBody["prompt"])
	}
	if gotBody["model"] != "doubao-seedream-4-5-251128" {
		t.Fatalf("expected model override, got=%v", gotBody["model"])
	}
	if gotBody["size"] != "1024x1024" {
		t.Fatalf("expected size, got=%v", gotBody["size"])
	}
	if gotBody["watermark"] != false {
		t.Fatalf("expected watermark=false, got=%v", gotBody["watermark"])
	}
	if !strings.HasPrefix(img.ContentType, "image/") {
		t.Fatalf("expected image content-type, got=%q", img.ContentType)
	}
}
