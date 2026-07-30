package users

import "time"

// RegisterRequest representa la información necesaria para registrar un usuario.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateProfileRequest es el payload para actualizar el perfil propio: username de la
// cuenta y/o username de Moxfield. Ambos son punteros para distinguir "no lo mandó" (nil,
// no tocar el campo) de "lo mandó vacío" (string vacío en MoxfieldUsername = desvincular;
// en Username, vacío es inválido, ver UpdateUsername). Sin esto, un PATCH que solo manda
// uno de los dos campos pisaría el otro con su zero-value.
type UpdateProfileRequest struct {
	Username         *string `json:"username,omitempty"`
	MoxfieldUsername *string `json:"moxfield_username,omitempty"`
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
	// HasPassword indica si la cuenta tiene password propio (false = cuenta creada vía
	// Google Sign-In, sin password_hash) — los clientes lo usan para decidir si mostrar
	// el flujo de "cambiar contraseña" (ver ChangePassword, que rechaza estas cuentas).
	HasPassword bool `json:"has_password"`
}

// UserSearchResult es el DTO de GET /users/search — deliberadamente sin email: a
// diferencia de UserResponse (el perfil propio), esto se muestra sobre resultados de
// OTROS usuarios, y el email es el único dato que el buscador no necesariamente ya sabe.
type UserSearchResult struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
