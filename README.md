# llmplaceholder

Mock LLM APIs for local dev and tests. Tenant-scoped OpenAI and Anthropic-compatible endpoints, chaos injection, and MCP support. No API key. No rate limits. Just HTTP.

## Quick start

```bash
go run ./cmd/server
# → http://localhost:8080
```

## Endpoints

### LLM

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/chat/completions` | OpenAI-compatible chat completions |
| POST | `/v1/messages` | Anthropic Messages API |

All LLM endpoints require `X-Tenant-ID` header. Pass `"stream": true` for SSE streaming or `"stream": false` for a single JSON response.

### MCP

| Method | Path | Description |
|--------|------|-------------|
| POST | `/mcp/message` | JSON-RPC 2.0 over HTTP (spec 2025-03-26) |
| GET | `/mcp/sse` | Legacy HTTP+SSE transport (spec 2024-11-05) |

Supported methods: `initialize`, `tools/list`, `tools/call`, `resources/list`, `prompts/list`.

### Admin

| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/admin/tenants` | List / create tenants |
| GET/PUT/DELETE | `/admin/tenants/{id}` | Get state / update state / delete |
| GET/POST | `/admin/tenants/{id}/scenarios` | List / create scenarios |
| DELETE | `/admin/tenants/{id}/scenarios/{sid}` | Delete a scenario |
| GET/PATCH | `/admin/tenants/{id}/settings` | Get / patch settings |
| GET/POST | `/admin/tenants/{id}/chaos` | Get / set chaos profile |

## Usage

### OpenAI SDK (Python)

```python
import openai

client = openai.OpenAI(
    base_url="http://localhost:8080",
    api_key="placeholder",
    default_headers={"X-Tenant-ID": "tenant_ecommerce"},
)

for chunk in client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Show me recent invoices"}],
    stream=True,
):
    print(chunk.choices[0].delta.content or "", end="")
```

### Anthropic SDK (Python)

```python
import anthropic

client = anthropic.Anthropic(
    base_url="http://localhost:8080",
    api_key="placeholder",
    default_headers={"X-Tenant-ID": "tenant_ecommerce"},
)

with client.messages.stream(
    model="claude-opus-4-5",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Show me recent invoices"}],
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)
```

### curl (OpenAI)

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-Tenant-ID: tenant_ecommerce" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Show me recent invoices"}],"stream":true}'
```

### curl (Anthropic)

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "X-Tenant-ID: tenant_ecommerce" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-opus-4-5","max_tokens":1024,"messages":[{"role":"user","content":"Show me recent invoices"}],"stream":true}'
```

## Tenants and scenarios

Each tenant has isolated state, scenarios, and chaos profile. Requests are matched to scenarios by keyword substring. If no tenant scenario matches, falls back to the global registry.

Create a tenant:

```bash
curl -X POST http://localhost:8080/admin/tenants \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"my-tenant","state":{}}'
```

Add a scenario:

```bash
curl -X POST http://localhost:8080/admin/tenants/my-tenant/scenarios \
  -H "Content-Type: application/json" \
  -d '{"keywords":["invoice","billing"],"response":"Here are your recent invoices..."}'
```

## Chaos injection

Profiles: `none`, `rate_limit`, `server_error`, `latency`.

```bash
curl -X POST http://localhost:8080/admin/tenants/my-tenant/chaos \
  -H "Content-Type: application/json" \
  -d '{"profile":"latency"}'
```

## Playground

Open [http://localhost:8080/playground](http://localhost:8080/playground) to manage tenants, fire live requests against all three protocols, and configure chaos profiles visually.
