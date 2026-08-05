package users

import "time"

// RegisterRequest represents the information needed to register a user.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateProfileRequest is the payload for updating one's own profile: account username
// and/or Moxfield username. Both are pointers to distinguish "didn't send it" (nil,
// don't touch the field) from "sent it empty" (empty string in MoxfieldUsername = unlink;
// in Username, empty is invalid, see UpdateUsername). Without this, a PATCH that only
// sends one of the two fields would overwrite the other with its zero-value.
type UpdateProfileRequest struct {
	Username         *string `json:"username,omitempty"`
	MoxfieldUsername *string `json:"moxfield_username,omitempty"`
}

// ChangePasswordRequest is the payload of POST /users/:id/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// VerifyEmailRequest is the payload of POST /auth/verify-email.
type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// ResendVerificationRequest is the payload of POST /auth/resend-verification.
type ResendVerificationRequest struct {
	Email string `json:"email"`
}

// UserResponse is the DTO sent to the client, without sensitive data (like the password hash).
type UserResponse struct {
	ID               string    `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	CreatedAt        time.Time `json:"created_at"`
	MoxfieldUsername *string   `json:"moxfield_username,omitempty"`
	// HasPassword indicates whether the account has its own password (false = account
	// created via Google Sign-In, without password_hash) — clients use it to decide
	// whether to show the "change password" flow (see ChangePassword, which rejects these accounts).
	HasPassword bool `json:"has_password"`
}

// UsernameAvailabilityResponse is the DTO of GET /users/username-available.
type UsernameAvailabilityResponse struct {
	Available bool `json:"available"`
}

// UserSearchResult is the DTO of GET /users/search — deliberately without email: unlike
// UserResponse (one's own profile), this is shown over results for OTHER users, and the
// email is the only piece of data the searcher doesn't necessarily already know.
type UserSearchResult struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}
