package testutil

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/users"
)

// testWebAppURL es la base usada para armar links de verificación en tests; ningún
// test de los que usan este helper resuelve el link de verdad.
const testWebAppURL = "http://localhost:3000"

// noopMailer no manda nada: alcanza para instanciar users.Service en los módulos que
// solo lo necesitan como dependencia (no ejercitan el flujo de verificación de email en
// sí). internal/users/service_test.go usa un fake propio que sí registra el token
// mandado, para poder probar VerifyEmail.
type noopMailer struct{}

// SendVerificationEmail no hace nada (ver noopMailer).
func (noopMailer) SendVerificationEmail(context.Context, string, string, string) error {
	return nil
}

// NewUsersService instancia users.Service con un mailer no-op y verificación de email
// requerida (a diferencia del default de producción — ver ADR-0012 —, acá se deja
// requireEmailVerification en true a propósito: así el registro sigue dejando la cuenta
// sin confirmar y VerifyUserEmail, más abajo, sigue siendo necesario y significativo
// para los tests que necesitan loguearse).
func NewUsersService(pool *pgxpool.Pool) users.Service {
	return users.NewService(pool, noopMailer{}, testWebAppURL, true)
}

// VerifyUserEmail marca un usuario como con email confirmado sin pasar por el flujo de
// token/mail, para tests de otros módulos que necesitan una cuenta que pueda loguearse
// (VerifyCredentials ahora exige email_verified, ver internal/users/service.go) pero no
// están probando el flujo de verificación en sí.
func VerifyUserEmail(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	const query = "UPDATE users SET email_verified = true WHERE id = $1"
	if _, err := pool.Exec(context.Background(), query, userID); err != nil {
		t.Fatalf("marcando email verificado para %s: %v", userID, err)
	}
}
