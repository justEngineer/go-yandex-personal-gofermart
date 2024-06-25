package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	models "github.com/justEngineer/go-yandex-personal-gofermart/internal/models"

	database "github.com/justEngineer/go-yandex-personal-gofermart/internal/database"
	config "github.com/justEngineer/go-yandex-personal-gofermart/internal/http/server/config"
	logger "github.com/justEngineer/go-yandex-personal-gofermart/internal/logger"
	security "github.com/justEngineer/go-yandex-personal-gofermart/internal/security"
)

type AuthMiddleware struct {
	config    *config.ServerConfig
	appLogger *logger.Logger
	storage   database.Storage
}

func New(config *config.ServerConfig, log *logger.Logger, conn database.Storage) *AuthMiddleware {
	return &AuthMiddleware{config, log, conn}
}

func (a *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			http.Error(w, "Bearer token is empty", http.StatusUnauthorized)
			return
		}

		token, err := security.VerifyToken(tokenString, a.config.SHA256Key)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error while validating token: %s", err.Error()), http.StatusUnauthorized)
			return
		}

		login, err := token.Claims.GetSubject()
		if err != nil {
			http.Error(w, fmt.Sprintf("Error while reading sub field: %s", err.Error()), http.StatusUnauthorized)
			return
		}

		user, err := a.storage.GetUser(r.Context(), login)
		if err != nil {
			http.Error(w, fmt.Sprintf("UnknownUser login %s doesn't exist", login), http.StatusConflict)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), models.UserInfoKey, user.ID)))
	})
}
