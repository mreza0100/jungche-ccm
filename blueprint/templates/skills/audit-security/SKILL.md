---
name: audit-security
version: "1.0.0"
description: "Security deep scan — injection, auth, API attack surface, LLM/prompt, sensitive data, crypto, transport, supply-chain. {SACRED_GROUND} data is sacred."
---

# Security — Deep Scan

> {PROJECT_NAME} handles {SENSITIVE_DATA_DESCRIPTION}. A security breach here doesn't just leak data; it {BREACH_IMPACT_DESCRIPTION}. This is not a checkbox — it's a fortress inspection.

<!-- At install time, replace the intro with your project's data sensitivity context.
     Examples:
     - Healthcare: "handles therapy session audio, transcripts, clinical observations"
     - Finance: "handles transaction records, account details, financial histories"
     - Education: "handles student records, assessments, learning analytics" -->

**Trigger:** `security`, `security <scope>`, or when `/audit` routes to security scopes.

**Scopes:** `security` (all), or individual sub-category scopes matching 8A-8I below.

Each sub-category is independent — run only applicable ones based on scope.

**Report format (used across all sub-categories):**

```
SECURITY: {sub-category}/{issue_type}
  Where: {file:line}
  What: {description — what's vulnerable and how}
  Severity: {CRITICAL | HIGH | MEDIUM | LOW}
  Risk: {what an attacker could exploit, especially in a {SENSITIVE_DATA_CONTEXT}}
  Fix: {specific remediation with code pattern}
```

**Severity guide:**

- **CRITICAL:** Sensitive data exposure, missing auth on mutations, hardcoded real credentials, direct prompt injection allowing data exfiltration, unvalidated LLM output executed as code
- **HIGH:** Exception internals reaching users, technology stack disclosure, missing input validation on external boundaries, indirect prompt injection vectors, sensitive data sent to external APIs without safeguards
- **MEDIUM:** Hardcoded enum lists, inconsistent auth patterns, verbose error messages, missing rate limiting, overly permissive CORS, missing security headers
- **LOW:** `X-Powered-By` header, internal IDs in non-sensitive responses, minor naming leaks in non-production code paths

---

## 8A — Information Leakage & Error Exposure

Internal system details leaking to end users through error messages, headers, or responses.

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Run `/audit security` or ask the Professor to hydrate after the codebase has enough code to analyze.
> The Professor will surface this gap: "Knowledge base is empty, waiting for user specification to fill it in."

<!-- Detection patterns filled by RR at install time:
     - Internal error details exposed to users (framework-specific patterns)
     - Technology stack disclosure (package names, URLs in user-visible strings)
     - Debug/development artifacts in production
     - API error verbosity configuration -->

---

## 8B — Injection Attacks

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.

<!-- Detection patterns filled by RR:
     - SQL injection: ORM-specific raw query patterns
     - Command injection: subprocess/exec patterns
     - XSS: framework-specific unsafe rendering
     - Template injection: eval/exec patterns
     - Header/CRLF injection: response header construction
     - Prototype pollution (JS) or equivalent language-specific issues -->

---

## 8C — Authentication & Authorization

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.

<!-- Detection patterns filled by RR:
     - Missing auth on API operations
     - JWT/session token vulnerabilities
     - IDOR (insecure direct object references)
     - RBAC implementation gaps
     - Session/token management
     - Auth bypass mechanisms (magic login, dev shortcuts) -->

---

## 8D — API Attack Surface

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.

<!-- Detection patterns filled by RR:
     - API-specific attack vectors (GraphQL depth bombs, REST mass assignment, etc.)
     - Introspection/documentation exposure in production
     - Batching/rate abuse vectors
     - WebSocket/real-time security
     - Field-level authorization for sensitive data -->

---

## 8E — LLM & Prompt Injection

_Skip this category if the project has no LLM/AI pipeline._

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.

<!-- Detection patterns filled by RR:
     - Direct prompt injection (user data in prompts)
     - Indirect prompt injection (untrusted content flowing to LLM)
     - Prompt leaking (system prompts exposed)
     - LLM output trust (raw output parsed/executed without validation)
     - Excessive agency (agents with unguarded tools)
     - Missing output guardrails
     - RAG/embedding security (multi-tenant isolation)
     - Framework-specific vulnerabilities (LangChain CVEs, etc.)
     - Data poisoning vectors -->

---

## 8F — Sensitive Data Protection

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.

<!-- Detection patterns filled by RR based on {REGULATION} and data sensitivity:
     - Sensitive data in logs
     - Sensitive data in error messages/API responses
     - Sensitive data in URLs/query params/headers
     - Client-side storage security
     - External API data transmission
     - File/media security
     - Audit trail completeness
     - Data deletion / right to erasure
     - Analytics data leakage (e.g., Cerebral precedent for healthcare)
     - Crash report data filtering
     - Special data categories requiring stricter access control
     - Consent management at data processing boundaries -->

---

## 8G — Cryptographic Failures & Secrets Management

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.

<!-- Detection patterns filled by RR:
     - Hardcoded secrets (grep patterns for API key prefixes)
     - Weak password hashing algorithms
     - Insecure random number generation in security contexts
     - JWT/token secret strength
     - Missing encryption (DB connections, API calls)
     - Secrets in version control (.gitignore coverage) -->

---

## 8H — Server & Transport Security

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.

<!-- Detection patterns filled by RR:
     - CORS misconfiguration
     - Missing security headers
     - SSRF vectors
     - Insecure deserialization
     - Rate limiting coverage
     - WebSocket security
     - Debug mode in production -->

---

## 8I — Supply Chain & Dependency Security

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.

<!-- Detection patterns filled by RR:
     - Lock file integrity
     - Known vulnerability patterns for your stack
     - Dependency confusion vectors
     - Post-install scripts
     - Version pinning policy for security-critical deps -->
