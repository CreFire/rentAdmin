package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"rentadmin/src/services"

	"github.com/gin-gonic/gin"
)

// OpenIDKey is the gin context key holding the authenticated mini program openid.
const OpenIDKey = "openid"

func bearerToken(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

// AdminAuth requires `Authorization: Bearer <adminToken>` on admin endpoints.
func AdminAuth(adminToken string) gin.HandlerFunc {
	expected := []byte(adminToken)
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" || subtle.ConstantTimeCompare([]byte(token), expected) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin authorization required"})
			return
		}
		c.Next()
	}
}

// MPAuth validates the mini program token and stores the openid under OpenIDKey.
func MPAuth(auth *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
			return
		}
		openid, err := auth.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.Set(OpenIDKey, openid)
		c.Next()
	}
}

// CORS allows only the configured origins; requests without an Origin header (same origin, mini program) pass through.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
