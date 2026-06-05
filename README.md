# llmplaceholder

Mock LLM APIs for local dev and testing. Drop-in replacement for OpenAI, Anthropic, and MCP — tenant-scoped, with chaos injection and no API keys required.

**[llmplaceholder.com](https://llmplaceholder.com)**

## Features

- **OpenAI-compatible** — `/v1/chat/completions` with streaming
- **Anthropic-compatible** — `/v1/messages` with streaming
- **MCP support** — JSON-RPC 2.0 and HTTP+SSE transports
- **Multi-tenant** — isolated state, scenarios, and chaos per tenant
- **Scenario matching** — keyword-based response routing, falls back to global registry
- **Chaos injection** — `rate_limit`, `server_error`, `latency` profiles
- **Auth** — GitHub OAuth + API token support
- **Playground** — visual UI to manage tenants, fire requests, configure chaos

## Usage

### OpenAI SDK (Python)

```python
import openai

client = openai.OpenAI(
    base_url="https://llmplaceholder.com",
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
    base_url="https://llmplaceholder.com",
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

### curl

```bash
# OpenAI
curl -X POST https://llmplaceholder.com/v1/chat/completions \
  -H "X-Tenant-ID: tenant_ecommerce" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Show me recent invoices"}],"stream":true}'

# Anthropic
curl -X POST https://llmplaceholder.com/v1/messages \
  -H "X-Tenant-ID: tenant_ecommerce" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-opus-4-5","max_tokens":1024,"messages":[{"role":"user","content":"Show me recent invoices"}],"stream":true}'
```

## Tenants and scenarios

Each tenant has isolated state, scenarios, and chaos profile. Requests match scenarios by keyword substring. No match falls back to the global tenant.

```bash
# Create tenant
curl -X POST https://llmplaceholder.com/public/tenants \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"my-tenant"}'

# Add scenario
curl -X POST https://llmplaceholder.com/public/tenants/my-tenant/scenarios \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"keywords":["invoice","billing"],"response":"Here are your recent invoices..."}'
```

## Chaos injection

Profiles: `none`, `rate_limit`, `server_error`, `latency`.

```bash
curl -X POST https://llmplaceholder.com/public/tenants/my-tenant/chaos \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"profile":"latency"}'
```
