package auth

import "context"

type ctxKey string

const (
    userIDKey  ctxKey = "userID"
    isAdminKey ctxKey = "isAdmin"
)

func UserIDFromContext(ctx context.Context) (string, bool) {
    v, ok := ctx.Value(userIDKey).(string)
    if !ok {
        return "", false
    }
    return v, true
}

func IsAdminFromContext(ctx context.Context) bool {
    v, ok := ctx.Value(isAdminKey).(bool)
    return ok && v
}
