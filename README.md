# xAI OpenAI-Compatible Proxy

OpenAI-compatible proxy server for xAI Grok API with HTTP/2. Converts OpenAI API requests to xAI backend format and returns standardized OpenAI responses.

## Features

- Full OpenAI API compatibility (`/v1/chat/completions`, `/v1/models`)
- HTTP/2 transport to xAI backend
- Streaming and non-streaming responses
- CORS support for browser-based clients
- Standardized OpenAI error format
- Proper `finish_reason` handling in streams
- Parameter passthrough (temperature, top_p, max_tokens, penalties, stop)

## Project Structure

```
x-ai-proxy-server/
├── bin/
│   ├── proxy-windows-amd64.exe
│   ├── proxy-linux-amd64
│   ├── proxy-darwin-amd64
│   └── proxy-darwin-arm64
├── cmd/
│   └── proxy/
│       └── main.go
├── internal/
│   ├── client/
│   │   └── xai.go
│   ├── handlers/
│   │   └── chat.go
│   └── models/
│       ├── openai.go
│       └── xai.go
├── config.json.example
├── go.mod
└── README.md
```

## Prerequisites

- Go 1.24+
- Valid xAI session cookies (from console.x.ai)

## Build

Pre-built binaries are in `bin/`:

- `proxy-windows-amd64.exe`
- `proxy-linux-amd64`
- `proxy-darwin-amd64`
- `proxy-darwin-arm64`

To rebuild:

```bash
go build -o bin/proxy ./cmd/proxy
```

## Configuration

All settings (host, port, api_key, headers) are in `config.json`. Copy `config.json.example` and edit. Headers (including Cookie) must be updated when Cloudflare tokens expire.

## Run

```bash
# Windows
bin\proxy-windows-amd64.exe

# Linux / macOS
./bin/proxy-linux-amd64
```

Copy `config.json.example` → `config.json`, edit, then run. `config.json` is gitignored.

### API Key Authentication

Optional `api_key` field in `config.json`:

```json
{
  "api_key": "sk-xxx",
  ...
}
```

- Empty or omitted: no auth required (default)
- Set: clients must send `Authorization: Bearer <api_key>`
- Invalid/missing key: `401 {"error":{"code":"invalid_api_key"}}`

## Endpoints

### POST /v1/chat/completions

OpenAI-compatible chat completions.

**Request:**
```json
{
  "model": "grok-4.3",
  "messages": [
    {"role": "system", "content": "You are helpful"},
    {"role": "user", "content": "Hello"}
  ],
  "temperature": 0.7,
  "top_p": 0.95,
  "max_tokens": 1000,
  "stream": false,
  "stop": ["END"],
  "presence_penalty": 0.1,
  "frequency_penalty": 0.1
}
```

**Non-stream response:**
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "grok-4.3",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "..."},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
}
```

**Stream response:** SSE format with `data: {...}\n\n` chunks, final chunk has `finish_reason: "stop"`, terminated by `data: [DONE]\n\n`.

### GET /v1/models

Returns available models.

**Response:**
```json
{
  "object": "list",
  "data": [
    {"id": "grok-build-0.1", "object": "model", "created": 1234567890, "owned_by": "x.ai"},
    {"id": "grok-4.3", "object": "model", "created": 1234567890, "owned_by": "x.ai"},
    {"id": "grok-4.20-multi-agent-0309", "object": "model", "created": 1234567890, "owned_by": "x.ai"},
    {"id": "grok-4.20-0309-reasoning", "object": "model", "created": 1234567890, "owned_by": "x.ai"},
    {"id": "grok-4.20-0309-non-reasoning", "object": "model", "created": 1234567890, "owned_by": "x.ai"}
  ]
}
```

## Usage Examples

### Python (openai SDK)

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:60443/v1",
    api_key="not-needed"
)

# Non-stream
response = client.chat.completions.create(
    model="grok-4.3",
    messages=[{"role": "user", "content": "Hello"}]
)
print(response.choices[0].message.content)

# Stream
stream = client.chat.completions.create(
    model="grok-4.3",
    messages=[{"role": "user", "content": "Count to 3"}],
    stream=True
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### curl

```bash
# Non-stream
curl -X POST http://localhost:60443/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"grok-4.3","messages":[{"role":"user","content":"Hi"}],"stream":false}'

# Stream
curl -X POST http://localhost:60443/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"grok-4.3","messages":[{"role":"user","content":"Hi"}],"stream":true}'

# Models
curl http://localhost:60443/v1/models
```

### JavaScript (fetch)

```javascript
const res = await fetch("http://localhost:60443/v1/chat/completions", {
  method: "POST",
  headers: {"Content-Type": "application/json"},
  body: JSON.stringify({
    model: "grok-4.3",
    messages: [{role: "user", content: "Hello"}]
  })
});
const data = await res.json();
console.log(data.choices[0].message.content);
```

## HTTP/2 Transport

Uses Go's standard `http.Transport` with `ForceAttemptHTTP2: true`:

```go
client: &http.Client{
    Transport: &http.Transport{
        ForceAttemptHTTP2:    true,
        MaxIdleConns:         1,
        IdleConnTimeout:      90 * time.Second,
        TLSHandshakeTimeout:  30 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
    },
    Timeout: 120 * time.Second,
}
```

This matches Postman behavior and bypasses Cloudflare 403 errors that occur with `golang.org/x/net/http2.Transport`.

## Error Handling

All errors returned in OpenAI format:

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error" | "api_error" | "server_error",
    "param": "...",
    "code": "method_not_allowed" | "invalid_json" | "upstream_error" | "..."
  }
}
```

**Error codes:**
- `405`: Method not allowed (non-POST to completions)
- `400`: Invalid JSON in request body
- `502`: Upstream xAI API error (wrapped response)
- `500`: Streaming not supported or internal error

## Supported Models

Models exposed via `/v1/models` (actual availability depends on xAI backend):

- `grok-build-0.1`
- `grok-4.3`
- `grok-4.20-multi-agent-0309`
- `grok-4.20-0309-reasoning`
- `grok-4.20-0309-non-reasoning`

## Limitations

- Usage tokens always return `0` (xAI backend does not provide token counts in SSE)
- Some OpenAI parameters (n>1, certain stop behaviors) may be ignored if xAI backend does not support them
- Cookie refresh required when Cloudflare tokens expire (403 responses)

## Troubleshooting

**403 Forbidden:** Cookie expired or IP mismatch. Refresh cookies from browser.

**Connection refused:** Server not running or wrong port.

**Duplicate text in stream:** Check `xai.go:ParseSSEStream` — ensure only `response.output_text.delta` events are processed.

**Missing finish_reason:** Verify `streamResponse` emits final chunk with `finish_reason: "stop"` before `[DONE]`.

**CORS errors in browser:** Server sets `Access-Control-Allow-Origin: *` — verify preflight OPTIONS returns 200.

## Testing

Manual test matrix (all passing):

| Flow | Status |
|------|--------|
| Models endpoint | ✓ |
| Non-stream completion | ✓ |
| Stream completion + finish_reason | ✓ |
| CORS preflight (OPTIONS) | ✓ |
| Method not allowed (405) | ✓ |
| Invalid JSON (400) | ✓ |
| Multi-turn conversation | ✓ |
| All 5 models routing | ✓ (2 active on backend) |
| Missing model field | ✓ |
| Empty messages | ✓ |
| Unicode/special chars | ✓ |
| Concurrent requests | ✓ |
| Long messages (5K chars) | ✓ |
| System messages | ✓ |
| All OpenAI params | ✓ |

## License

Internal use only.