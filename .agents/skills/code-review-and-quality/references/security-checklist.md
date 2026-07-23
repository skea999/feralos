# Security Checklist

- [ ] Auth: rate limit, MFA, least privilege
- [ ] Input: validate ALL(user, API, uploads); sanitize output
- [ ] Data: encrypt at rest; TLS 1.3; mask PII
- [ ] Secrets: env vars only; no .env* .pem .key in repo
- [ ] SQL: no raw queries; parameterized only
- [ ] XSS/CORS: proper headers, CSP
- [ ] Dependencies: scan for CVEs pre-merge
- [ ] Errors: typed+logged; no stack traces to client
- [ ] Container: non-root, read-only FS, drop capabilities
