# Debug WriteHeader Middleware

## Purpose

This middleware helps trace the source of "superfluous response.WriteHeader call" warnings that occur when `WriteHeader()` is called multiple times on the same response.

## Location

`internal/api/middleware/debughttp.go`

## How It Works

The `DebugWriteHeaderMiddleware` wraps the `http.ResponseWriter` and:

1. Tracks every call to `WriteHeader()`
2. Prints a full stack trace for each call
3. Counts how many times `WriteHeader()` is called per request
4. Only allows the first `WriteHeader()` call to pass through (preventing the actual error)

## Usage

The middleware is currently enabled in `internal/api/routes/routes.go`:

```go
r.Use(apiMiddleware.DebugWriteHeaderMiddleware())
```

## Output Format

For each request, you'll see:

```
>>> Starting request: GET /api/v1/health

=== WriteHeader(200) called (1 time(s)) ===
[stack trace showing where WriteHeader was called]
==========================================

=== WriteHeader(200) called (2 time(s)) ===
[stack trace showing the duplicate call]
==========================================

<<< Finished request: GET /api/v1/health (WriteHeader called 2 times, status: 200)
```

## Finding the Issue

1. Look for requests where `WriteHeader called X times` shows X > 1
2. Compare the stack traces to see which middleware or handler is calling it multiple times
3. The second stack trace will show you the problematic code path

## Common Causes

- Middleware calling `WriteHeader()` and then the handler calling it again
- Error handling code calling `WriteHeader()` after it was already called
- Wrapper middleware (like `otelhttp`) calling it internally
- Multiple middleware layers each trying to set headers

## Removal

Once you've identified and fixed the issue, remove this middleware by deleting the line:

```go
r.Use(apiMiddleware.DebugWriteHeaderMiddleware())
```

from `internal/api/routes/routes.go`.

## Performance Impact

⚠️ **WARNING**: This middleware has significant performance impact due to:
- Stack trace generation on every WriteHeader call
- Console output for every request
- Should ONLY be used in development/debugging

**DO NOT** deploy this to production!
