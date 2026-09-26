package gommatranslate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/eve-learn/gommatranslate/internal/config"
	"github.com/eve-learn/gommatranslate/internal/huggingface"
	"github.com/eve-learn/gommatranslate/internal/languages"
	"github.com/eve-learn/gommatranslate/internal/models"
	"github.com/eve-learn/gommatranslate/internal/modelstore"
	"github.com/eve-learn/gommatranslate/internal/runtime"
	lf "github.com/eve-learn/gommatranslate/internal/runtime/llamafile"
	"github.com/eve-learn/gommatranslate/internal/runtimeutil"
	"github.com/eve-learn/gommatranslate/internal/translate"
)

const defaultModelID = "q4_k_m"

var (
	// ErrNotInstalled is returned when Start or ModelPath is used before the
	// selected model has been downloaded.
	ErrNotInstalled = errors.New("model is not installed")

	// ErrNotRunning is returned when Translate is used before Start.
	ErrNotRunning = errors.New("runtime is not running")
)

// Options configures where a Client stores its runtime and which model it runs.
type Options struct {
	// DataDir is the directory for the downloaded runtime, logs, and state.
	// The default is $HOME/.gommatranslate.
	DataDir string

	// ModelID selects a packaged runtime such as "q4_k_m", "q6_k", "q8_0",
	// or "q8_0_vision". The default is the recommended text model.
	ModelID string

	// BackendURL is the local address the llamafile server listens on.
	// The default is http://127.0.0.1:8080. Start chooses the next free port
	// when that address is already taken.
	BackendURL string
}

// DownloadProgress reports bytes written while a runtime file is fetched.
type DownloadProgress struct {
	Downloaded     int64
	Total          int64
	Percent        float64
	BytesPerSecond float64
	Message        string
}

// Request is one text translation.
type Request struct {
	Text        string
	SourceLang  string
	TargetLang  string
	Instruction string
}

// Client downloads a platform-specific TranslateGemma runtime, runs it, and
// translates text.
type Client struct {
	dataDir    string
	backendURL string

	mu      sync.Mutex
	modelID string
	manager *lf.Manager
	service *translate.Service
	running bool
}

// New prepares a client. It creates the data directory and does not download
// or start the runtime.
func New(opts Options) (*Client, error) {
	dir, err := config.EnsureDataDirs(opts.DataDir)
	if err != nil {
		return nil, fmt.Errorf("gommatranslate: create data dir: %w", err)
	}
	backend := strings.TrimSpace(opts.BackendURL)
	if backend == "" {
		backend = runtime.DefaultBackendURL
	} else {
		backend = runtime.NormalizeBackendURL(backend)
	}
	return &Client{
		dataDir:    dir,
		backendURL: backend,
		modelID:    strings.TrimSpace(opts.ModelID),
	}, nil
}

// Download fetches the selected runtime for this operating system when it is
// not already present. On Windows the file is saved with a .exe suffix. On
// macOS and Linux it is saved as an executable .llamafile.
func (c *Client) Download(ctx context.Context, onProgress func(DownloadProgress)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	item, err := c.chosenModel()
	if err != nil {
		return err
	}
	path, err := huggingface.DownloadModelWithContext(ctx, c.dataDir, item, func(p huggingface.DownloadProgress) {
		if onProgress == nil {
			return
		}
		onProgress(DownloadProgress{
			Downloaded:     p.Downloaded,
			Total:          p.Total,
			Percent:        p.Percent,
			BytesPerSecond: p.SpeedBytesPerSec,
			Message:        p.Message,
		})
	})
	if err != nil {
		return err
	}
	return c.rememberModel(item, path)
}

// Start launches the downloaded llamafile and blocks until its HTTP server
// accepts translation requests.
func (c *Client) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	item, path, err := c.installedModel()
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("gommatranslate: %w", ErrNotInstalled)
	}

	c.mu.Lock()
	if c.manager == nil {
		c.manager = lf.NewManager(c.dataDir, c.backendURL)
	}
	manager := c.manager
	c.mu.Unlock()

	manager.SetPreferredModelPath(path)
	if _, err := manager.EnsureRunningWithContext(ctx, nil); err != nil {
		return err
	}

	backend := manager.CurrentBackendURL()
	c.mu.Lock()
	c.backendURL = backend
	if c.service == nil {
		c.service = translate.NewService(backend)
	} else {
		c.service.SetBackendURL(backend)
	}
	c.running = true
	c.mu.Unlock()

	return c.rememberModel(item, path)
}

// Translate translates text from sourceLang into targetLang.
// sourceLang may be empty or "auto" to let the model detect the source language.
func (c *Client) Translate(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	return c.TranslateRequest(ctx, Request{
		Text:       text,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	})
}

// TranslateRequest translates text and can include an extra instruction, such
// as "Use concise UI wording."
func (c *Client) TranslateRequest(ctx context.Context, req Request) (string, error) {
	return c.translate(ctx, req, nil, false)
}

// TranslateStream translates text and calls onDelta with each streamed chunk.
// The returned string is the full translation.
func (c *Client) TranslateStream(ctx context.Context, text, sourceLang, targetLang string, onDelta func(string) error) (string, error) {
	return c.translate(ctx, Request{
		Text:       text,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	}, onDelta, true)
}

// ModelPath returns the local runtime file for this operating system.
func (c *Client) ModelPath() (string, error) {
	_, path, err := c.installedModel()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("gommatranslate: %w", ErrNotInstalled)
	}
	return path, nil
}

// BackendURL returns the local llamafile address. After Start it reflects the
// port the process actually bound.
func (c *Client) BackendURL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.manager != nil {
		return c.manager.CurrentBackendURL()
	}
	return c.backendURL
}

// Close stops the llamafile process started by this client.
func (c *Client) Close() error {
	c.mu.Lock()
	manager := c.manager
	c.running = false
	c.mu.Unlock()
	if manager == nil {
		return nil
	}
	return manager.StopOwned()
}

func (c *Client) translate(ctx context.Context, req Request, onDelta func(string) error, stream bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized, err := normalizeRequest(req)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	svc := c.service
	running := c.running
	c.mu.Unlock()
	if !running || svc == nil {
		return "", fmt.Errorf("gommatranslate: %w", ErrNotRunning)
	}

	payload := translate.Request{
		SourceLang:             normalized.SourceLang,
		TargetLang:             normalized.TargetLang,
		Text:                   normalized.Text,
		TranslationInstruction: normalized.Instruction,
	}
	if stream {
		return svc.StreamTranslateWithContext(ctx, payload, onDelta)
	}
	return svc.TranslateWithContext(ctx, payload)
}

func (c *Client) chosenModel() (models.QuantizedModel, error) {
	c.mu.Lock()
	id := c.modelID
	c.mu.Unlock()

	item, err := selectModel(huggingface.ListTranslateGemmaModels(), id)
	if err != nil {
		return models.QuantizedModel{}, err
	}
	c.mu.Lock()
	if c.modelID == "" {
		c.modelID = item.ID
	}
	c.mu.Unlock()
	return item, nil
}

func (c *Client) installedModel() (models.QuantizedModel, string, error) {
	item, err := c.chosenModel()
	if err != nil {
		return models.QuantizedModel{}, "", err
	}
	return item, modelstore.LocalModelPath(c.dataDir, item.FileName), nil
}

func (c *Client) rememberModel(item models.QuantizedModel, path string) error {
	cfg, err := config.LoadAppConfig(c.dataDir)
	if err != nil {
		return err
	}
	state, err := config.LoadAppState(c.dataDir)
	if err != nil {
		return err
	}
	runtimeutil.ApplyActiveModel(&cfg, &state, item, path)
	c.mu.Lock()
	state.BackendURL = c.backendURL
	c.mu.Unlock()
	if err := config.SaveAppConfig(c.dataDir, cfg); err != nil {
		return err
	}
	return config.SaveAppState(c.dataDir, state)
}

func selectModel(items []models.QuantizedModel, id string) (models.QuantizedModel, error) {
	id = strings.TrimSpace(id)
	if id != "" {
		item, ok := models.FindByID(items, id)
		if !ok {
			return models.QuantizedModel{}, fmt.Errorf("gommatranslate: unknown model %q", id)
		}
		return item, nil
	}
	for _, item := range items {
		if item.Recommended && !models.SupportsVision(item) {
			return item, nil
		}
	}
	if item, ok := models.FindByID(items, defaultModelID); ok && !models.SupportsVision(item) {
		return item, nil
	}
	for _, item := range items {
		if !models.SupportsVision(item) {
			return item, nil
		}
	}
	if len(items) == 0 {
		return models.QuantizedModel{}, errors.New("gommatranslate: model catalog is empty")
	}
	return items[0], nil
}

func normalizeRequest(req Request) (Request, error) {
	req.Text = strings.TrimSpace(req.Text)
	req.Instruction = strings.TrimSpace(req.Instruction)
	if req.Text == "" {
		return Request{}, errors.New("gommatranslate: input text is empty")
	}
	source, err := normalizeLang("source", req.SourceLang, true)
	if err != nil {
		return Request{}, err
	}
	target, err := normalizeLang("target", req.TargetLang, false)
	if err != nil {
		return Request{}, err
	}
	req.SourceLang = source
	req.TargetLang = target
	return req, nil
}

func normalizeLang(field, code string, allowAuto bool) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		if allowAuto {
			return "auto", nil
		}
		return "", fmt.Errorf("gommatranslate: %s language is required", field)
	}
	canon, ok := languages.Canonical(code)
	if !ok || (canon == "auto" && !allowAuto) {
		return "", fmt.Errorf("gommatranslate: unknown %s language %q", field, code)
	}
	return canon, nil
}
