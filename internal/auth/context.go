package auth

import "context"

type ctxKey string

const userIDKey ctxKey = "userID"

func UserIDFromContext(ctx context.Context) (string, bool) {
    v, ok := ctx.Value(userIDKey).(string)
    if !ok {
        return "", false
    }
    return v, true
}
