package users

import "time"

// RegisterRequest representa la información necesaria para registrar un usuario.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateProfileRequest es el payload para actualizar el perfil propio. Por ahora
// solo cubre el username de Moxfield; futuros campos de perfil se agregan acá.
type UpdateProfileRequest struct {
	MoxfieldUsername string `json:"moxfield_username"`
}

// ChangePasswordRequest es el payload de POST /users/:id/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// VerifyEmailRequest es el payload de POST /auth/verify-email.
type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// ResendVerificationRequest es el payload de POST /auth/resend-verification.
type ResendVerificationRequest struct {
	Email string `json:"email"`
}

// UserResponse es el DTO que se envía al cliente, sin datos sensibles (como el hash de contraseña).
type UserResponse struct {
	ID               string    `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	CreatedAt        time.Time `json:"created_at"`
	MoxfieldUsername *string   `json:"moxfield_username,omitempty"`
}

// UserSearchResult es el DTO de GET /users/search — deliberadamente sin email: a
// diferencia de UserResponse (el perfil propio), esto se muestra sobre resultados de
// OTROS usuarios, y el email es el único dato que el buscador no necesariamente ya sabe.
type UserSearchResult struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
