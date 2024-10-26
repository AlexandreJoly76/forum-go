package server

import (
	"encoding/base64"
	"forum-go/internal/models"
	"log"
	"math/rand"
	"net/http"
)

func (s *Server) isLoggedIn(r *http.Request) bool {
	user := r.Context().Value(contextKeyUser)
	return user != nil
}
func IsAdmin(r *http.Request) bool {
	user := r.Context().Value(contextKeyUser)
	if user == nil {
		return false
	}
	return user.(models.User).Role == "admin"
}

func generateToken(lenght int) string {
	bytes := make([]byte, lenght)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)

}
