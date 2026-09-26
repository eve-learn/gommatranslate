package gommatranslate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/eve-learn/gommatranslate/internal/huggingface"
	"github.com/eve-learn/gommatranslate/internal/models"
	"github.com/eve-learn/gommatranslate/internal/platform"
	"github.com/eve-learn/gommatranslate/internal/translate"
)

func TestDownloadSavesRuntimeForCurrentOS(t *testing.T) {
	const fileName = "translategemma-4b-it.Q4_K_M.llamafile"
	payload := []byte("llamafile-bytes")
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	restore := huggingface.SeedCatalogForTests([]models.QuantizedModel{{
		ID:          "q4_k_m",
		Kind:        "model",
		FileName:    fileName,
		SizeBytes:   int64(len(payload)),
		DownloadURL: srv.URL + "/model.llamafile",
		Recommended: true,
	}})
	defer restore()

	client, err := New(Options{DataDir: t.TempDir(), ModelID: "q4_k_m"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()

	var last DownloadProgress
	if err := client.Download(context.Background(), func(p DownloadProgress) {
		last = p
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}

	wantName := platform.PreferredRuntimeFileName(runtime.GOOS, fileName)
	got, err := os.ReadFile(filepath.Join(client.dataDir, "runtimes", wantName))
	if err != nil {
		t.Fatalf("read downloaded runtime: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded bytes = %q", got)
	}
	if last.Percent != 100 {
		t.Fatalf("final progress = %#v", last)
	}
	path, err := client.ModelPath()
	if err != nil {
		t.Fatalf("ModelPath: %v", err)
	}
	if filepath.Base(path) != wantName {
		t.Fatalf("ModelPath base = %q, want %q", filepath.Base(path), wantName)
	}

	if err := client.Download(context.Background(), nil); err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if hits != 1 {
		t.Fatalf("download requests = %d, want 1", hits)
	}
}

func TestDownloadRejectsUnknownModel(t *testing.T) {
	restore := huggingface.SeedCatalogForTests([]models.QuantizedModel{{
		ID:       "q4_k_m",
		FileName: "translategemma-4b-it.Q4_K_M.llamafile",
	}})
	defer restore()

	client, err := New(Options{DataDir: t.TempDir(), ModelID: "missing"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = client.Download(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "unknown model") {
		t.Fatalf("Download error = %v", err)
	}
}

func TestStartRequiresDownload(t *testing.T) {
	restore := huggingface.SeedCatalogForTests([]models.QuantizedModel{{
		ID:          "q4_k_m",
		FileName:    "translategemma-4b-it.Q4_K_M.llamafile",
		Recommended: true,
	}})
	defer restore()

	client, err := New(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = client.Start(context.Background())
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Start error = %v", err)
	}
}

func TestTranslateRequiresRuntimeAndKnownLanguages(t *testing.T) {
	client := &Client{}
	_, err := client.Translate(context.Background(), "hello", "en", "es")
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Translate error = %v", err)
	}

	_, err = client.Translate(context.Background(), "hello", "en", "not-a-language")
	if err == nil || !strings.Contains(err.Error(), "unknown target language") {
		t.Fatalf("Translate error = %v", err)
	}

	_, err = client.Translate(context.Background(), "   ", "en", "es")
	if err == nil || !strings.Contains(err.Error(), "input text is empty") {
		t.Fatalf("Translate error = %v", err)
	}
}

func TestTranslateSendsCanonicalLanguages(t *testing.T) {
	var gotSource, gotTarget, gotText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tokenize":
			http.NotFound(w, r)
		case "/v1/translate":
			var payload struct {
				Text       string `json:"text"`
				SourceLang string `json:"source_lang"`
				TargetLang string `json:"target_lang"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			gotSource, gotTarget, gotText = payload.SourceLang, payload.TargetLang, payload.Text
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":" hola "}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := &Client{
		running: true,
		service: translate.NewService(srv.URL),
	}
	out, err := client.Translate(context.Background(), " hello ", "EN", "es")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if out != "hola" {
		t.Fatalf("output = %q", out)
	}
	if gotText != "hello" || gotSource != "en" || gotTarget != "es" {
		t.Fatalf("payload text=%q source=%q target=%q", gotText, gotSource, gotTarget)
	}
}

func TestTranslateStreamReturnsFullText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tokenize" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Bon\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"jour\"},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	client := &Client{
		running: true,
		service: translate.NewService(srv.URL),
	}
	var chunks []string
	out, err := client.TranslateStream(context.Background(), "hello", "en", "fr", func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("TranslateStream: %v", err)
	}
	if out != "Bonjour" {
		t.Fatalf("output = %q", out)
	}
	if strings.Join(chunks, "") != "Bonjour" {
		t.Fatalf("chunks = %#v", chunks)
	}
}

func TestLanguagesIncludeCommonCodes(t *testing.T) {
	codes := Languages()
	seen := map[string]bool{}
	for _, code := range codes {
		seen[code] = true
	}
	for _, code := range []string{"en", "es", "zh-CN", "ja"} {
		if !seen[code] {
			t.Fatalf("missing language %q", code)
		}
	}
	if seen["auto"] {
		t.Fatal("Languages included auto")
	}
}
