package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/akdandapat/OmniLedger/internal/auth"
	"github.com/akdandapat/OmniLedger/internal/errs"
	"github.com/akdandapat/OmniLedger/internal/model"
	"github.com/akdandapat/OmniLedger/internal/store"
)

const bcryptCost = 12

// AuthService handles user registration, login, and logout.
type AuthService struct {
	store *store.Store
	tm    *auth.TokenManager
}

// NewAuthService returns an AuthService wired to the given store and token manager.
func NewAuthService(s *store.Store, tm *auth.TokenManager) *AuthService {
	return &AuthService{store: s, tm: tm}
}

// Register creates a new user with a bcrypt-hashed password and issues a JWT.
// The first user registered in the database is automatically promoted to system user.
func (as *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.AuthResponse, error) {

	if req.Username == "" || req.Email == "" || req.Password == "" || req.FullName == "" {
		return nil, errs.ErrMissingFields
	}

	existing, _ := as.store.GetUserByUsername(ctx, req.Username)
	if existing != nil {
		return nil, errs.ErrUsernameTaken
	}
	existingEmail, _ := as.store.GetUserByEmail(ctx, req.Email)
	if existingEmail != nil {
		return nil, errs.ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt hash: %w", err)
	}

	isFirst, err := as.store.IsFirstUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("first user check: %w", err)
	}

	user := &model.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		FullName:     req.FullName,
		IsSystemUser: isFirst,
		IsActive:     true,
	}

	if err := as.store.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	acct := &model.Account{
		ID:       uuid.New(),
		UserID:   user.ID,
		Currency: "INR",
		Status:   model.AccountStatusActive,
	}
	if err := as.store.CreateAccount(ctx, acct); err != nil {
		return nil, fmt.Errorf("create default account: %w", err)
	}

	if isFirst {
		log.Printf("[INFO] first user registered as system user: %s", user.Username)
	}

	tokenStr, _, err := as.tm.IssueToken(user.ID, user.Username, user.IsSystemUser)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		User:  *user,
		Token: tokenStr,
	}, nil
}

// Login authenticates a user by username and password and issues a JWT.
func (as *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, errs.ErrMissingFields
	}

	user, err := as.store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, errs.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, errs.ErrAccountNotActive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errs.ErrInvalidCredentials
	}

	tokenStr, _, err := as.tm.IssueToken(user.ID, user.Username, user.IsSystemUser)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		User:  *user,
		Token: tokenStr,
	}, nil
}

// Logout blacklists the current JWT by its JTI so it cannot be reused.
func (as *AuthService) Logout(ctx context.Context, jti string, expiry time.Time) error {
	return as.tm.BlacklistToken(ctx, jti, expiry)
}

// SetTokenCookie writes the JWT into a secure HttpOnly cookie.
// In production (secure=true), SameSite=None is required for cross-domain
// requests between the Vercel frontend and the Render API.
func SetTokenCookie(w http.ResponseWriter, token string, secure bool) {
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "vault_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   int(auth.TokenTTL.Seconds()),
	})
}

// ClearTokenCookie expires the auth cookie immediately.
func ClearTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "vault_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   -1,
	})
}
