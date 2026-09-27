# gommatranslate

`gommatranslate` is a Go package that downloads a [TranslateGemma](https://huggingface.co/xzhih/translategemma-4b-it-llamafile) llamafile for the current operating system, runs it locally, and translates text from one language to another.

This package was taken from [translategemma-ui](https://github.com/xzhih/translategemma-ui) by [xzhih](https://github.com/xzhih). That project is a local TranslateGemma app with a CLI, TUI, and embedded web UI. `gommatranslate` keeps the llamafile download, local runtime, and text translation pieces as a Go library. The original work is released under the MIT License; see `LICENSE`.

```go
import "github.com/eve-learn/gommatranslate"
```

```go
ctx := context.Background()

client, err := gommatranslate.New(gommatranslate.Options{
    ModelID: "q4_k_m",
})
if err != nil {
    return err
}
defer client.Close()

if err := client.Download(ctx, func(p gommatranslate.DownloadProgress) {
    fmt.Printf("\r%.0f%% %s", p.Percent, p.Message)
}); err != nil {
    return err
}

if err := client.Start(ctx); err != nil {
    return err
}

out, err := client.Translate(ctx, "Hello world", "en", "es")
```

`Download` stores the runtime for this machine:

- Windows: `*.llamafile.exe`
- macOS and Linux: an executable `*.llamafile`

The same llamafile binary runs on each of those systems. `Start` launches it as a local HTTP server, picking the next free port when `127.0.0.1:8080` is already in use. `Close` stops the process this client started.

## Translate

```go
out, err := client.Translate(ctx, "Hello world", "en", "ja")

out, err = client.TranslateRequest(ctx, gommatranslate.Request{
    Text:        "Save changes?",
    SourceLang:  "en",
    TargetLang:  "zh-CN",
    Instruction: "Use concise UI wording.",
})

out, err = client.TranslateStream(ctx, "Hello", "en", "fr", func(delta string) error {
    fmt.Print(delta)
    return nil
})
```

Leave `SourceLang` empty, or pass `"auto"`, to detect the source language. `TargetLang` is required. Codes match the model, including region tags such as `zh-CN` and `pt-BR`. `Languages()` returns the full list.

`Translate`, `TranslateRequest`, and `TranslateStream` return `ErrNotRunning` until `Start` has succeeded.

## Models

| ID | Use |
| --- | --- |
| `q4_k_m` | Recommended text model |
| `q6_k` | Higher-quality text model |
| `q8_0` | Largest text model |
| `q8_0_vision` | Text and image runtime |

`AvailableModels()` returns the catalog from the Hugging Face manifest. When that manifest cannot be fetched, the package uses this built-in list. An empty `ModelID` selects the recommended text model.

Downloaded files, logs, and the selected model are stored in `$HOME/.gommatranslate` unless `Options.DataDir` is set.

`example/main.go` is a small program that downloads the default model, starts it, and translates one string.

## Contributing

Development uses Go 1.26 or later, matching `go.mod`.

```sh
go test ./...
go run ./example
```

`go test ./...` covers the public client and the internal packages. `go run ./example` downloads the default model on first run, starts it, and prints one translation. Pass a string to translate something other than the built-in sample: `go run ./example "Save changes?"`.

The import path callers use is `github.com/eve-learn/gommatranslate`. Public types and functions live in the module root (`client.go`, `catalog.go`, `doc.go`). Code under `internal/` is for this module only.

When you change behavior:

- Add or update a test next to the code you changed.
- Keep the public API small. New options belong on `Options` or `Request` when callers need them.
- Downloaded models, logs, and the selected model stay under `$HOME/.gommatranslate` unless a test or example sets `Options.DataDir`.

Open a pull request with a short description of the behavior change and how you tested it. Bug reports and feature requests can go in GitHub issues.

## tag

git tag -a v0.1.1 -m "version 0.1.1" \n
git push origin v0.1.1
