package users

import "time"

// RegisterRequest representa la información necesaria para registrar un usuario.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UserResponse es el DTO que se envía al cliente, sin datos sensibles (como el hash de contraseña).
type UserResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}
