package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// CallbackResult holds the result from the OAuth callback.
type CallbackResult struct {
	Code  string
	Error string
}

const (
	// CallbackPort is the fixed port for the local OAuth callback server.
	// This must match the redirect URI registered with the OAuth client.
	CallbackPort = 19837

	// RedirectURI is the full redirect URI registered with the OAuth client.
	RedirectURI = "http://127.0.0.1:19837/callback"
)

// StartCallbackServer starts a local HTTP server on the fixed callback port
// to receive the OAuth authorization code callback. It returns a channel that
// will receive the authorization code.
//
// The server automatically shuts down after receiving the callback or when
// the context is cancelled.
func StartCallbackServer(ctx context.Context) (<-chan CallbackResult, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", CallbackPort))
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server on port %d: %w", CallbackPort, err)
	}
	resultChan := make(chan CallbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!DOCTYPE html>
<html><body>
<h1>Authentication Failed</h1>
<p>Error: %s</p>
<p>You can close this window.</p>
</body></html>`, errParam)
			resultChan <- CallbackResult{Error: errParam}
			return
		}

		if code == "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!DOCTYPE html>
<html><body>
<h1>Authentication Failed</h1>
<p>No authorization code received.</p>
<p>You can close this window.</p>
</body></html>`)
			resultChan <- CallbackResult{Error: "no authorization code received"}
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html><body>
<h1>Authentication Successful</h1>
<p>You can close this window and return to the terminal.</p>
</body></html>`)
		resultChan <- CallbackResult{Code: code}
	})

	server := &http.Server{Handler: mux}

	// Start serving in a goroutine
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			resultChan <- CallbackResult{Error: fmt.Sprintf("callback server error: %v", err)}
		}
	}()

	// Shutdown when context is cancelled
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return resultChan, nil
}
