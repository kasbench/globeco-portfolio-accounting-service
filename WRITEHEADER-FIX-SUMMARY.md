# WriteHeader Issue - Root Cause and Fix

## Problem Identified

Thousands of warnings:
```
http: superfluous response.WriteHeader call from 
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/request.(*RespWriterWrapper).writeHeader
```

## Root Cause

The issue was in `internal/api/routes/routes.go`. The `otelhttp.NewHandler` was wrapping the **entire router** as the outermost layer:

```go
// WRONG - This causes duplicate WriteHeader calls
return otelhttp.NewHandler(r, config.ServiceName, otelhttp.WithFilter(otelFilter))
```

### Why This Caused the Problem

1. Handler calls `WriteHeader(200)` → goes through middleware chain
2. Logging middleware's wrapper calls `WriteHeader(200)` on its wrapped writer
3. Eventually reaches the actual `http.ResponseWriter`
4. Response completes and returns back up the stack
5. **`otelhttp` wrapper (outermost layer) then calls `WriteHeader()` again** ❌

The `otelhttp` wrapper was **outside** the middleware chain, so it had its own response writer that tried to call `WriteHeader()` after the inner layers had already called it.

## The Fix

Move `otelhttp` **inside** the middleware chain instead of wrapping the entire router:

```go
// CORRECT - Add otelhttp as middleware in the chain
r.Use(func(next http.Handler) http.Handler {
    return otelhttp.NewHandler(next, config.ServiceName, otelhttp.WithFilter(otelFilter))
})

// Return the router directly (not wrapped)
return r
```

### Why This Works

Now the call flow is:

1. Request enters middleware chain
2. Goes through RequestID, RealIP, Recoverer, etc.
3. Reaches **otelhttp middleware** (inside the chain)
4. Goes through logging middleware
5. Reaches handler
6. Handler calls `WriteHeader(200)` **once**
7. Propagates back through the chain cleanly

Each middleware wrapper properly delegates to the next layer, and `WriteHeader()` is only called once.

## Changes Made

### File: `internal/api/routes/routes.go`

**Before:**
```go
// Setup routes
setupHealthRoutes(r, deps.HealthHandler)
setupAPIRoutes(r, deps)
setupDocumentationRoutes(r, deps.SwaggerHandler)
setupMetricsRoute(r, config.EnableMetrics)

// Wrap router with OTel HTTP handler for tracing
return otelhttp.NewHandler(r, config.ServiceName, otelhttp.WithFilter(otelFilter))
```

**After:**
```go
// Add OTel middleware as part of the chain (not wrapping the entire router)
r.Use(func(next http.Handler) http.Handler {
    return otelhttp.NewHandler(next, config.ServiceName, otelhttp.WithFilter(otelFilter))
})

// Setup routes
setupHealthRoutes(r, deps.HealthHandler)
setupAPIRoutes(r, deps)
setupDocumentationRoutes(r, deps.SwaggerHandler)
setupMetricsRoute(r, config.EnableMetrics)

// Return the router directly (not wrapped)
return r
```

## Testing

1. Build and run the server:
   ```bash
   go build -o server ./cmd/server && ./server
   ```

2. Make some requests:
   ```bash
   curl http://localhost:8087/api/v1/health
   curl http://localhost:8087/api/v1/transactions
   curl http://localhost:8087/api/v1/balances
   ```

3. Check the logs - the superfluous WriteHeader warnings should be **gone** ✅

## Cleanup

Once you've verified the fix works, you can remove the debug middleware:

1. Remove from `internal/api/routes/routes.go`:
   ```go
   // Remove these lines:
   r.Use(apiMiddleware.DebugWriteHeaderMiddleware())
   ```

2. Optionally delete debug files:
   ```bash
   rm internal/api/middleware/debughttp.go
   rm docs/DEBUG-WRITEHEADER.md
   rm DEBUG-WRITEHEADER-GUIDE.md
   rm QUICK-DEBUG-REFERENCE.md
   rm WRITEHEADER-FIX-SUMMARY.md
   rm debug-writeheader.sh test-writeheader.sh analyze-writeheader.sh
   ```

## Key Takeaway

When using middleware wrappers like `otelhttp.NewHandler`:
- ✅ **DO** add them as middleware in the chain using `r.Use()`
- ❌ **DON'T** wrap the entire router at the end

This ensures proper delegation through the middleware chain and prevents duplicate `WriteHeader()` calls.
