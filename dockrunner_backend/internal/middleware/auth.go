package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"user_id"`
	Login  string `json:"login"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := parseToken(c.GetHeader("Authorization"), secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token claims",
			})
			return
		}

		setUserContext(c, claims)
		c.Next()
	}
}

func AgentAuth(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		expectedHeader := "Bearer " + expectedToken

		tokenNotConfigured := expectedToken == ""
		tokenMismatch := subtle.ConstantTimeCompare([]byte(authorization), []byte(expectedHeader)) != 1

		if tokenNotConfigured || tokenMismatch {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid agent token",
			})
			return
		}

		c.Next()
	}
}

func OptionalAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := parseToken(c.GetHeader("Authorization"), secret)
		if err != nil {
			c.Next()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.Next()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("claims", claims)

		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	allowedRoles := make(map[string]struct{}, len(roles))

	for _, role := range roles {
		allowedRoles[role] = struct{}{}
	}

	return func(c *gin.Context) {
		role := c.GetString("role")

		if _, ok := allowedRoles[role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			return
		}

		c.Next()
	}
}

func parseToken(header string, secret string) (*jwt.Token, error) {
	tokenString, err := extractBearerToken(header)
	if err != nil {
		return nil, err
	}

	return jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
	)
}

func extractBearerToken(header string) (string, error) {
	if header == "" {
		return "", jwt.ErrTokenMalformed
	}

	parts := strings.SplitN(header, " ", 2)

	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", jwt.ErrTokenMalformed
	}

	return parts[1], nil
}

func setUserContext(c *gin.Context, claims *Claims) {
	c.Set("user_id", claims.UserID)
	c.Set("login", claims.Login)
	c.Set("role", claims.Role)
	c.Set("claims", claims)
}
