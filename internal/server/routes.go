package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func (s *Server) RegisterRoutes() http.Handler {

	mux := http.NewServeMux()

	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))

	mux.HandleFunc("/", s.HomePageHandler)

	mux.HandleFunc("GET /login", s.GetLoginHandler)
	mux.HandleFunc("POST /login", s.PostLoginHandler)

	mux.HandleFunc("POST /logout", s.LogoutHandler)

	mux.HandleFunc("GET /register", s.GetRegisterHandler)
	mux.HandleFunc("POST /register", s.PostRegisterHandler)

	mux.HandleFunc("GET /users", s.GetUsersHandler)
	mux.HandleFunc("GET /delete/users/{id}", s.DeleteUsersHandler)

	mux.HandleFunc("GET /posts", s.GetPostsHandler)
	mux.HandleFunc("GET /posts/create", s.GetNewPostHandler)
	mux.HandleFunc("POST /posts/create", s.PostNewPostsHandler)
	mux.HandleFunc("POST /posts/delete/{id}", s.DeletePostsHandler)

	mux.HandleFunc("GET /categories", s.GetCategoriesHandler)
	mux.HandleFunc("POST /categories/add", s.PostCategoriesHandler)
	mux.HandleFunc("POST /categories/delete/{id}", s.DeleteCategoriesHandler)
	mux.HandleFunc("POST /categories/edit/{id}", s.EditCategoriesHandler)

	mux.HandleFunc("GET /post/{id}", s.GetPostHandler)
	mux.HandleFunc("POST /comment/delete/{id}", s.DeleteCommentHandler)
	mux.HandleFunc("POST /comment/edit/{id}", s.EditCommentHandler)
	mux.HandleFunc("POST /post/comment", s.PostCommentHandler)
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("GET /adminPanel", s.AdminPanelHandler)
	mux.HandleFunc("GET /report", s.reportHandler)

	return s.authenticate(mux)
}

func (s *Server) reportHandler(w http.ResponseWriter, r *http.Request) {
	if !s.isLoggedIn(r) || !IsAdmin(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	users, err := s.db.GetUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, "report", map[string]interface{}{"users": users})

}

func (s *Server) AdminPanelHandler(w http.ResponseWriter, r *http.Request) {
	if !s.isLoggedIn(r) || !IsAdmin(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	users, err := s.db.GetUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, "adminPanel", map[string]interface{}{"users": users})
}

func (s *Server) HomePageHandler(w http.ResponseWriter, r *http.Request) {
	err := error(nil)
	s.categories, err = s.db.GetCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.posts, err = s.db.GetPosts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, "home", map[string]interface{}{"Categories": s.categories, "Posts": s.posts})
}
func (s *Server) HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	resp := make(map[string]string)
	resp["message"] = "Hello World"
	for k, v := range r.Header {
		resp[k] = fmt.Sprintf("%v", v)
	}
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("error handling JSON marshal. Err: %v", err)
	}

	_, _ = w.Write(jsonResp)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResp, err := json.Marshal(s.db.Health())

	if err != nil {
		log.Fatalf("error handling JSON marshal. Err: %v", err)
	}

	_, _ = w.Write(jsonResp)
}
