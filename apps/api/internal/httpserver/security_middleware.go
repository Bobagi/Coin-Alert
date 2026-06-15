package httpserver

import (
	"log"
	"net/http"
	"net/url"
	"strings"
)

// maxRequestBodyBytes caps the size of any request body we are willing to read. The JSON payloads
// this API accepts are tiny (a few fields), so 1 MiB is generous while still blocking memory-abuse.
const maxRequestBodyBytes int64 = 1 << 20

// SecurityMiddleware wraps the application router with two cheap, broad defenses:
//
//   - a request-body size cap (http.MaxBytesReader), so no handler can be made to read an
//     unbounded body; and
//   - a same-origin check on state-changing methods (POST/PUT/DELETE/PATCH). Browsers always
//     attach an Origin header to cross-site requests of these methods, so rejecting a mismatched
//     Origin blocks CSRF even if the SameSite cookie attribute is ever relaxed or bypassed. When no
//     Origin is present we fall back to the Referer; a request with neither (e.g. a server-to-server
//     client) is allowed through, since CSRF requires a victim browser that *would* send one.
//
// allowedOrigin is the scheme://host the SPA is served from (derived from APP_BASE_URL).
func SecurityMiddleware(next http.Handler, allowedOrigin string) http.Handler {
	normalizedAllowedOrigin := normalizeOrigin(allowedOrigin)

	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Body != nil {
			request.Body = http.MaxBytesReader(responseWriter, request.Body, maxRequestBodyBytes)
		}

		if isStateChangingMethod(request.Method) && normalizedAllowedOrigin != "" {
			if !requestIsSameOrigin(request, normalizedAllowedOrigin) {
				log.Printf("rejected cross-origin %s %s (origin=%q referer=%q)", request.Method, request.URL.Path, request.Header.Get("Origin"), request.Header.Get("Referer"))
				writeJSONError(responseWriter, http.StatusForbidden, "Cross-origin request rejected.")
				return
			}
		}

		next.ServeHTTP(responseWriter, request)
	})
}

func isStateChangingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}

// requestIsSameOrigin reports whether the request demonstrably comes from our own origin. It trusts
// the Origin header when present, then the Referer; a request carrying neither is treated as
// same-origin (not a browser-driven CSRF vector).
func requestIsSameOrigin(request *http.Request, allowedOrigin string) bool {
	if origin := request.Header.Get("Origin"); origin != "" {
		return normalizeOrigin(origin) == allowedOrigin
	}
	if referer := request.Header.Get("Referer"); referer != "" {
		return normalizeOrigin(referer) == allowedOrigin
	}
	return true
}

// normalizeOrigin reduces a URL (or bare origin) to a lowercase scheme://host[:port] string so two
// origins can be compared for equality. Returns "" when it cannot be parsed.
func normalizeOrigin(rawValue string) string {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return ""
	}
	parsed, parseError := url.Parse(trimmed)
	if parseError != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}
