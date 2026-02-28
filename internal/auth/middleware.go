package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/AVGsync/study_flow_api/internal/database"
)

type Middleware struct {
    Secret []byte
    Users  *database.UserRepository
}

func NewMiddleware(secret []byte, users *database.UserRepository) *Middleware {
    return &Middleware{
        Secret: secret,
        Users:  users,
    }
}

func (m *Middleware) Auth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "missing Authorization header", http.StatusUnauthorized)
            return
        }

        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
            http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
            return
        }

        tokenStr := parts[1]

        token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
            if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, jwt.ErrSignatureInvalid
            }
            return m.Secret, nil
        })
				
        if err != nil || !token.Valid {
            http.Error(w, "invalid token", http.StatusUnauthorized)
            return
        }

        claims, ok := token.Claims.(jwt.MapClaims)
        if !ok {
            http.Error(w, "invalid token claims", http.StatusUnauthorized)
            return
        }

        userID, ok := claims["id"].(string)
        if !ok || userID == "" {
            http.Error(w, "user_id missing in token", http.StatusUnauthorized)
            return
        }

        user, err := m.Users.FindByID(r.Context(), userID)
        if err != nil {
            http.Error(w, "user not found", http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), userIDKey, user.ID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
