package authctx

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

// Выносим запись auth-данных в отдельный пакет, чтобы сервисы не зависели от HTTP middleware.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// Флаг администратора живет рядом с userID, потому что это одна область данных запроса.
func WithAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, isAdminKey, isAdmin)
}
