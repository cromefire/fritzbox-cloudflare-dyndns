package util

import "net/http"

type TokenHandler struct {
	h     http.Handler
	token string
}

func NewTokenHandler(h http.Handler, token string) TokenHandler {
	return TokenHandler{
		h:     h,
		token: token,
	}
}

func (t TokenHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	tokenParam := request.URL.Query().Get("token")

	if tokenParam != t.token {
		http.Error(writer, "invalid token", http.StatusUnauthorized)
	} else {
		t.h.ServeHTTP(writer, request)
	}
}
