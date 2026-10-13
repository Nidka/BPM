package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
	"github.com/getsentry/sentry-go"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"
)

type contextKey string

const (
	requestIDKey contextKey = "request-id"
	identityKey  contextKey = "identity"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}
		writer.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(request.Context(), requestIDKey, requestID)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func IdentityFromContext(ctx context.Context) (contracts.Identity, bool) {
	value, ok := ctx.Value(identityKey).(contracts.Identity)
	return value, ok
}

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					panicType := reflect.TypeOf(recovered).String()
					hub := sentry.CurrentHub().Clone()
					hub.WithScope(func(scope *sentry.Scope) {
						scope.SetTag("panic.type", panicType)
						scope.SetTag("request.id", RequestIDFromContext(request.Context()))
						hub.CaptureMessage("panic recovered")
					})
					logger.Error("panic recovered",
						"request_id", RequestIDFromContext(request.Context()),
						"method", request.Method,
						"path", request.URL.Path,
						"panic_type", panicType,
						"stack", string(debug.Stack()),
					)
					response.InternalError(writer, RequestIDFromContext(request.Context()))
				}
			}()
			next.ServeHTTP(writer, request)
		})
	}
}

func AccessLog(logger *slog.Logger, trustProxyHeaders bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			wrapped := chimiddleware.NewWrapResponseWriter(writer, request.ProtoMajor)
			next.ServeHTTP(wrapped, request)

			logger.Info("http request",
				"request_id", RequestIDFromContext(request.Context()),
				"method", request.Method,
				"path", request.URL.Path,
				"status", wrapped.Status(),
				"bytes", wrapped.BytesWritten(),
				"duration_ms", time.Since(started).Milliseconds(),
				"client_ip", clientIP(request, trustProxyHeaders),
			)
		})
	}
}

func SecurityHeaders(productionLike bool, hstsMaxAgeSeconds int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			headers := writer.Header()
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("X-Frame-Options", "DENY")
			headers.Set("Referrer-Policy", "no-referrer")
			headers.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
			headers.Set("Cache-Control", "no-store")
			if productionLike {
				headers.Set("Strict-Transport-Security", "max-age="+strconv.Itoa(hstsMaxAgeSeconds)+"; includeSubDomains")
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(writer, request)
				return
			}

			_, exactAllowed := allowed[origin]
			_, wildcardAllowed := allowed["*"]
			if !exactAllowed && !wildcardAllowed {
				response.Error(writer, http.StatusForbidden, RequestIDFromContext(request.Context()), response.APIError{
					Code:    "ORIGIN_NOT_ALLOWED",
					Message: "El origen no está permitido.",
				})
				return
			}

			writer.Header().Add("Vary", "Origin")
			if wildcardAllowed {
				writer.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
			writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			writer.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key,If-Match,X-Request-ID")
			writer.Header().Set("Access-Control-Expose-Headers", "ETag,X-Request-ID")
			writer.Header().Set("Access-Control-Max-Age", "600")

			if request.Method == http.MethodOptions {
				writer.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Body != nil {
				request.Body = http.MaxBytesReader(writer, request.Body, maxBytes)
			}
			next.ServeHTTP(writer, request)
		})
	}
}

func Authenticate(verifier contracts.TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if verifier == nil {
				response.AuthenticationUnavailable(writer, RequestIDFromContext(request.Context()))
				return
			}

			scheme, token, found := strings.Cut(strings.TrimSpace(request.Header.Get("Authorization")), " ")
			if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
				unauthorized(writer, request)
				return
			}

			identity, err := verifier.VerifyIDToken(request.Context(), strings.TrimSpace(token))
			if err != nil {
				unauthorized(writer, request)
				return
			}
			ctx := context.WithValue(request.Context(), identityKey, identity)
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu          sync.Mutex
	visitors    map[string]*visitor
	rps         rate.Limit
	burst       int
	lastCleanup time.Time
	trustProxy  bool
}

func NewIPRateLimiter(rps float64, burst int, trustProxyHeaders bool) *IPRateLimiter {
	return &IPRateLimiter{
		visitors:    make(map[string]*visitor),
		rps:         rate.Limit(rps),
		burst:       burst,
		lastCleanup: time.Now(),
		trustProxy:  trustProxyHeaders,
	}
}

func (limiter *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !limiter.allow(clientIP(request, limiter.trustProxy)) {
			writer.Header().Set("Retry-After", "1")
			response.Error(writer, http.StatusTooManyRequests, RequestIDFromContext(request.Context()), response.APIError{
				Code:    "RATE_LIMITED",
				Message: "Se superó el límite de solicitudes.",
			})
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (limiter *IPRateLimiter) allow(ip string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	if now.Sub(limiter.lastCleanup) > time.Minute {
		for key, item := range limiter.visitors {
			if now.Sub(item.lastSeen) > 5*time.Minute {
				delete(limiter.visitors, key)
			}
		}
		limiter.lastCleanup = now
	}

	item, exists := limiter.visitors[ip]
	if !exists {
		item = &visitor{limiter: rate.NewLimiter(limiter.rps, limiter.burst)}
		limiter.visitors[ip] = item
	}
	item.lastSeen = now
	return item.limiter.Allow()
}

func unauthorized(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("WWW-Authenticate", "Bearer")
	response.Unauthenticated(writer, RequestIDFromContext(request.Context()))
}

func clientIP(request *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if value := net.ParseIP(strings.TrimSpace(request.Header.Get("CF-Connecting-IP"))); value != nil {
			return value.String()
		}
		if forwarded := request.Header.Get("X-Forwarded-For"); forwarded != "" {
			if first, _, _ := strings.Cut(forwarded, ","); net.ParseIP(strings.TrimSpace(first)) != nil {
				return strings.TrimSpace(first)
			}
		}
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

func validRequestID(value string) bool {
	if len(value) < 8 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func newRequestID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(buffer)
}
