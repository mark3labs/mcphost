#!/usr/bin/env -S mcphost script
---
# Example script demonstrating TLS skip verify for self-signed certificates
model: "ollama:llama3.2"
provider-url: "https://localhost:8443"
tls-skip-verify: true
max-tokens: 1000
---
Hello! Can you tell me about TLS certificates and why someone might need to skip certificate verification in development environments?