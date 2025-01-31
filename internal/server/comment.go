package server

import (
	"forum-go/internal/models"
	"forum-go/internal/shared"
	"net/http"
	"time"
)

// Implement function : retrieve form values and call AddCommet function in database/comment.go
// Add model instance
func (s *Server) PostCommentHandler(w http.ResponseWriter, r *http.Request) {
	type CommentData struct {
		Content string
		UserID  string
		PostID  string
		Errors  map[string]string
	}

	commentData := CommentData{
		Content: r.FormValue("comment"),
		PostID:  r.FormValue("PostId"),
		Errors:  make(map[string]string),
	}

	if ValidateCommentChar(commentData.Content) {
		commentData.Errors["Comment"] = "Comments must have a maximum of 400 characters"
	}
	if len(commentData.Errors) > 0 {
		post, err := s.db.GetPost(commentData.PostID)
		if err != nil {
			s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		}
		http.Redirect(w, r, "/post/"+post.PostId, http.StatusSeeOther)
		return
	}

	newComment := models.Comment{
		CommentId:    shared.ParseUUID(shared.GenerateUUID()),
		Content:      r.FormValue("comment"),
		CreationDate: time.Now(),
		UserID:       r.FormValue("UserId"),
		PostID:       r.FormValue("PostId"),
		Likes:        0,
		Dislikes:     0,
	}
	err := s.db.AddComment(newComment)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	post, err := s.db.GetPost(newComment.PostID)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
	}
	post.Comments = append(post.Comments, newComment)
	for i, p := range s.posts {
		if p.PostId == post.PostId {
			s.posts[i] = post
			break
		}
	}
	if post.UserID != newComment.UserID {
		newActivity := models.NewActivity(post.UserID, newComment.UserID, string(models.GET_COMMENT), newComment.PostID, newComment.CommentId, newComment.Content)
		s.db.CreateActivity(newActivity)
	}
	newActivity := models.NewActivity(newComment.UserID, newComment.UserID, string(models.COMMENT_CREATED), newComment.PostID, newComment.CommentId, newComment.Content)
	s.db.CreateActivity(newActivity)
	http.Redirect(w, r, "/post/"+newComment.PostID, http.StatusSeeOther)
}

func (s *Server) GetCommentsHandler(w http.ResponseWriter, r *http.Request) {
	// Get all comments
	posts, err := s.db.GetPosts()
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	render(w, r, "../posts", map[string]interface{}{"Posts": posts})
}

func (s *Server) DeleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Delete a comment
	PostID := r.FormValue("PostId")
	CommentID := r.FormValue("CommentId")
	if !s.isLoggedIn(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	SelectedPost := models.Post{}
	for _, post := range s.posts {
		if post.PostId == PostID {
			SelectedPost = post
			break
		}
	}
	if SelectedPost.PostId == "" {
		s.errorHandler(w, r, http.StatusInternalServerError, "Post not found")
		return
	}
	isPresent := false
	SelectedComment := models.Comment{}
	for _, comment := range SelectedPost.Comments {
		if comment.CommentId == CommentID {
			isPresent = true
			SelectedComment = comment
			break
		}
	}

	if !isPresent {
		s.errorHandler(w, r, http.StatusBadRequest, "Comment not found")
		return
	}
	if SelectedComment.UserID != s.getUser(r).UserId && !IsAdmin(r) {
		s.errorHandler(w, r, http.StatusForbidden, "You are not allowed to delete this comment")
		return
	}
	err := s.db.DeleteComment(CommentID)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, "/post/"+PostID, http.StatusSeeOther)
}

func (s *Server) GetNewCommentHandler(w http.ResponseWriter, r *http.Request) {
	// GetNewCommentHandler handles the new comment page
	categories, err := s.db.GetCategories()
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	if !s.isLoggedIn(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	render(w, r, "createPost", map[string]interface{}{"Categories": categories})
}

const MaxCharComment = 400

func ValidateCommentChar(content string) bool {
	// Validate comment character length
	if len(content) > MaxCharComment || len(content) == 0 {
		return true
	}
	return false
}

func (s *Server) EditCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Edit a comment
	CommentID := r.FormValue("CommentId")
	PostId := r.FormValue("PostId")
	UpdatedContent := r.FormValue("UpdatedContent")

	err := s.db.EditComment(CommentID, UpdatedContent)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, "/post/"+PostId, http.StatusSeeOther)
}
