package inbound

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// signatureTolerance is how far a webhook's timestamp may be from the server's
// clock, either way, before it is refused as a possible replay. Five minutes is
// what the Svix libraries use.
const signatureTolerance = 5 * time.Minute

var (
	// ErrInvalidSignature means the webhook carries no signature made with the
	// configured secret: it was not sent by Resend, or the body was altered.
	ErrInvalidSignature = errors.New("invalid webhook signature")
	// ErrStaleTimestamp means the signed timestamp is outside signatureTolerance.
	ErrStaleTimestamp = errors.New("webhook timestamp outside the allowed window")
)

// VerifySignature checks a Resend webhook the way the Svix libraries do (Resend
// delivers webhooks through Svix): an HMAC-SHA256, keyed with the base64 part of
// the "whsec_..." secret, over "<svix-id>.<svix-timestamp>.<raw body>". The
// svix-signature header holds one or more space-separated "v1,<base64>" values
// (several while a secret is being rotated); any one matching is enough.
// Implemented here rather than pulling in the Svix SDK for these few lines.
func VerifySignature(secret, msgID, timestamp, signatureHeader string, body []byte, now time.Time) error {
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil || len(key) == 0 || msgID == "" || signatureHeader == "" {
		return ErrInvalidSignature
	}
	if err := checkTimestamp(timestamp, now); err != nil {
		return err
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msgID + "." + timestamp + "."))
	mac.Write(body)
	if !anySignatureMatches(signatureHeader, mac.Sum(nil)) {
		return ErrInvalidSignature
	}
	return nil
}

func checkTimestamp(timestamp string, now time.Time) error {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrInvalidSignature
	}
	if diff := now.Sub(time.Unix(ts, 0)); diff > signatureTolerance || diff < -signatureTolerance {
		return ErrStaleTimestamp
	}
	return nil
}

// anySignatureMatches reports whether one of the header's "v1,<base64>" values
// is expected. Values with another version prefix are skipped, as Svix does.
func anySignatureMatches(signatureHeader string, expected []byte) bool {
	for _, candidate := range strings.Fields(signatureHeader) {
		version, sig, ok := strings.Cut(candidate, ",")
		if !ok || version != "v1" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(sig)
		if err == nil && hmac.Equal(decoded, expected) {
			return true
		}
	}
	return false
}
