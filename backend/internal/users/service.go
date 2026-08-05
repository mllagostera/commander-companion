package users

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	maxUsernameAttempts = 3
	googleIDSuffixLen   = 6
	usernameConstraint  = "users_username_key"

	// emailVerificationTokenBytes is the length in bytes of the opaque email
	// verification token (before encoding), same as the refresh token
	// (see internal/auth/token.go).
	emailVerificationTokenBytes = 32
	// emailVerificationTokenTTL is how long a verification link sent by mail
	// takes to expire before a resend has to be requested.
	emailVerificationTokenTTL = 24 * time.Hour

	// minSearchQueryLength avoids 1-character searches that would return half
	// the user directory (and would make brute-force enumeration easier one
	// generic character at a time).
	minSearchQueryLength = 2
	// searchResultLimit caps the SearchUsers response — there's no pagination
	// here, it's an autocomplete, not a listing.
	searchResultLimit = 10
)

var (
	// ErrInvalidCredentials indicates that the email doesn't exist or the password doesn't match.
	ErrInvalidCredentials = common.Unauthorized("invalid email or password")
	// ErrGoogleOnlyAccount indicates that the account has no password (it was registered with Google).
	ErrGoogleOnlyAccount = common.Unauthorized("account has no password set, sign in with google instead")
	// ErrUserNotFound indicates that no user exists with the given ID.
	ErrUserNotFound = common.NotFound("user not found")
	// ErrEmailNotVerified indicates that Google doesn't confirm the email associated with the account.
	ErrEmailNotVerified = common.InvalidInput("google email not verified")
	// ErrEmailNotConfirmed indicates that the account is valid (correct password) but
	// hasn't confirmed its own email yet — distinct from ErrEmailNotVerified (which is
	// about what Google asserts), and distinct from ErrInvalidCredentials (here we
	// already know who it is). The client can offer to resend the verification email.
	ErrEmailNotConfirmed = common.Forbidden("email not confirmed, check your inbox")
	// ErrInvalidVerificationToken indicates that the email verification token
	// doesn't exist, was already used, or has expired.
	ErrInvalidVerificationToken = common.InvalidInput("invalid or expired verification token")
	// ErrUserAlreadyExists indicates that the username or email is already taken.
	ErrUserAlreadyExists = common.Conflict("User already exists")
	// ErrUsernameExhausted indicates that a unique username couldn't be generated for a Google account.
	// It's not a translatable domain error: it comes out as a 500, because it implies a server problem.
	ErrUsernameExhausted = errors.New("could not allocate a unique username for google user")
	// ErrInvalidCurrentPassword indicates that the current password sent to ChangePassword doesn't match.
	ErrInvalidCurrentPassword = common.Unauthorized("current password is incorrect")
	// ErrPasswordTooShort indicates that the new password doesn't meet the minimum length.
	ErrPasswordTooShort = common.InvalidInput("password must be at least 8 characters long")
	// ErrSearchQueryTooShort indicates that the search query is too short.
	ErrSearchQueryTooShort = common.InvalidInput("search query must be at least 2 characters long")
	// ErrUsernameEmpty indicates that an empty/whitespace-only username was sent to UpdateUsername.
	ErrUsernameEmpty = common.InvalidInput("username cannot be empty")
	// ErrUsernameTaken indicates that the chosen username is already in use by another account.
	ErrUsernameTaken = common.Conflict("username already taken")
)

// minPasswordLength is the minimum length of the new password in ChangePassword, matching the
// minimum already enforced by the client-side registration form (see web/app/pages/register.vue).
const minPasswordLength = 8

// Mailer is what users needs to send the account verification email
// (allows mocking it in tests; see decks.MoxfieldClient for the same pattern).
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, username, verifyURL string) error
}

// Service is the interface for user logic.
type Service interface {
	RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error)
	GetUser(ctx context.Context, id string) (*UserResponse, error)
	VerifyCredentials(ctx context.Context, email, password string) (*UserResponse, error)
	FindOrCreateGoogleUser(ctx context.Context, googleID, email string, emailVerified bool) (*UserResponse, error)
	UpdateMoxfieldUsername(ctx context.Context, id, moxfieldUsername string) (*UserResponse, error)
	// UpdateUsername changes the login/profile username (distinct from the Moxfield one). Returns
	// ErrUsernameTaken (409) if it's already in use by another account.
	UpdateUsername(ctx context.Context, id, username string) (*UserResponse, error)
	// ChangePassword validates the current password and, if it matches, replaces it with the new one.
	ChangePassword(ctx context.Context, id, currentPassword, newPassword string) error
	// VerifyEmail confirms the account associated with the verification token sent by mail.
	VerifyEmail(ctx context.Context, token string) error
	// ResendVerification sends a new verification email if applicable. It never
	// reveals whether the email exists, is already verified, or is a Google account:
	// it always "succeeds" from the caller's perspective (same anti-enumeration
	// criteria as VerifyCredentials).
	ResendVerification(ctx context.Context, email string) error
	// SearchUsers searches by username (contains, case-insensitive) or email (exact, see
	// query.sql for why it's not partial). Excludes the requesterID itself and never
	// exposes the email in the result (see UserSearchResult).
	SearchUsers(ctx context.Context, requesterID, query string) ([]UserSearchResult, error)
	// IsUsernameAvailable reports whether username is free to register (or to move to,
	// via UpdateUsername) — an exact, case-sensitive match against the same uniqueness
	// constraint the database enforces. Public/unauthenticated, unlike SearchUsers: it
	// doesn't reveal anything the register form's own submission wouldn't already (see
	// ErrUserAlreadyExists), just earlier and for one specific name.
	IsUsernameAvailable(ctx context.Context, username string) (bool, error)
}

type service struct {
	repo                     *Queries
	mailer                   Mailer
	webAppURL                string
	requireEmailVerification bool
}

// NewService creates a new users service. requireEmailVerification controls whether
// signup via email/password requires confirming the email before being able to log in
// (see ADR-0012); when false (alpha phase) new accounts are left verified upfront and
// RegisterUser neither generates the token nor sends mail — there's no point spending
// that send if nobody's going to require the click.
func NewService(db *pgxpool.Pool, mailer Mailer, webAppURL string, requireEmailVerification bool) Service {
	return &service{
		repo:                     New(db),
		mailer:                   mailer,
		webAppURL:                webAppURL,
		requireEmailVerification: requireEmailVerification,
	}
}

// RegisterUser creates a new user with email and password. If requireEmailVerification
// is active, the account is left unconfirmed and triggers the mail (see sendVerificationEmail);
// if not, it's left verified upfront and nothing is sent.
func (s *service) RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, CreateUserParams{
		Username:      req.Username,
		Email:         req.Email,
		PasswordHash:  pgtype.Text{String: string(hash), Valid: true},
		EmailVerified: !s.requireEmailVerification,
	})
	if err != nil {
		//nolint:godox // Deferred to the refinement phase: distinguish username vs. duplicate email.
		// TODO: inspect pgErr.ConstraintName to return a more precise message.
		return nil, ErrUserAlreadyExists
	}

	if !s.requireEmailVerification {
		return toUserResponse(&user), nil
	}

	// The account is already created: a mail that fails to send doesn't revert the
	// registration, the user can request a resend from /login.
	if err := s.sendVerificationEmail(ctx, &user); err != nil {
		log.Printf("no se pudo mandar el mail de verificación a %s: %v", user.Email, err)
	}

	return toUserResponse(&user), nil
}

// sendVerificationEmail generates a new verification token, persists it hashed, and
// triggers the mail with the link built over webAppURL.
func (s *service) sendVerificationEmail(ctx context.Context, user *User) error {
	plain, err := common.NewOpaqueToken(emailVerificationTokenBytes)
	if err != nil {
		return err
	}

	if _, err := s.repo.CreateEmailVerificationToken(ctx, CreateEmailVerificationTokenParams{
		UserID:    user.ID,
		TokenHash: common.HashToken(plain),
		ExpiresAt: pgtype.Timestamp{Time: time.Now().Add(emailVerificationTokenTTL), Valid: true},
	}); err != nil {
		return fmt.Errorf("storing verification token: %w", err)
	}

	verifyURL := s.webAppURL + "/verify-email?token=" + plain
	if err := s.mailer.SendVerificationEmail(ctx, user.Email, user.Username, verifyURL); err != nil {
		return fmt.Errorf("sending verification email: %w", err)
	}
	return nil
}

// GetUser returns a user by their ID.
func (s *service) GetUser(ctx context.Context, id string) (*UserResponse, error) {
	uid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.repo.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("looking up user by id: %w", err)
	}

	return toUserResponse(&user), nil
}

// UpdateMoxfieldUsername links (or changes) the profile's Moxfield username. It doesn't
// validate against the Moxfield API that the username actually exists — that's left for
// when it's used (see internal/moxfieldimport).
func (s *service) UpdateMoxfieldUsername(ctx context.Context, id, moxfieldUsername string) (*UserResponse, error) {
	uid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.repo.UpdateMoxfieldUsername(ctx, UpdateMoxfieldUsernameParams{
		ID:               uid,
		MoxfieldUsername: pgtype.Text{String: moxfieldUsername, Valid: moxfieldUsername != ""},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("updating moxfield username: %w", err)
	}

	return toUserResponse(&user), nil
}

// UpdateUsername changes the account's username (the one used to log in, distinct
// from the Moxfield one). Unlike UpdateMoxfieldUsername, here an empty string is
// invalid (the login username can never be left empty, it has NOT NULL in the DB).
func (s *service) UpdateUsername(ctx context.Context, id, username string) (*UserResponse, error) {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return nil, ErrUsernameEmpty
	}

	uid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.repo.UpdateUsername(ctx, UpdateUsernameParams{ID: uid, Username: trimmed})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == usernameConstraint {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("updating username: %w", err)
	}

	return toUserResponse(&user), nil
}

// ChangePassword validates currentPassword against the stored hash and, if it
// matches, replaces it with the hash of newPassword. Google accounts (without
// password_hash) can't use this path: they must keep signing in via Google Sign-In.
func (s *service) ChangePassword(ctx context.Context, id, currentPassword, newPassword string) error {
	if len(newPassword) < minPasswordLength {
		return ErrPasswordTooShort
	}

	uid, err := common.ParseUUID(id)
	if err != nil {
		return ErrUserNotFound
	}

	user, err := s.repo.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("looking up user by id: %w", err)
	}

	if !user.PasswordHash.Valid {
		return ErrGoogleOnlyAccount
	}

	if compareErr := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash.String), []byte(currentPassword),
	); compareErr != nil {
		return ErrInvalidCurrentPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing new password: %w", err)
	}

	if _, err := s.repo.UpdatePasswordHash(ctx, UpdatePasswordHashParams{
		ID:           uid,
		PasswordHash: pgtype.Text{String: string(hash), Valid: true},
	}); err != nil {
		return fmt.Errorf("updating password hash: %w", err)
	}

	return nil
}

// VerifyCredentials validates the email/password and returns the user if they're correct.
func (s *service) VerifyCredentials(ctx context.Context, email, password string) (*UserResponse, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("looking up user by email: %w", err)
	}

	if !user.PasswordHash.Valid {
		return nil, ErrGoogleOnlyAccount
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if !user.EmailVerified {
		return nil, ErrEmailNotConfirmed
	}

	return toUserResponse(&user), nil
}

// FindOrCreateGoogleUser looks up a user by their Google ID. If it doesn't exist but
// the email is already registered, it links the Google account; if that doesn't
// exist either, it creates a new user without a password.
func (s *service) FindOrCreateGoogleUser(
	ctx context.Context, googleID, email string, emailVerified bool,
) (*UserResponse, error) {
	googleIDText := pgtype.Text{String: googleID, Valid: true}

	user, err := s.repo.GetUserByGoogleID(ctx, googleIDText)
	switch {
	case err == nil:
		return toUserResponse(&user), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("looking up user by google id: %w", err)
	}

	if !emailVerified {
		return nil, ErrEmailNotVerified
	}

	existing, err := s.repo.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		var linked User
		linked, err = s.repo.LinkGoogleID(ctx, LinkGoogleIDParams{ID: existing.ID, GoogleID: googleIDText})
		if err != nil {
			return nil, fmt.Errorf("linking google account: %w", err)
		}
		return toUserResponse(&linked), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("looking up user by email: %w", err)
	}

	return s.createGoogleUser(ctx, googleID, googleIDText, email)
}

func (s *service) createGoogleUser(
	ctx context.Context, googleID string, googleIDText pgtype.Text, email string,
) (*UserResponse, error) {
	base := usernameFromEmail(email)

	for attempt := range maxUsernameAttempts {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s-%s", base, googleID[:min(googleIDSuffixLen, len(googleID))])
		}

		created, err := s.repo.CreateUserWithGoogle(ctx, CreateUserWithGoogleParams{
			Username: candidate,
			Email:    email,
			GoogleID: googleIDText,
		})
		if err == nil {
			return toUserResponse(&created), nil
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != usernameConstraint {
			return nil, fmt.Errorf("creating google user: %w", err)
		}
	}

	return nil, ErrUsernameExhausted
}

func usernameFromEmail(email string) string {
	local, _, found := strings.Cut(email, "@")
	if !found || local == "" {
		return "user"
	}
	return local
}

// VerifyEmail confirms the account associated with the verification token sent by mail.
// A nonexistent, already-used, or expired token returns the same error, so as not to
// leak which of the three cases it is.
func (s *service) VerifyEmail(ctx context.Context, token string) error {
	record, err := s.repo.GetEmailVerificationTokenByHash(ctx, common.HashToken(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidVerificationToken
		}
		return fmt.Errorf("looking up verification token: %w", err)
	}
	if record.UsedAt.Valid || !record.ExpiresAt.Valid || record.ExpiresAt.Time.Before(time.Now()) {
		return ErrInvalidVerificationToken
	}

	if err := s.repo.MarkEmailVerificationTokenUsed(ctx, record.ID); err != nil {
		return fmt.Errorf("marking verification token used: %w", err)
	}

	if _, err := s.repo.SetUserEmailVerified(ctx, record.UserID); err != nil {
		return fmt.Errorf("marking user verified: %w", err)
	}

	return nil
}

// ResendVerification sends a new verification token if applicable. It never reveals
// anything to the caller (always "success"): if the email doesn't exist, is already
// verified, or is an account without a password (Google-only, never logs in by
// password and therefore never gets blocked by this), it does nothing.
func (s *service) ResendVerification(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("looking up user by email: %w", err)
	}

	if user.EmailVerified || !user.PasswordHash.Valid {
		return nil
	}

	if err := s.sendVerificationEmail(ctx, &user); err != nil {
		log.Printf("no se pudo reenviar el mail de verificación a %s: %v", user.Email, err)
	}
	return nil
}

// SearchUsers searches by username (ILIKE, contains) and/or email (exact). The two
// paths are combined and deduplicated; the requesterID itself never appears in the
// result (there's no point inviting yourself to a playgroup) nor does its email (see
// UserSearchResult).
func (s *service) SearchUsers(ctx context.Context, requesterID, query string) ([]UserSearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if len([]rune(trimmed)) < minSearchQueryLength {
		return nil, ErrSearchQueryTooShort
	}

	seen := map[string]bool{requesterID: true}
	results := []UserSearchResult{} // never nil: without this, an empty result serializes to JSON `null` instead of `[]`

	byEmail, err := s.repo.GetUserByEmail(ctx, trimmed)
	switch {
	case err == nil:
		id := byEmail.ID.String()
		if !seen[id] {
			seen[id] = true
			results = append(results, UserSearchResult{ID: id, Username: byEmail.Username})
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("searching user by email: %w", err)
	}

	byUsername, err := s.repo.SearchUsersByUsername(
		ctx,
		SearchUsersByUsernameParams{
			Pattern:     pgtype.Text{String: trimmed, Valid: true},
			ResultLimit: searchResultLimit,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("searching users by username: %w", err)
	}
	for i := range byUsername {
		id := byUsername[i].ID.String()
		if seen[id] {
			continue
		}
		seen[id] = true
		results = append(results, UserSearchResult{ID: id, Username: byUsername[i].Username})
	}

	return results, nil
}

// IsUsernameAvailable reports whether username is free, trimmed the same way
// UpdateUsername trims it before writing (so a check against "  foo  " agrees
// with what registering/renaming to "  foo  " would actually collide with).
func (s *service) IsUsernameAvailable(ctx context.Context, username string) (bool, error) {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return false, ErrUsernameEmpty
	}

	exists, err := s.repo.UsernameExists(ctx, trimmed)
	if err != nil {
		return false, fmt.Errorf("checking username existence: %w", err)
	}
	return !exists, nil
}

func toUserResponse(user *User) *UserResponse {
	var createdAt time.Time
	if user.CreatedAt.Valid {
		createdAt = user.CreatedAt.Time
	}

	res := &UserResponse{
		ID:          user.ID.String(),
		Username:    user.Username,
		Email:       user.Email,
		CreatedAt:   createdAt,
		HasPassword: user.PasswordHash.Valid,
	}
	if user.MoxfieldUsername.Valid {
		res.MoxfieldUsername = &user.MoxfieldUsername.String
	}
	return res
}
