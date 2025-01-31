package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"forum-go/internal/models"
	"forum-go/internal/shared"

	"golang.org/x/crypto/bcrypt"
)

//////////////////////////////////////////////////////////////////
///////////////////////////// GOOGLE /////////////////////////////
//////////////////////////////////////////////////////////////////

// handles the Google login process by constructing a URL for Google OAuth2 authentication. It includes
// the client ID, redirect URL, response type, scope, and state parameters in the URL. Finally, it
// redirects the user to this URL using an HTTP temporary redirect response.
func (s *Server) GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	googleClientID := shared.GetEnv("googleClientID")
	googleRedirectURL := shared.GetEnv("googleRedirectURL")

	url := "https://accounts.google.com/o/oauth2/auth?client_id=" + googleClientID +
		"&redirect_uri=" + googleRedirectURL +
		"&response_type=code&scope=email%20profile&state=state"
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (s *Server) GoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	googleClientID := shared.GetEnv("googleClientID")
	googleClientSecret := shared.GetEnv("googleClientSecret")
	googleRedirectURL := shared.GetEnv("googleRedirectURL")
	// Gets the authorization code from the query string
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code missing", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for an access token
	// Making an HTTP POST request to the Google OAuth2 token endpoint to exchange an
	// authorization code for an access token. It is sending the client ID, client secret, redirect URI,
	// grant type, and authorization code as form data in the request.
	tokenResp, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"client_id":     {googleClientID},
		"client_secret": {googleClientSecret},
		"redirect_uri":  {googleRedirectURL},
		"grant_type":    {"authorization_code"},
		"code":          {code},
	})
	if err != nil {
		http.Error(w, "Exchanging the token failed : "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tokenResp.Body.Close()

	// Check the HTTP status
	if tokenResp.StatusCode != http.StatusOK {
		_, _ = io.ReadAll(tokenResp.Body)
		http.Error(w, "Error exchanging token", http.StatusInternalServerError)
		return
	}

	// Decode the token response
	var tokenData map[string]interface{}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		http.Error(w, "Error parsing the token : "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Verify that the access token is present
	accessToken, ok := tokenData["access_token"]
	if !ok || accessToken == nil {
		http.Error(w, "Access token missing or invalid", http.StatusInternalServerError)
		return
	}

	accessTokenStr, ok := accessToken.(string)
	if !ok {
		http.Error(w, "Access token is not a valid string", http.StatusInternalServerError)
		return
	}

	// Use the access token to fetch user information creating a new HTTP GET request
	// to the Google OAuth2 userinfo endpoint with a nil request body. It then sets the
	// Authorization header with the access token retrieved as a string.

	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		http.Error(w, "Error creating user request : "+err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+accessTokenStr)

	// Create an HTTP client to send requests. An HTTP client is used to send requests to a server and receive responses.
	// It manages the connection and handles the communication over the HTTP protocol.
	client := &http.Client{}
	// The Do method executes the request and returns the response or an error.
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Error fetching user information: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check the HTTP status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error fetching user information: %s", body)
		http.Error(w, "Error fetching user information", http.StatusInternalServerError)
		return
	}

	// Decode the user information
	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "Error parsing user information : "+err.Error(), http.StatusInternalServerError)
		log.Println("JSON error when parsing the user information :", err)
		return
	}
	log.Printf("User information: %+v\n", userInfo)

	// Fetch the user's email and name
	email := userInfo["email"].(string)
	name := userInfo["name"].(string)

	// Verify that the email is unique
	IsUnique, err := s.db.FindEmailUser(email)
	if err != nil {
		http.Error(w, "Error checking the user : "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !IsUnique {
		// Fetch the user from the database
		user, err := s.db.FindUserByEmail(email)
		if err != nil {
			http.Error(w, "Error fetching the user : "+err.Error(), http.StatusInternalServerError)
			return
		}

		if user.Role == "ban" {
			render(w, r, "login", map[string]interface{}{"Error": "You are banned", "email": email})
			return
		}
		if user.Provider != "google" {
			render(w, r, "login", map[string]interface{}{"Error": "Email already used by another provider", "email": email})
			return
		}

		userID := shared.ParseUUID(shared.GenerateUUID())

		// Create a session for the user
		expiration := time.Now().Add(time.Hour)
		cookie := http.Cookie{
			Name:    s.SESSION_ID,
			Value:   userID,
			Expires: expiration,
			Path:    "/",
		}
		user.SessionId = sql.NullString{String: userID, Valid: true}
		user.SessionExpire = sql.NullTime{Time: expiration, Valid: true}
		err = s.db.UpdateUser(user)
		if err != nil {
			s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		http.SetCookie(w, &cookie)
		// Redirect the user to the home page
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	password := shared.GenerateUUID().String()
	// Hash the password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Create a new user
	user := models.User{
		Username:     name,
		Email:        email,
		Password:     string(passwordHash),
		Role:         "user",
		CreationDate: time.Now(),
		UserId:       shared.ParseUUID(shared.GenerateUUID()),
		Provider:     "google",
	}
	err = s.db.CreateUser(user)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.users = append(s.users, user)
	userID := shared.ParseUUID(shared.GenerateUUID())

	// Create a session for the user
	expiration := time.Now().Add(time.Hour)
	cookie := http.Cookie{
		Name:    s.SESSION_ID,
		Value:   userID,
		Expires: expiration,
		Path:    "/",
	}
	user.SessionId = sql.NullString{String: userID, Valid: true}
	user.SessionExpire = sql.NullTime{Time: expiration, Valid: true}
	err = s.db.UpdateUser(user)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, &cookie)
	// Redirect the user to the home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

//////////////////////////////////////////////////////////////////
///////////////////////////// GITHUB /////////////////////////////
//////////////////////////////////////////////////////////////////

// GithubLoginHandler initiates the GitHub OAuth flow.
func (s *Server) GithubLoginHandler(w http.ResponseWriter, r *http.Request) {
	GitHubclientID := shared.GetEnv("GitHubClientID")
	GitHubredirectURI := shared.GetEnv("GitHubredirectURI")
	authURL := "https://github.com/login/oauth/authorize?client_id=" + GitHubclientID +
		"&redirect_uri=" + url.QueryEscape(GitHubredirectURI) +
		"&scope=user:email"
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// GithubCallbackHandler handles the callback from GitHub.
func (s *Server) GithubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	GitHubclientID := shared.GetEnv("GitHubClientID")
	GitHubclientSecret := shared.GetEnv("GitHubClientSecret")
	GitHubredirectURI := shared.GetEnv("GitHubredirectURI")
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code is missing", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for an access token
	tokenResp, err := http.PostForm("https://github.com/login/oauth/access_token", url.Values{
		"client_id":     {GitHubclientID},
		"client_secret": {GitHubclientSecret},
		"redirect_uri":  {GitHubredirectURI},
		"code":          {code},
	})
	if err != nil {
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		http.Error(w, "Token exchange failed", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		http.Error(w, "Error reading token response", http.StatusInternalServerError)
		return
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, "Error parsing token response", http.StatusInternalServerError)
		return
	}

	accessToken := values.Get("access_token")
	if accessToken == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect) // Restart the OAuth flow
		return
	}

	// Fetch user information
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "Error fetching user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "Error parsing user info", http.StatusInternalServerError)
		return
	}

	// Extract user details
	_, emailOk := userInfo["email"].(string)
	username, usernameOk := userInfo["login"].(string)

	if !usernameOk || username == "" {
		http.Error(w, "Failed to retrieve username", http.StatusInternalServerError)
		return
	}
	email, errMail := getMail(accessToken)
	if (errMail != nil) && !emailOk || email == "" {
		errMsg := fmt.Sprintf("Error occured :%s %s", email, errMail)
		render(w, r, "login", map[string]interface{}{"Error": errMsg})
		return
	}

	// Check if the email already exists in the database
	IsUnique, err := s.db.FindEmailUser(email)
	if err != nil {
		http.Error(w, "Error checking user existence", http.StatusInternalServerError)
		return
	}

	if !IsUnique {
		user, err := s.db.FindUserByEmail(email)
		if err != nil {
			http.Error(w, "Error fetching user", http.StatusInternalServerError)
			return
		}

		if user.Role == "ban" {
			render(w, r, "login", map[string]interface{}{"Error": "You are banned", "email": email})
			return
		}
		if user.Provider != "github" {
			render(w, r, "login", map[string]interface{}{"Error": "Email already used by another provider", "email": email})
			return
		}

		// Automatically log the user in by creating a session
		userID := shared.ParseUUID(shared.GenerateUUID())
		expiration := time.Now().Add(time.Hour)
		cookie := http.Cookie{
			Name:    s.SESSION_ID,
			Value:   userID,
			Expires: expiration,
			Path:    "/",
		}
		user.SessionId = sql.NullString{String: userID, Valid: true}
		user.SessionExpire = sql.NullTime{Time: expiration, Valid: true}
		err = s.db.UpdateUser(user)
		if err != nil {
			s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Create a new user if the email is not found
	password := shared.GenerateUUID().String()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	user := models.User{
		Username:     username,
		Email:        email,
		Password:     string(passwordHash),
		Role:         "user",
		CreationDate: time.Now(),
		UserId:       shared.ParseUUID(shared.GenerateUUID()),
		Provider:     "github",
	}
	err = s.db.CreateUser(user)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Automatically log the new user in
	userID := user.UserId
	expiration := time.Now().Add(time.Hour)
	cookie := http.Cookie{
		Name:    s.SESSION_ID,
		Value:   userID,
		Expires: expiration,
		Path:    "/",
	}
	user.SessionId = sql.NullString{String: userID, Valid: true}
	user.SessionExpire = sql.NullTime{Time: expiration, Valid: true}
	err = s.db.UpdateUser(user)
	if err != nil {
		s.errorHandler(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// getMail retrieves the user's primary email from GitHub.
func getMail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch emails")
	}

	var emails []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if primary, ok := email["primary"].(bool); ok && primary {
			if emailAddr, ok := email["email"].(string); ok {
				return emailAddr, nil
			}
		}
	}
	return "", fmt.Errorf("no primary email found")
}
