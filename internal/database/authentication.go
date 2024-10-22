package database

import (
	"forum-go/internal/models"

	"golang.org/x/crypto/bcrypt"
)

func (s *service) CreateUser(User models.User) error {

	query := "INSERT INTO User (user_id, email, username, password, role, creation_date, session_id, session_expire) VALUES (?, ?, ?, ?, ?, ?,?,?)"
	_, err := s.db.Exec(query, User.UserId, User.Email, User.Username, User.Password, User.Role, User.CreationDate, User.SessionId, User.SessionExpired)
	return err
}

func (s *service) GetUsers() ([]models.User, error) {
	query := "SELECT * FROM User"
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []models.User{}
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.UserId, &user.Email, &user.Username, &user.Password, &user.Role, &user.CreationDate, &user.SessionId, &user.SessionExpired); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (s *service) GetUser(email, password string) (models.User, error) {
	query := "SELECT * FROM User WHERE email=?"
	row := s.db.QueryRow(query, email)
	var user models.User
	if err := row.Scan(&user.UserId, &user.Email, &user.Username, &user.Password, &user.Role, &user.CreationDate, &user.SessionId, &user.SessionExpired); err != nil {
		return models.User{}, err
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *service) FindEmailUser(email string) (bool, error) {
	query := "SELECT * FROM User WHERE email=?"
	row := s.db.QueryRow(query, email)
	var user models.User
	err := row.Scan(&user.UserId, &user.Email, &user.Username, &user.Password, &user.Role, &user.CreationDate, &user.SessionId, &user.SessionExpired)
	if err != nil {
		return true, nil
	}
	return false, nil
}

func (s *service) DeleteUser(id string) error {
	query := "DELETE FROM User WHERE user_id=?"
	_, err := s.db.Exec(query, id)
	return err
}

//hue