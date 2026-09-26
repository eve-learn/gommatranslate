# TranslateGemmaUI


TranslateGemmaUI is a local TranslateGemma app with:

- a Go CLI entrypoint
- Bubble Tea TUI mode
- an embedded Web UI served from the same binary
- local model download and activation flows
- streaming text translation output
- image translation for multimodal runtimes

The app downloads packaged TranslateGemma runtimes from Hugging Face on demand and stores them under the user data directory. End users do not need Go or Bun if they install from GitHub Releases or Homebrew.


```bash
cd desktop
bun install
bun run dev
```
### Check the version

```bash
translategemma-ui --version
```

### Manage runtimes from the CLI

```bash
translategemma-ui models list
translategemma-ui models download --id q4_k_m
translategemma-ui models delete --id q4_k_m
```

### Translate text from the CLI

```bash
translategemma-ui translate text \
  --text "Hello world" \
  --source-lang en \
  --target-lang zh-CN
```

### Translate an image from the CLI

```bash
translategemma-ui translate image \
  --file /path/to/image.png \
  --model-id q8_0_vision
```

## External Translation API

TranslateGemmaUI exposes translation endpoints that can be used by third-party apps, browser UIs, or automation scripts.

Base URL when running locally:

- `http://127.0.0.1:8090`

Available endpoints:

- `POST /api/translate` for single-shot text translation
- `POST /api/translate/stream` for streaming text translation
- `POST /api/translate/image` for multipart image translation when the active runtime supports vision
- `GET /healthz` for a simple health check

`/api/translate*` and `/healthz` send permissive CORS headers and answer `OPTIONS` preflight requests, so browser-based clients from another origin can call them directly.

Text endpoints accept either `application/json` or `application/x-www-form-urlencoded` payloads. Use `source_lang`, `target_lang`, `translation_instruction`, and either `input_text` or `text`. If `source_lang` is omitted it defaults to `auto`; if `target_lang` is omitted it defaults to `zh-CN`.

Example JSON translation request:

```bash
curl http://127.0.0.1:8090/api/translate \
  -H 'Content-Type: application/json' \
  -d '{
    "source_lang": "en",
    "target_lang": "zh-CN",
    "input_text": "Hello world",
    "translation_instruction": "Use concise UI wording."
  }'
```

Example response:

```json
{
  "ok": true,
  "output": "你好，世界",
  "message": "Translation completed",
  "messageCode": "translation_completed",
  "history": {
    "id": 1,
    "source": "en",
    "target": "zh-CN",
    "input": "Hello world",
    "output": "你好，世界",
    "when": "09:27:54"
  },
  "count": 1
}
```

Example streaming request:

```bash
curl http://127.0.0.1:8090/api/translate/stream \
  -H 'Content-Type: application/json' \
  -d '{
    "source_lang": "ja",
    "target_lang": "en",
    "text": "こんにちは"
  }'
```

For successful streaming requests, the response is newline-delimited JSON with `status`, `progress`, `delta`, `error`, and `done` event types. Method errors or malformed payloads return regular HTTP error responses instead of an event stream.

Image translation uses `multipart/form-data` with an `image_file` field plus optional `source_lang`, `target_lang`, and `translation_instruction` fields. It accepts JPEG, PNG, and GIF uploads up to 10 MB. If the active runtime does not support vision, the endpoint returns `active_runtime_no_image_support`.

## Runtime Model Source

Default runtime source:

- Hugging Face repo: [xzhih/translategemma-4b-it-llamafile](https://huggingface.co/xzhih/translategemma-4b-it-llamafile)
- Manifest URL: `https://huggingface.co/xzhih/translategemma-4b-it-llamafile/resolve/main/manifest-v1.json`

Current runtime matrix:

- `q4_k_m` for text translation
- `q6_k` for text translation
- `q8_0` for text translation
- `q8_0_vision` for text and image translation

## Data Directory

Default data directory:

- macOS / Linux: `$HOME/.translategemma-ui`
- Windows: `%USERPROFILE%\\.translategemma-ui`

Created structure:

```text
<user-home>/.translategemma-ui/
  config.json
  history.json
  state.json
  logs/
  runtimes/
  tmp/
```
