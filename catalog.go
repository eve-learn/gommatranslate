package gommatranslate

import (
	"github.com/eve-learn/gommatranslate/internal/huggingface"
	"github.com/eve-learn/gommatranslate/internal/languages"
	"github.com/eve-learn/gommatranslate/internal/models"
)

// Model is a packaged TranslateGemma runtime that can be downloaded and started.
type Model struct {
	ID          string
	FileName    string
	Size        string
	SizeBytes   int64
	Vision      bool
	Recommended bool
}

// AvailableModels returns the packaged runtimes. The list is read from the
// Hugging Face manifest and falls back to the built-in catalog when the
// manifest cannot be fetched.
func AvailableModels() []Model {
	items := huggingface.ListTranslateGemmaModels()
	out := make([]Model, 0, len(items))
	for _, item := range items {
		out = append(out, toModel(item))
	}
	return out
}

// Languages returns the language codes the model accepts, such as "en" and "zh-CN".
// Translate also accepts "auto" as a source language so the model can detect it.
func Languages() []string {
	return languages.Codes()
}

func toModel(item models.QuantizedModel) Model {
	return Model{
		ID:          item.ID,
		FileName:    item.FileName,
		Size:        item.Size,
		SizeBytes:   item.SizeBytes,
		Vision:      models.SupportsVision(item),
		Recommended: item.Recommended,
	}
}
