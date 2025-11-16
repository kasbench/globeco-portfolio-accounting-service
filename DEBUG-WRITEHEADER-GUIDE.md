# Debugging Superfluous WriteHeader Calls

## Problem

You're seeing thousands of these warnings:
```
2025/11/16 13:18:49 http: superfluous response.WriteHeader call from 
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/request.(*RespWriterWrapper).writeHeader 
(resp_writer_wrapper.go:78)
```

This happens when `WriteHeader()` is called multiple times on the same HTTP response, which is not allowed in Go's `net/http` package.

## Solution Implemented

A debug middleware has been added to trace exactly where these duplicate calls are coming from.

### Files Added/Modified

1. **`internal/api/middleware/debughttp.go`** - Debug middleware that wraps ResponseWriter
2. **`internal/api/routes/routes.go`** - Modified to include the debug middleware
3. **`docs/DEBUG-WRITEHEADER.md`** - Documentation for the debug middleware
4. **`debug-writeheader.sh`** - Script to run the server with debug output
5. **`test-writeheader.sh`** - Script to trigger test requests
6. **`analyze-writeheader.sh`** - Script to analyze the debug output

## How to Use

### Step 1: Start the Server with Debug Middleware

```bash
./debug-writeheader.sh 2>&1 | tee debug.log
```

This will:
- Build the server
- Start it with debug middleware enabled
- Capture all output to both console and `debug.log` file

### Step 2: Trigger Some Requests

In another terminal:

```bash
./test-writeheader.sh
```

This will hit various endpoints to trigger the issue.

### Step 3: Analyze the Output

```bash
./analyze-writeheader.sh debug.log
```

This will show you:
- Which requests had multiple WriteHeader calls
- The most common code paths causing duplicates
- Which endpoints are affected

### Step 4: Read the Stack Traces

Look at `debug.log` for detailed stack traces. For each request, you'll see:

```
>>> Starting request: GET /api/v1/transactions

=== WriteHeader(200) called (1 time(s)) ===
goroutine 123 [running]:
runtime/debug.Stack()
    /usr/local/go/src/runtime/debug/stack.go:24 +0x64
[... first call stack trace ...]

=== WriteHeader(200) called (2 time(s)) ===
goroutine 123 [running]:
runtime/debug.Stack()
    /usr/local/go/src/runtime/debug/stack.go:24 +0x64
[... second call stack trace - THIS IS THE PROBLEM ...]

<<< Finished request: GET /api/v1/transactions (WriteHeader called 2 times, status: 200)
```

The **second stack trace** shows you exactly where the duplicate call is coming from.

## Common Causes & Fixes

### 1. OTel HTTP Wrapper Issue

If you see `otelhttp` in the stack traces, the issue is likely that `otelhttp.NewHandler` is wrapping the response writer and calling `WriteHeader()` internally.

**Possible fixes:**
- Move `otelhttp.NewHandler` to wrap individual handlers instead of the entire router
- Use `otelhttp.WithFilter` to exclude certain endpoints
- Check if middleware is calling `WriteHeader()` before passing to the next handler

### 2. Middleware Chain Issue

If multiple middleware are in the chain, one might be calling `WriteHeader()` and then another tries to call it again.

**Fix:** Ensure middleware only calls `WriteHeader()` once, or doesn't call it at all (let the handler do it).

### 3. Error Handling

Error handling code might call `WriteHeader()` after it was already called by the handler.

**Fix:** Check if headers were already written before calling `WriteHeader()`.

## Likely Culprit in Your Case

Based on the error message mentioning `otelhttp`, the issue is probably in `internal/api/routes/routes.go`:

```go
// This wraps the entire router
return otelhttp.NewHandler(r, config.ServiceName, otelhttp.WithFilter(otelFilter))
```

The `otelhttp.NewHandler` wrapper is likely calling `WriteHeader()` internally, and then your middleware or handlers are calling it again.

## Potential Fixes to Try

### Option 1: Remove otelhttp wrapper temporarily

Comment out the otelhttp wrapper to confirm it's the issue:

```go
// return otelhttp.NewHandler(r, config.ServiceName, otelhttp.WithFilter(otelFilter))
return r
```

### Option 2: Use otelhttp at handler level instead

Instead of wrapping the entire router, wrap individual handlers:

```go
r.Get("/api/v1/transactions", otelhttp.NewHandler(
    http.HandlerFunc(deps.TransactionHandler.GetTransactions),
    "GetTransactions",
).ServeHTTP)
```

### Option 3: Check middleware order

Ensure the debug middleware is the FIRST middleware in the chain to catch all calls:

```go
r.Use(apiMiddleware.DebugWriteHeaderMiddleware()) // Should be first
r.Use(middleware.RequestID)
r.Use(middleware.RealIP)
// ... other middleware
```

## Cleanup

Once you've identified and fixed the issue:

1. Remove the debug middleware from `internal/api/routes/routes.go`:
   ```go
   // Remove this line:
   r.Use(apiMiddleware.DebugWriteHeaderMiddleware())
   ```

2. Optionally delete the debug files:
   ```bash
   rm internal/api/middleware/debughttp.go
   rm docs/DEBUG-WRITEHEADER.md
   rm debug-writeheader.sh test-writeheader.sh analyze-writeheader.sh
   rm DEBUG-WRITEHEADER-GUIDE.md
   rm debug.log
   ```

## Performance Warning

⚠️ **The debug middleware has significant performance impact!**

- Generates stack traces for every WriteHeader call
- Prints to console for every request
- **DO NOT** deploy to production with this enabled

Only use it in development for debugging purposes.

## Need Help?

If you're still stuck after analyzing the output:

1. Look for the common pattern in the second stack trace
2. Check if it's always the same middleware/handler
3. Try the potential fixes above one by one
4. Consider posting the stack traces (with sensitive info removed) for help

Good luck debugging! 🐛🔍
