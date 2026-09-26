package tui

import (
	"strings"

	"translategemma-ui/internal/config"
	"translategemma-ui/internal/huggingface"
	"translategemma-ui/internal/models"
	"translategemma-ui/internal/modelstore"
	lf "translategemma-ui/internal/runtime/llamafile"
	"translategemma-ui/internal/runtimeutil"
	"translategemma-ui/internal/translate"
)

type model struct {
	models       []models.QuantizedModel
	catalog      []modelstore.CatalogItem
	cursor       int
	selected     *models.QuantizedModel
	selectedName string
	service      *translate.Service
	runtime      *lf.Manager
	backendURL   string
	dataRoot     string
	cfg          config.AppConfig
	state        config.AppState
}

type provisionProgressMsg struct {
	Stage            string
	Percent          float64
	Downloaded       int64
	Total            int64
	SpeedBytesPerSec float64
	Message          string
}

type provisionDoneMsg struct {
	ModelPath  string
	BackendURL string
	Message    string
}

type provisionErrMsg struct {
	Message string
}

// Run starts the app and manages runtime lifecycle.
func Run(preselectedModelID, dataRoot string) error {
	m := newModel(preselectedModelID, dataRoot)
	stopErr := m.runtime.StopOwned()
	if stopErr != nil {
		return stopErr
	}
	return nil
}

func newModel(preselectedModelID, dataRoot string) model {
	all := huggingface.ListTranslateGemmaModels()
	cfg, _ := config.LoadAppConfig(dataRoot)
	state, _ := config.LoadAppState(dataRoot)
	backendURL := runtimeutil.SyncBackendURL(&state, state.BackendURL)

	activeID := strings.TrimSpace(preselectedModelID)
	if activeID == "" {
		activeID = strings.TrimSpace(cfg.ActiveModelID)
	}

	m := model{
		models:     all,
		service:    translate.NewService(backendURL),
		runtime:    lf.NewManager(dataRoot, backendURL),
		backendURL: backendURL,
		dataRoot:   dataRoot,
		cfg:        cfg,
		state:      state,
	}

	if len(all) > 0 {
		for i := range all {
			if all[i].ID == activeID {
				m.cursor = i
				break
			}
		}
	}

	m.bootstrapInstalledRuntime(activeID)
	return m
}

func (m model) localModelPath(fileName string) string {
	return modelstore.LocalModelPath(m.dataRoot, fileName)
}

func (m *model) bootstrapInstalledRuntime(preferredID string) bool {
	catalog := modelstore.Catalog(m.dataRoot, m.models, strings.TrimSpace(preferredID), strings.TrimSpace(m.state.ActiveModelPath))

	if idx, item, ok := modelstore.ResolveCatalogItem(catalog, m.state.ActiveModelPath, modelstore.ResolveOptions{
		PreferTextRuntime: true,
	}, preferredID, m.cfg.ActiveModelID); ok {
		m.cursor = idx
		return m.prepareStartupRuntime(m.setKnownSelection(item, item.Path))
	}
	return false
}

func (m *model) setKnownSelection(item modelstore.CatalogItem, modelPath string) string {
	selected := item.QuantizedModel
	m.selected = &selected
	m.selectedName = selected.FileName
	runtimeutil.ApplyActiveModel(&m.cfg, &m.state, selected, modelPath)
	_ = config.SaveAppConfig(m.dataRoot, m.cfg)
	return m.applyRuntimePath(modelPath)
}

func (m *model) prepareStartupRuntime(modelPath string) bool {
	if strings.TrimSpace(modelPath) == "" {
		return false
	}
	return m.runtime.RuntimeStatus().Ready
}

func (m *model) applyRuntimePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	runtimeutil.ApplyRuntimePath(&m.state, path)
	m.runtime.SetPreferredModelPath(path)
	_ = config.SaveAppState(m.dataRoot, m.state)
	return path
}

func (m *model) syncBackendURL(next string, persist bool) {
	m.backendURL = runtimeutil.SyncBackendURL(&m.state, next, m.runtime, m.service)
	if persist {
		_ = config.SaveAppState(m.dataRoot, m.state)
	}
}

func (m *model) resolveActiveRuntimePath() string {
	catalog := modelstore.Catalog(m.dataRoot, m.models, strings.TrimSpace(m.cfg.ActiveModelID), strings.TrimSpace(m.state.ActiveModelPath))

	preferredIDs := make([]string, 0, 2)
	if m.selected != nil {
		preferredIDs = append(preferredIDs, m.selected.ID)
	}
	preferredIDs = append(preferredIDs, m.cfg.ActiveModelID)
	if idx, item, ok := modelstore.ResolveCatalogItem(catalog, m.state.ActiveModelPath, modelstore.ResolveOptions{
		PreferTextRuntime: true,
	}, preferredIDs...); ok {
		m.cursor = idx
		return m.setKnownSelection(item, item.Path)
	}
	return ""
}
