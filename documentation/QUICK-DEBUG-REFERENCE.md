# Quick Debug Reference

## TL;DR - How to Find the Problem

```bash
# Terminal 1: Start server with debug
./debug-writeheader.sh 2>&1 | tee debug.log

# Terminal 2: Trigger requests
./test-writeheader.sh

# Terminal 1: Stop server (Ctrl+C)

# Analyze the output
./analyze-writeheader.sh debug.log

# Look at debug.log for the second stack trace in each duplicate call
```

## What to Look For

In `debug.log`, find sections like this:

```
=== WriteHeader(200) called (2 time(s)) ===  <-- THIS IS THE PROBLEM
```

The stack trace below this line shows you exactly where the duplicate call is coming from.

## Most Likely Fix

The issue is probably in `internal/api/routes/routes.go` at line 89:

```go
return otelhttp.NewHandler(r, config.ServiceName, otelhttp.WithFilter(otelFilter))
```

Try commenting this out temporarily:

```go
// return otelhttp.NewHandler(r, config.ServiceName, otelhttp.WithFilter(otelFilter))
return r
```

If the warnings disappear, you've confirmed `otelhttp` is the culprit.

## Cleanup After Fixing

Remove this line from `internal/api/routes/routes.go`:

```go
r.Use(apiMiddleware.DebugWriteHeaderMiddleware())
```

## Files You Can Delete After Debugging

- `internal/api/middleware/debughttp.go`
- `docs/DEBUG-WRITEHEADER.md`
- `debug-writeheader.sh`
- `test-writeheader.sh`
- `analyze-writeheader.sh`
- `DEBUG-WRITEHEADER-GUIDE.md`
- `QUICK-DEBUG-REFERENCE.md`
- `debug.log`
