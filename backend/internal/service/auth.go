package service

import (
	"errors"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v5"

	"nodeforge/internal/model"
	"nodeforge/internal/repository"
)

type AuthService struct {
	repo      *repository.Repo
	jwtSecret []byte
}

func NewAuthService(repo *repository.Repo, jwtSecret string) *AuthService {
	return &AuthService{repo: repo, jwtSecret: []byte(jwtSecret)}
}

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *AuthService) Login(username, password string) (*model.LoginResponse, error) {
	u, err := s.repo.GetUserByUsername(username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})

	tokenStr, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	return &model.LoginResponse{Token: tokenStr, User: *u}, nil
}

func (s *AuthService) EnsureAdmin(adminUser, adminPass string) error {
	if adminUser == "" || adminPass == "" {
		adminUser = envOr("ADMIN_USER", "admin")
		adminPass = envOr("ADMIN_PASS", "admin123")
	}

	_, err := s.repo.GetUserByUsername(adminUser)
	if err == nil {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = s.repo.CreateUser(adminUser, string(hash), "admin", time.Time{}, 0)
	return err
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
