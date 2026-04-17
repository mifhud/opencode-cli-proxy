# OpenCode CLI Proxy

Turn your local [`opencode`](https://opencode.ai) CLI into an OpenAI-compatible API.

Any client that speaks the OpenAI API — Cherry Studio, NextChat, Cursor, the official
OpenAI SDKs, or your own scripts — can point at this proxy and use the models already
configured in your local `opencode` installation. No API keys to hand out, no remote
service to run.

- Repository: <https://github.com/mifhud/opencode-cli-proxy>
- Default endpoint: `http://127.0.0.1:18080/v1`

---

## How it works

The proxy does not call a remote OpenCode HTTP API. It shells out to the local CLI:

```text
OpenAI-compatible client
        |
        |  POST /v1/chat/completions
        v
+---------------------------+
|  opencode-cli-proxy       |
|  - gateway API key auth   |
|  - model alias mapping    |
|  - OpenAI <-> CLI mapping |
|  - SSE stream conversion  |
+---------------------------+
        |
        |  opencode run --format json --model <id> [--attach <server>]
        v
+---------------------------+
|  local opencode CLI       |
|  - opencode models        |
|  - opencode run           |
+---------------------------+
```

Each request becomes one `opencode run` invocation. Messages are flattened into a
single prompt with role labels:

```text
[SYSTEM]
You are a helpful assistant.

[USER]
Explain goroutines in one paragraph.
```

The JSONL events printed by `opencode run --format json` are parsed and translated
back into OpenAI-style JSON, or streamed as OpenAI-compatible SSE chunks.

## Features

- OpenAI-compatible endpoints: `/v1/models`, `/v1/chat/completions`, `/v1/completions`
- Streaming (`stream: true`) over SSE, with keep-alive comments for long tool runs
- Accepts `messages[].content` as a plain string **or** an array of content parts
- Gateway API keys with per-key model allow-lists
- Model alias mapping (expose `glm-latest`, run `opencode-go/glm-5.1`)
- Optional `--attach` support for an existing `opencode serve` instance
- Native desktop GUI built with Fyne (macOS / Windows / Linux)
- npm launcher for install-and-run without a Go toolchain

## Requirements

1. Go 1.22+ (only when building from source)
2. `opencode` installed and working:

   ```bash
   opencode models
   opencode run --format json --model <model> "reply with exactly: ok"
   ```

3. Node.js 18+ (only for the npm launcher)

If `opencode` is not on your `PATH`, set `upstream.binary` to its absolute path.

## Quick start

### 1. Configure

```bash
cp configs/config.example.yaml configs/config.local.yaml
```

Edit at least `models` and `keys`. A minimal working config:

```yaml
server:
  host: 127.0.0.1
  port: 18080

upstream:
  binary: opencode

models:
  glm-latest: opencode-go/glm-5.1

accounts:
  default:
    auth_mode: local

keys:
  sk-local-demo:
    account: default
```

> `configs/config.local.yaml` is git-ignored so machine-specific paths and keys stay
> out of version control.

### 2. Run

```bash
make run
# or, with an explicit config file:
go run -buildvcs=false ./cmd/server -config configs/config.local.yaml
```

The gateway listens on `http://127.0.0.1:18080` by default.

### 3. Verify

```bash
curl http://127.0.0.1:18080/health
curl http://127.0.0.1:18080/v1/models -H "Authorization: Bearer sk-local-demo"
```

## API

| Method | Path                  | Auth | Description                          |
| ------ | --------------------- | ---- | ------------------------------------ |
| GET    | `/`                   | no   | Service banner and route list        |
| GET    | `/health`             | no   | Health probe                         |
| GET    | `/v1`                 | no   | API info                             |
| GET    | `/v1/models`          | yes  | Models reported by `opencode models` |
| POST   | `/v1/chat/completions`| yes  | Chat completions (JSON or SSE)       |
| POST   | `/v1/completions`     | yes  | Legacy text completions (JSON or SSE)|

Authenticated routes expect `Authorization: Bearer <gateway-key>`, where the key is
one of the entries under `keys` in your config.

### Chat completion

```bash
curl http://127.0.0.1:18080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-local-demo" \
  -d '{
    "model": "glm-latest",
    "messages": [
      { "role": "system", "content": "You are concise." },
      { "role": "user", "content": "Introduce yourself in one sentence." }
    ]
  }'
```

### Streaming

```bash
curl -N http://127.0.0.1:18080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-local-demo" \
  -d '{
    "model": "glm-latest",
    "stream": true,
    "messages": [
      { "role": "user", "content": "Explain Go interfaces briefly." }
    ]
  }'
```

Streams are standard SSE: `data: {"object":"chat.completion.chunk",...}` per delta,
then a final chunk carrying `finish_reason`, then `data: [DONE]`.

### Content parts

Both forms are accepted:

```json
{ "role": "user", "content": "plain text" }
```

```json
{
  "role": "user",
  "content": [
    { "type": "text", "text": "first part" },
    { "type": "text", "text": "second part" }
  ]
}
```

Text parts are joined; non-text parts are ignored.

## Using the proxy from other apps

| Setting   | Value                          |
| --------- | ------------------------------ |
| Base URL  | `http://127.0.0.1:18080/v1`    |
| API key   | any key from your config       |
| Model     | any alias from your `models` map |

- **Cherry Studio** — provider type *OpenAI Compatible*, use the values above.
- **NextChat** — *Custom Endpoint* set to the Base URL.
- **Cursor** — add an OpenAI-compatible provider with the Base URL and key.
- **OpenAI SDK** — point `base_url` at the proxy:

  ```python
  from openai import OpenAI

  client = OpenAI(base_url="http://127.0.0.1:18080/v1", api_key="sk-local-demo")
  print(client.chat.completions.create(
      model="glm-latest",
      messages=[{"role": "user", "content": "hello"}],
  ).choices[0].message.content)
  ```

## Configuration

Full example: [`configs/config.example.yaml`](configs/config.example.yaml).

```yaml
server:
  host: 127.0.0.1
  port: 18080
  read_timeout: 15s
  write_timeout: 0s

upstream:
  binary: opencode
  attach: ""
  timeout: 120s

models:
  glm-latest: opencode-go/glm-5.1
  glm-5: opencode-go/glm-5

accounts:
  default:
    auth_mode: local
    token: ""

keys:
  sk-local-demo:
    account: default
    allowed_models:
      - glm-latest

mapping:
  temperature:
    target_min: 0
    target_max: 1

rate_limit:
  enabled: false
  rpm: 60
```

| Field                     | Description                                                        |
| ------------------------- | ------------------------------------------------------------------ |
| `server.host` / `port`    | Bind address. Defaults: `0.0.0.0:8080`                             |
| `server.read_timeout`     | HTTP read timeout. Default `15s`                                   |
| `server.write_timeout`    | HTTP write timeout. Keep `0s` for long streams                     |
| `upstream.binary`         | `opencode` executable name or absolute path                        |
| `upstream.attach`         | Optional `opencode serve` address passed as `--attach`             |
| `upstream.timeout`        | Accepted for forward compatibility; not enforced yet               |
| `models`                  | Alias → real model ID. The alias is what clients send             |
| `accounts`                | Upstream credential profiles (`local`, `bearer_token`, `iam_token`)|
| `keys`                    | Gateway API keys → account and optional `allowed_models`           |
| `mapping.temperature`     | Rescales incoming `temperature` into `[target_min, target_max]` (see limitations) |
| `rate_limit`              | Config surface only; not enforced yet                              |

Environment variables are expanded in the config file with `${VAR}` syntax, which is
handy for keeping upstream tokens out of the file:

```yaml
accounts:
  default:
    auth_mode: bearer_token
    token: ${OPENCODE_UPSTREAM_TOKEN}
```

## Desktop app

A Fyne GUI wraps the same gateway and config file.

```bash
make desktop          # run
make build-desktop    # build ./opencode-desktop
```

The window exposes config path, listen host/port, `opencode` binary, attach server,
gateway key, default model, allowed models, and the client base URL.

## CLI launcher (npm)

```bash
npm install -g opencode-cli-proxy
opencode-proxy setup     # download binary, create config
opencode-proxy start     # run in background
opencode-proxy status
opencode-proxy stop
```

| Command             | Purpose                                   |
| ------------------- | ----------------------------------------- |
| `init`              | Create a default config                   |
| `setup`             | Download the binary and create config     |
| `download`          | Download or refresh the platform binary   |
| `start` / `stop` / `restart` | Manage the background gateway     |
| `status`            | Print gateway status                      |
| `install-service`   | Enable auto-start (macOS LaunchAgent)     |
| `uninstall-service` | Remove auto-start                         |
| `paths`             | Show config, binary, and log locations    |

State lives in `~/.opencode-cli-proxy/` (`config.yaml`, `bin/`, `logs/`, `run/`).

The launcher downloads release binaries from this repository's GitHub Releases for
the current `package.json` version. If no release asset exists for your platform,
either build from source (`make build`) or point the downloader at a URL:

```bash
OPENCODE_PROXY_BINARY_URL=https://example.com/opencode-cli-proxy-linux-amd64 \
  opencode-proxy download
```

## Docker

```bash
docker build -t opencode-cli-proxy .
docker run --rm -p 18080:18080 \
  -v "$PWD/configs:/app/configs" \
  opencode-cli-proxy
```

The image runs the server against `/app/configs/config.example.yaml`; mount your own
config and adjust `server.host` to `0.0.0.0`. Note that the container also needs an
`opencode` binary and its credentials available inside the image.

## Building

```bash
make build            # ./server
make build-desktop    # ./opencode-desktop
make release-dist     # cross-platform binaries into dist/
make test
```

Release assets produced by `make release-dist`:

```text
opencode-cli-proxy-darwin-arm64
opencode-cli-proxy-darwin-amd64
opencode-cli-proxy-linux-amd64
opencode-cli-proxy-windows-amd64.exe
checksums-v<version>.txt
```

`npm run preflight` checks those artifacts before publishing, and `make npm-pack`
creates the npm tarball. Publishing is manual: build `dist/`, attach the files to a
GitHub Release tagged `v<version>`, then `npm publish`.

## Project structure

```text
cmd/
  server/            HTTP server entry point
  desktop/           Fyne desktop app
configs/
  config.example.yaml
internal/
  adapter/           OpenAI request/response/stream mapping
  app/               Gateway lifecycle
  config/            Config loading and validation
  domain/            Shared protocol types
  openai/            HTTP handlers
  server/            Router and middleware
  upstream/          Local opencode CLI integration
lib/                 npm launcher (config, download, process, service)
scripts/             Release, preflight, and postinstall helpers
bin/                 `opencode-proxy` entry point
e2e/                 End-to-end gateway tests
```

## Limitations

- Prompt mapping is text-oriented: roles are kept as labels, but tool calls, images,
  and other structured parts are not forwarded to the CLI.
- `max_tokens`, `stop`, and `temperature` are accepted for client compatibility;
  only `temperature` is rescaled per config, and none are passed to `opencode run` yet.
- `upstream.timeout` and `rate_limit` are config placeholders, not enforced.
- `/v1/models` mirrors `opencode models`, so it may list models that are not mapped in
  config; unmapped models are rejected on request.
- No audit log, metrics, or usage accounting beyond what the CLI reports.

## License

No `LICENSE` file is included yet, and `package.json` declares `UNLICENSED`. Add a
license before distributing this project publicly.
