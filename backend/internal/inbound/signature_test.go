package inbound_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/usuario/commander-companion-backend/internal/inbound"
)

const (
	testKey   = "a-32-byte-test-signing-key-12345"
	testMsgID = "msg_1"
)

// testSecret is testKey in Resend's "whsec_<base64>" format.
func testSecret() string {
	return "whsec_" + base64.StdEncoding.EncodeToString([]byte(testKey))
}

// sign returns the svix-signature value Resend would send for testMsgID.
func sign(key string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(testMsgID + "." + strconv.FormatInt(ts, 10) + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	now := time.Unix(1_790_000_000, 0)
	body := []byte(`{"type":"email.received","data":{"email_id":"abc"}}`)
	ts := now.Unix()
	tsStr := strconv.FormatInt(ts, 10)
	valid := sign(testKey, ts, body)
	oldTS, futureTS := ts-600, ts+600

	tests := []struct {
		name      string
		secret    string
		msgID     string
		timestamp string
		signature string
		body      []byte
		want      error
	}{
		{"valid", testSecret(), testMsgID, tsStr, valid, body, nil},
		{"valid among several (secret rotation)", testSecret(), testMsgID, tsStr, "v1,bm90LWl0 " + valid, body, nil},
		{
			name: "wrong secret", secret: "whsec_" + base64.StdEncoding.EncodeToString([]byte("another-key")),
			msgID: testMsgID, timestamp: tsStr, signature: valid, body: body, want: inbound.ErrInvalidSignature,
		},
		{
			name: "tampered body", secret: testSecret(), msgID: testMsgID, timestamp: tsStr, signature: valid,
			body: []byte(`{"type":"email.received","data":{"email_id":"evil"}}`), want: inbound.ErrInvalidSignature,
		},
		{"other message id", testSecret(), "msg_2", tsStr, valid, body, inbound.ErrInvalidSignature},
		{"unknown version only", testSecret(), testMsgID, tsStr, "v2" + valid[2:], body, inbound.ErrInvalidSignature},
		{"missing signature", testSecret(), testMsgID, tsStr, "", body, inbound.ErrInvalidSignature},
		{"non-numeric timestamp", testSecret(), testMsgID, "yesterday", valid, body, inbound.ErrInvalidSignature},
		{"secret not base64", "whsec_!!!", testMsgID, tsStr, valid, body, inbound.ErrInvalidSignature},
		{
			name: "timestamp too old", secret: testSecret(), msgID: testMsgID, timestamp: strconv.FormatInt(oldTS, 10),
			signature: sign(testKey, oldTS, body), body: body, want: inbound.ErrStaleTimestamp,
		},
		{
			name: "timestamp in the future", secret: testSecret(), msgID: testMsgID,
			timestamp: strconv.FormatInt(futureTS, 10), signature: sign(testKey, futureTS, body), body: body,
			want: inbound.ErrStaleTimestamp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := inbound.VerifySignature(tt.secret, tt.msgID, tt.timestamp, tt.signature, tt.body, now)
			if !errors.Is(err, tt.want) {
				t.Fatalf("VerifySignature() = %v, want %v", err, tt.want)
			}
		})
	}
}
