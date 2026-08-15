package friends

import "time"

// SendFriendRequestRequest is the payload for sending a friend request. AddresseeID
// is resolved client-side beforehand, either via GET /users/search (typing a
// username) or by decoding the target's own profile QR (which encodes their id) --
// both entry points end up calling this same endpoint with the same shape.
type SendFriendRequestRequest struct {
	AddresseeID string `json:"addressee_id"`
}

// FriendRequestResponse is the DTO of a request just sent, returned by
// POST /friends/requests. Status is "accepted" instead of "pending" when it
// auto-accepted a pre-existing reverse request (see Service.SendFriendRequest).
type FriendRequestResponse struct {
	ID                string    `json:"id"`
	AddresseeID       string    `json:"addressee_id"`
	AddresseeUsername string    `json:"addressee_username"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

// IncomingFriendRequestResponse is the DTO of a pending request addressed to
// the authenticated user (GET /friends/requests?direction=incoming).
type IncomingFriendRequestResponse struct {
	ID                string    `json:"id"`
	RequesterID       string    `json:"requester_id"`
	RequesterUsername string    `json:"requester_username"`
	CreatedAt         time.Time `json:"created_at"`
}

// OutgoingFriendRequestResponse is the DTO of a pending request sent by the
// authenticated user (GET /friends/requests?direction=outgoing).
type OutgoingFriendRequestResponse struct {
	ID                string    `json:"id"`
	AddresseeID       string    `json:"addressee_id"`
	AddresseeUsername string    `json:"addressee_username"`
	CreatedAt         time.Time `json:"created_at"`
}

// FriendResponse is the DTO of an accepted friendship, resolved to the OTHER
// user regardless of who originally sent the request (GET /friends, and the
// result of accepting a request).
type FriendResponse struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	FriendsSince time.Time `json:"friends_since"`
}
