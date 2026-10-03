package friends_test

import (
	"context"
	"testing"

	"github.com/usuario/commander-companion-backend/internal/friends"
	"github.com/usuario/commander-companion-backend/internal/testutil"
)

func requireIncomingCount(t *testing.T, svc friends.Service, userID string, want int64, when string) {
	t.Helper()
	got, err := svc.CountIncomingRequests(context.Background(), userID)
	requireNoErr(t, err, "CountIncomingRequests() "+when)
	if got.Incoming != want {
		t.Fatalf("CountIncomingRequests() %s = %d, want %d", when, got.Incoming, want)
	}
}

// The count follows the same lifecycle as the incoming list: it only counts pending
// requests addressed to the user, never the ones they sent, and drops as soon as a
// request is answered or withdrawn.
func TestCountIncomingRequests_FollowsPendingRequests(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)
	ctx := context.Background()

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "count-a@example.com")
	b := createTestUser(t, pool, "count-b@example.com")
	c := createTestUser(t, pool, "count-c@example.com")
	d := createTestUser(t, pool, "count-d@example.com")
	e := createTestUser(t, pool, "count-e@example.com")

	requireIncomingCount(t, svc, a.ID, 0, "with no requests")

	fromB, err := svc.SendFriendRequest(ctx, b.ID, friends.SendFriendRequestRequest{AddresseeID: a.ID})
	requireNoErr(t, err, "SendFriendRequest(b -> a)")
	fromC, err := svc.SendFriendRequest(ctx, c.ID, friends.SendFriendRequestRequest{AddresseeID: a.ID})
	requireNoErr(t, err, "SendFriendRequest(c -> a)")
	fromD, err := svc.SendFriendRequest(ctx, d.ID, friends.SendFriendRequestRequest{AddresseeID: a.ID})
	requireNoErr(t, err, "SendFriendRequest(d -> a)")
	// An outgoing request is not something a has to answer.
	_, err = svc.SendFriendRequest(ctx, a.ID, friends.SendFriendRequestRequest{AddresseeID: e.ID})
	requireNoErr(t, err, "SendFriendRequest(a -> e)")

	requireIncomingCount(t, svc, a.ID, 3, "with three incoming and one outgoing")
	requireIncomingCount(t, svc, b.ID, 0, "for the requester")

	_, err = svc.AcceptFriendRequest(ctx, a.ID, fromB.ID)
	requireNoErr(t, err, "AcceptFriendRequest(b)")
	requireNoErr(t, svc.RejectFriendRequest(ctx, a.ID, fromC.ID), "RejectFriendRequest(c)")
	requireNoErr(t, svc.CancelFriendRequest(ctx, d.ID, fromD.ID), "CancelFriendRequest(d)")

	requireIncomingCount(t, svc, a.ID, 0, "after accepting, rejecting and a cancellation")
}
