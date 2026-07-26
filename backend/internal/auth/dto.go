package auth

import "github.com/usuario/commander-companion-backend/internal/users"

// LoginRequest es el payload de login con email y password.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// GoogleLoginRequest es el payload para iniciar sesión con Google.
type GoogleLoginRequest struct {
	IDToken string `json:"id_token"`
}

// RefreshRequest es el payload para renovar un access token vencido.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest es el payload para cerrar sesión invalidando el refresh token.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenResponse es la respuesta de login/refresh/google con el par de tokens y el usuario.
type TokenResponse struct {
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token"`
	TokenType    string              `json:"token_type"`
	ExpiresIn    int64               `json:"expires_in"`
	User         *users.UserResponse `json:"user"`
}
