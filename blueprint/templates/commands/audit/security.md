---
name: audit:security
version: "1.0.0"
description: "Security deep scan — injection, auth, {API_PROTOCOL}, LLM/prompt, {SENSITIVE_DATA}, health endpoints, crypto, secrets, transport, supply-chain. {DOMAIN_ADJ} data is sacred."
---

# Security — Deep Scan

> {PROJECT_NAME} handles {SESSION_NOUN} records, {SUBJECT_NOUN} data, and {DOMAIN_ADJ} observations — among the most sensitive data categories in this domain. A security breach here doesn't just leak emails; it exposes someone's most private information to whoever should never see it. This is not a checkbox — it's a fortress inspection. {DOMAIN_SAFETY}

**Trigger:** `security`, `security <scope>`.

**Scopes:** `security` (all), `injection`, `auth`, `graphql`, `llm`, `prompt`, `phi`, `health`, `crypto`, `secrets`, `transport`, `supply-chain`.

Each sub-category is independent — run only applicable ones based on scope.

**Report format (used across all sub-categories):**

```
SECURITY: {sub-category}/{issue_type}
  Where: {file:line}
  What: {description — what's vulnerable and how}
  Severity: {CRITICAL | HIGH | MEDIUM | LOW}
  Risk: {what an attacker could exploit, especially in a {DOMAIN_ADJ}/{SENSITIVE_DATA} context}
  Fix: {specific remediation with code pattern}
```

**Severity guide:**

- **CRITICAL:** {SUBJECT_NOUN} data exposure, missing auth on mutations, hardcoded real credentials, direct prompt injection allowing data exfiltration, unvalidated LLM output executed as code
- **HIGH:** Exception internals reaching users, technology stack disclosure, missing input validation on external boundaries, indirect prompt injection vectors, {SENSITIVE_DATA} sent to external APIs without safeguards, JWT algorithm confusion
- **MEDIUM:** Hardcoded enum lists, inconsistent auth patterns, verbose error messages, missing rate limiting, overly permissive CORS, missing security headers
- **LOW:** `X-Powered-By` header, internal IDs in non-sensitive responses, minor naming leaks in non-production code paths

---

## 8A — Information Leakage & Error Exposure

Internal system details leaking to end users through error messages, headers, or responses.

**How to detect:**

1. **Internal error details exposed to users:** Grep for `type(e).__name__` or `str(e)` stored in user-visible fields, `e.message` or raw exception strings in {API_PROTOCOL} error responses, Python/Node exception class names in any client-visible field, `stack`/`stackTrace` in API responses.

2. **Technology stack disclosure:** {ORM}/{AI_FRAMEWORK} driver or library names (e.g. ORM connection URLs, DB driver names, LLM SDK names) in user-visible strings, {API_FRAMEWORK} server-identity headers, {API_PROTOCOL} error `extensions` with internal paths.

3. **Debug/development artifacts in production:** `breakpoint()` in Python projects, `TODO`/`FIXME` describing security workarounds, commented-out auth checks, `NODE_ENV === 'development'` blocks disabling security.

4. **Verbose {API_PROTOCOL} errors:** Check `maskedErrors` configuration, custom error formatting, `extensions` field leaking resolver paths.

**Files to check:** {API_FRAMEWORK} error middleware, {API_FRAMEWORK} config, all catch blocks in resolvers/services, AI/pipeline-project exception handlers (if the roster has one).

---

## 8B — Injection Attacks

**How to detect:**

1. **SQL injection:** `sql.raw(` with user values, template literals with variables in {ORM}, Python `text(` or `execute(` with f-strings/.format() (e.g. in {AI_FRAMEWORK}).

2. **Command injection:** `child_process.exec(` with string args containing variables, `subprocess.run(`/`os.system(` in Python.

3. **XSS:** `dangerouslySetInnerHTML` in UI projects, `innerHTML` assignments, LLM-generated content rendered without sanitization.

4. **Template injection:** `vm.runInContext(` with user/LLM content, Python `eval(`/`exec(`/`compile(` with variable args.

5. **Header/CRLF injection:** `res.setHeader(`/`res.redirect(` with user-controlled values.

6. **Prototype pollution:** `Object.assign({}, userInput)`, vulnerable lodash versions (<4.17.21), missing `__proto__`/`constructor` filtering.

**Files to check:** Resolver input handling, AI/pipeline-project DB query code, shell command construction, UI components rendering AI content.

---

## 8C — Authentication & Authorization

**How to detect:**

1. **Missing auth on {API_PROTOCOL} operations:** Every mutation/sensitive query must have auth middleware. Mutations without auth (except login/register/health) are CRITICAL.

2. **JWT vulnerabilities:** `algorithms` not explicitly specified (algorithm none attack), `jwt.decode` without verify, hardcoded JWT secret, fallback patterns (`|| 'default'`), missing `expiresIn`, `ignoreExpiration: true`.

3. **IDOR:** Resolvers taking `id` without verifying requesting user has permission. {SUBJECT_NOUN} record access without {USER_NOUN}-{SUBJECT_NOUN} relationship check.

4. **Broken RBAC:** Role checks using string comparison instead of enum, role settable by user (mass assignment).

5. **Session/token management:** Missing refresh token rotation, no invalidation on password change, tokens in URLs/query params.

6. **Magic login bypass:** `MAGIC_LOGIN_EMAIL` must check `FEATURE_FLAG_MAGIC_LOGIN=true`. Not enabled in `.env.local` (only `.env.demo`).

**Files to check:** JWT middleware, resolver auth patterns, auth service, feature flag config, token storage in UI/client projects, env files.

---

## 8D — {API_PROTOCOL} Attack Surface

**How to detect:**

1. **Introspection enabled in production:** Must be disabled. Check for environment-based toggling.

2. **Query depth & complexity bombs:** Check for `depthLimit`/`queryComplexity`/`maxDepth`/`costAnalysis` plugins.

3. **Batching attacks:** Check batch size limits — 1000 login mutations in one request = brute force.

4. **Mass assignment:** Input types accepting privilege or ownership fields (`role`, `isAdmin`, an `{ORG_UNIT}` id, `createdAt`).

5. **Subscription/{REALTIME_PROTOCOL} security:** Auth on {REALTIME_PROTOCOL} connect, subscription resolver auth checks, connection limits.

6. **Field-level authorization:** {SENSITIVE_DATA} fields need per-field auth checks.

7. **Field suggestion leakage:** "Did you mean 'socialSecurityNumber'?" in errors.

**Files to check:** {API_FRAMEWORK} config, schema type definitions, input types, {REALTIME_PROTOCOL} setup, subscription resolvers.

---

## 8E — LLM & Prompt Injection

**How to detect:**

1. **Direct prompt injection:** f-strings/`.format()` in prompt construction with user data, `HumanMessage(content=` with raw user/source text.

2. **Indirect prompt injection:** Source text containing "Ignore previous instructions..." flowing to LLM without hardening. Check system prompt instructs LLM to treat source text as DATA.

3. **Prompt leaking:** System prompts in user-accessible files/responses, raw LLM responses returned unfiltered, prompts logged at INFO level.

4. **LLM output trust:** `json.loads()`/`JSON.parse()` on raw LLM output without validation, LLM output inserted into SQL/shell, LLM output rendered as HTML.

5. **Excessive agency:** {AI_FRAMEWORK} agents with unguarded DB write/API call tools, missing `max_iterations`/`recursion_limit`.

6. **Missing output guardrails:** No content filtering, missing `aiGenerated` flags on AI output.

7. **RAG/embedding security:** Multi-{SUBJECT_NOUN} data in same vector space without isolation, search results not scoped by auth context.

8. **{AI_FRAMEWORK} vulnerabilities:** Check the framework's published CVE advisories and pin to a patched version. Check for `verbose=True`, tracing with {SENSITIVE_DATA}, `PythonREPL`/`ShellTool`/`BashProcess` imports.

9. **Data poisoning via vector store:** Source text not scanned before embedding, no knowledge file integrity verification.

**Files to check:** Chain definitions, prompt templates, {AI_FRAMEWORK} agent/tool definitions, LLM response processing, {QUEUE} consumer, UI rendering of AI content, `pyproject.toml` for the {AI_FRAMEWORK} version.

---

## 8F — {SENSITIVE_DATA} & {DOMAIN_ADJ} Data Protection

**How to detect:**

1. **{SUBJECT_NOUN} data in logs:** `logger.*` calls with {SENSITIVE_DATA} fields, `JSON.stringify({subject})`/`str(result)` without filtering. NEVER log {SENSITIVE_DATA} — only anonymized IDs.

2. **{SENSITIVE_DATA} in error messages/API responses:** Error strings interpolating {subject}/{session} data, AI/pipeline-project step status `reason` with raw source excerpts.

3. **{SENSITIVE_DATA} in URLs/query params/headers:** {SUBJECT_NOUN} names in URL construction — URLs are logged by proxies, stored in browser history.

4. **{SENSITIVE_DATA} in client-side storage:** `localStorage`/`AsyncStorage` storing {subject} data instead of a secure store (`expo-secure-store`).

5. **{SENSITIVE_DATA} sent to external APIs:** {TRANSCRIPTION_SERVICE} must use the {DATA_REGION} endpoint. Check data minimization in LLM prompts. Verify HTTPS.

6. **Audio file security:** Auth checks on audio URLs, unpredictable file paths (UUIDs not sequential IDs), signed URLs with short expiry.

7. **Missing audit trail:** No logging of who accessed what {SUBJECT_NOUN} data when.

8. **Data deletion / right to erasure:** Cascade deletion across all stores ({session}s, transcripts, analysis, audio, embeddings, external service copies).

9. **{SENSITIVE_DATA} in analytics (the $7M FTC-fine precedent):** Check for `gtag`/`fbq`/`mixpanel`/`amplitude`/`posthog`/`segment` sending {DOMAIN_ADJ} data. Marketing site must not link visitors to {DOMAIN_ADJ} identities.

10. **{SENSITIVE_DATA} in crash reports:** Sentry `beforeSend` must strip {SENSITIVE_DATA}, breadcrumbs must not capture {API_PROTOCOL} response bodies with {SENSITIVE_DATA}, session replay disabled on {DOMAIN_ADJ} screens.

11. **{DOMAIN_ADJ}-notes special protection:** Under {DOMAIN_STANDARDS} and {REGULATION} Article {N}, {DOMAIN_ADJ} notes need stricter access controls than general {SESSION_NOUN} data. {ROLE_SUPER} role should NOT access note content.

12. **Consent management:** Consent flags verified before audio recording, AI analysis, vector embedding, external service calls. Check at every data processing boundary.

**Files to check:** All log statements, error handlers, {TRANSCRIPTION_SERVICE} integration, LLM prompt construction, UI/client storage, media handling, URL construction, `package.json` for analytics SDKs, marketing site for tracking pixels, notes resolver access control, consent flag checks.

---

## 8G — Cryptographic Failures & Secrets Management

**How to detect:**

1. **Hardcoded secrets:** Grep for `sk-`, `{LLM_KEY_PREFIX}`, `pk_live_`, `AKIA`, `Bearer `, `password = "`, base64-encoded credentials.

2. **Weak password hashing:** `md5(`/`sha1(`/`sha256(` for passwords. Must use bcrypt (>= cost 10) or argon2id.

3. **Insecure random:** `Math.random()` in security contexts. Must use `crypto.randomBytes`/`crypto.randomUUID`. Python: `random` module in security contexts — use `secrets`.

4. **JWT secret strength:** Minimum length, no fallback defaults, fail hard if missing.

5. **Missing encryption:** DB connections without SSL/TLS, `http://` in API base URLs.

6. **Secrets in version control:** Check `.gitignore` includes `.env*`/`*.pem`/`*.key`, `.env.example` has placeholders not real values.

**Files to check:** All `.env*` files, auth/JWT config, password hashing, token generation, DB connection config.

---

## 8H — Server & Transport Security

**How to detect:**

1. **CORS misconfiguration:** `origin: '*'` or `origin: true` allows any website. Check credential + wildcard combination.

2. **Missing security headers:** Check for `helmet()` middleware. Missing CSP/HSTS/X-Frame-Options.

3. **SSRF:** `fetch(`/`axios(` where URL comes from user input. Check {TRANSCRIPTION_SERVICE} webhook URL validation.

4. **Insecure deserialization:** Python `pickle.load`/`yaml.load` without SafeLoader, `node-serialize`.

5. **Rate limiting:** Check for {API_FRAMEWORK} rate-limit middleware. Login endpoint must be rate-limited. {API_PROTOCOL} mutations per-user limited.

6. **{REALTIME_PROTOCOL} security:** Auth on connect, message schema validation, connection limits per user.

7. **Debug mode in production:** `NODE_ENV` checks disabling security, source maps served in prod builds.

**Files to check:** {API_FRAMEWORK} middleware stack, CORS config, HTTP client usage, {REALTIME_PROTOCOL} config, rate limiting, environment config branching.

---

## 8I — Supply Chain & Dependency Security

**How to detect:**

1. **Lock file integrity:** Verify each roster project has its package manager's lock file committed (one per `{project}`).

2. **Known vulnerabilities:** Check for `node-serialize` (RCE), `lodash` < 4.17.21 (prototype pollution), `jsonwebtoken` < 9.0.0 (algorithm confusion), `express` < 4.19.2 (open redirect).

3. **Dependency confusion:** Check for scoped packages (`@{project}/...`), registry pinning.

4. **Post-install scripts:** Check `package.json` for `preinstall`/`postinstall`/`prepare` scripts.

5. **Pinned vs floating versions:** Security-critical deps (auth, crypto, validation) should use exact pins.

**Files to check:** `package.json` files, lock files, `.npmrc`, CI config for audit steps.
