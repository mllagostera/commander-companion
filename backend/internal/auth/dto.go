package auth

import "github.com/usuario/commander-companion-backend/internal/users"

// LoginRequest is the login payload with email and password.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// GoogleLoginRequest is the payload for signing in with Google.
type GoogleLoginRequest struct {
	IDToken string `json:"id_token"`
}

// RefreshRequest is the payload for renewing an expired access token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest is the payload for logging out by invalidating the refresh token.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenResponse is the login/refresh/google response with the token pair and the user.
type TokenResponse struct {
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token"`
	TokenType    string              `json:"token_type"`
	ExpiresIn    int64               `json:"expires_in"`
	User         *users.UserResponse `json:"user"`
}
