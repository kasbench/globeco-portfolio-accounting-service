package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"sync/atomic"
)

// debugWriter wraps http.ResponseWriter to track WriteHeader calls
type debugWriter struct {
	http.ResponseWriter
	wroteHeader int32
	statusCode  int
}

// WriteHeader tracks all WriteHeader calls and prints stack traces
func (d *debugWriter) WriteHeader(code int) {
	n := atomic.AddInt32(&d.wroteHeader, 1)

	// Store the status code
	d.statusCode = code

	// Always log stack trace for every call
	fmt.Printf("\n=== WriteHeader(%d) called (%d time(s)) ===\n", code, n)
	debug.PrintStack()
	fmt.Println("==========================================")

	// Only call the underlying WriteHeader if this is the first call
	if n == 1 {
		d.ResponseWriter.WriteHeader(code)
	}
}

// Write delegates to the underlying ResponseWriter
func (d *debugWriter) Write(b []byte) (int, error) {
	// Write implicitly triggers WriteHeader(200) if not called yet
	return d.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for middleware compatibility
func (d *debugWriter) Unwrap() http.ResponseWriter {
	return d.ResponseWriter
}

// DebugWriteHeaderMiddleware wraps ResponseWriter to catch all WriteHeader calls
func DebugWriteHeaderMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("\n>>> Starting request: %s %s\n", r.Method, r.URL.Path)

			dw := &debugWriter{
				ResponseWriter: w,
				wroteHeader:    0,
				statusCode:     0,
			}

			next.ServeHTTP(dw, r)

			fmt.Printf("<<< Finished request: %s %s (WriteHeader called %d times, status: %d)\n\n",
				r.Method, r.URL.Path, atomic.LoadInt32(&dw.wroteHeader), dw.statusCode)
		})
	}
}
