package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/v-starostin/gophermart/internal/model"
)

type contextKey string

const KeyUserID contextKey = "userID"

func Authenticate(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var userID uuid.UUID
			switch r.URL.Path {
			case "/api/user/register":
			case "/api/user/login":
			default:
				authHeader := r.Header.Get("Authorization")
				if len(authHeader) == 0 {
					writeResponse(w, http.StatusUnauthorized, model.Error{Error: "Unauthorized"})
					return
				}
				h := strings.SplitN(authHeader, " ", 2)
				if len(h) != 2 {
					writeResponse(w, http.StatusUnauthorized, model.Error{Error: "Unauthorized"})
					return
				}
				if strings.ToLower(h[0]) != "bearer" {
					writeResponse(w, http.StatusUnauthorized, model.Error{Error: "Unauthorized"})
					return
				}
				token, err := jwt.ParseString(h[1], jwt.WithVerify(jwa.HS256, secret), jwt.WithValidate(true))
				if err != nil {
					writeResponse(w, http.StatusUnauthorized, model.Error{Error: "Unauthorized"})
					return
				}
				userID, err = uuid.Parse(token.Subject())
				if err != nil {
					writeResponse(w, http.StatusInternalServerError, model.Error{Error: "Internal server error"})
					return
				}
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), KeyUserID, userID)))
		})
	}
}

func writeResponse(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{error: "Internal server error"}`))
		return
	}
	w.WriteHeader(code)
	w.Write(b)
}
