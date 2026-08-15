package friends

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	statusPending   = "pending"
	statusAccepted  = "accepted"
	statusRejected  = "rejected"
	statusCancelled = "cancelled"
)

var (
	// ErrUserNotFound indicates that the target user of a friend request doesn't exist.
	ErrUserNotFound = common.NotFound("user not found")
	// ErrInvalidUserID indicates that the received addressee_id/user id isn't a valid UUID.
	ErrInvalidUserID = common.InvalidInput("invalid user id")
	// ErrCannotFriendSelf indicates an attempt to send a friend request to oneself.
	ErrCannotFriendSelf = common.InvalidInput("cannot send a friend request to yourself")
	// ErrAlreadyFriends indicates that both users are already friends.
	ErrAlreadyFriends = common.Conflict("users are already friends")
	// ErrRequestAlreadyPending indicates a duplicate request in the same direction.
	ErrRequestAlreadyPending = common.Conflict("a friend request is already pending")
	// ErrRequestNotFound indicates that the request doesn't exist or doesn't belong to the caller
	// (as requester or addressee, depending on the operation) -- deliberately not distinguished,
	// same "don't reveal" criteria as internal/playgroups' ErrPlaygroupNotFound.
	ErrRequestNotFound = common.NotFound("friend request not found")
	// ErrRequestNotPending indicates an accept/reject/cancel on a request already responded to.
	ErrRequestNotPending = common.Conflict("friend request is not pending")
	// ErrFriendshipNotFound indicates an unfriend attempt between users that aren't friends.
	ErrFriendshipNotFound = common.NotFound("friendship not found")
)

// Service defines the business logic of the friends module.
type Service interface {
	// SendFriendRequest creates a pending request from requesterID to req.AddresseeID.
	// If the addressee already has a pending request TO the requester, it auto-accepts
	// that existing row instead of creating a second one (see the package doc on
	// friend_requests_pending_direction_idx for why this isn't a DB constraint).
	SendFriendRequest(
		ctx context.Context, requesterID string, req SendFriendRequestRequest,
	) (*FriendRequestResponse, error)
	ListIncomingRequests(ctx context.Context, userID string) ([]IncomingFriendRequestResponse, error)
	ListOutgoingRequests(ctx context.Context, userID string) ([]OutgoingFriendRequestResponse, error)
	// AcceptFriendRequest accepts requestID. Only its addressee may accept it.
	AcceptFriendRequest(ctx context.Context, userID, requestID string) (*FriendResponse, error)
	// RejectFriendRequest rejects requestID. Only its addressee may reject it.
	RejectFriendRequest(ctx context.Context, userID, requestID string) error
	// CancelFriendRequest cancels an outgoing pending requestID. Only its
	// original requester may cancel it.
	CancelFriendRequest(ctx context.Context, userID, requestID string) error
	ListFriends(ctx context.Context, userID string) ([]FriendResponse, error)
	// RemoveFriend deletes the accepted friendship between userID and friendUserID, if any.
	RemoveFriend(ctx context.Context, userID, friendUserID string) error
}

type service struct {
	repo *Queries
}

// NewService creates a new friends service.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// SendFriendRequest see Service.SendFriendRequest.
func (s *service) SendFriendRequest(
	ctx context.Context, requesterID string, req SendFriendRequestRequest,
) (*FriendRequestResponse, error) {
	requesterUID, err := common.ParseUUID(requesterID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}
	addresseeUID, err := common.ParseUUID(req.AddresseeID)
	if err != nil {
		return nil, ErrInvalidUserID
	}
	if requesterUID == addresseeUID {
		return nil, ErrCannotFriendSelf
	}

	addressee, err := s.repo.GetUserByID(ctx, addresseeUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("looking up addressee: %w", err)
	}

	fr, err := s.resolveOrCreateRequest(ctx, requesterUID, addresseeUID)
	if err != nil {
		return nil, err
	}

	return &FriendRequestResponse{
		ID:                fr.ID.String(),
		AddresseeID:       addressee.ID.String(),
		AddresseeUsername: addressee.Username,
		Status:            fr.Status,
		CreatedAt:         fr.CreatedAt.Time,
	}, nil
}

// resolveOrCreateRequest implements SendFriendRequest's actual rules once both
// users are known to exist and aren't the same person: reject if already
// friends, auto-accept a pre-existing reverse-direction pending request,
// reject a duplicate same-direction pending request, or create a new one.
func (s *service) resolveOrCreateRequest(
	ctx context.Context, requesterUID, addresseeUID pgtype.UUID,
) (*FriendRequest, error) {
	_, err := s.repo.GetAcceptedFriendship(ctx, GetAcceptedFriendshipParams{UserA: requesterUID, UserB: addresseeUID})
	if err == nil {
		return nil, ErrAlreadyFriends
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("checking existing friendship: %w", err)
	}

	// A pending request already exists in the OPPOSITE direction (the addressee
	// asked the requester first): accept it instead of creating a second row.
	reverse, err := s.repo.GetPendingRequestBetween(
		ctx, GetPendingRequestBetweenParams{RequesterID: addresseeUID, AddresseeID: requesterUID},
	)
	if err == nil {
		accepted, respondErr := s.repo.RespondFriendRequest(
			ctx, RespondFriendRequestParams{ID: reverse.ID, Status: statusAccepted},
		)
		if respondErr != nil {
			return nil, fmt.Errorf("auto-accepting reverse request: %w", respondErr)
		}
		return &accepted, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("checking reverse request: %w", err)
	}

	_, err = s.repo.GetPendingRequestBetween(
		ctx, GetPendingRequestBetweenParams{RequesterID: requesterUID, AddresseeID: addresseeUID},
	)
	if err == nil {
		return nil, ErrRequestAlreadyPending
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("checking existing request: %w", err)
	}

	created, err := s.repo.CreateFriendRequest(
		ctx, CreateFriendRequestParams{RequesterID: requesterUID, AddresseeID: addresseeUID},
	)
	if err != nil {
		return nil, fmt.Errorf("creating friend request: %w", err)
	}
	return &created, nil
}

// ListIncomingRequests returns userID's pending incoming requests.
func (s *service) ListIncomingRequests(ctx context.Context, userID string) ([]IncomingFriendRequestResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	rows, err := s.repo.ListIncomingFriendRequests(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing incoming friend requests: %w", err)
	}

	result := make([]IncomingFriendRequestResponse, 0, len(rows))
	for i := range rows {
		result = append(result, IncomingFriendRequestResponse{
			ID:                rows[i].ID.String(),
			RequesterID:       rows[i].RequesterID.String(),
			RequesterUsername: rows[i].RequesterUsername,
			CreatedAt:         rows[i].CreatedAt.Time,
		})
	}
	return result, nil
}

// ListOutgoingRequests returns userID's pending outgoing requests.
func (s *service) ListOutgoingRequests(ctx context.Context, userID string) ([]OutgoingFriendRequestResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	rows, err := s.repo.ListOutgoingFriendRequests(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing outgoing friend requests: %w", err)
	}

	result := make([]OutgoingFriendRequestResponse, 0, len(rows))
	for i := range rows {
		result = append(result, OutgoingFriendRequestResponse{
			ID:                rows[i].ID.String(),
			AddresseeID:       rows[i].AddresseeID.String(),
			AddresseeUsername: rows[i].AddresseeUsername,
			CreatedAt:         rows[i].CreatedAt.Time,
		})
	}
	return result, nil
}

// AcceptFriendRequest see Service.AcceptFriendRequest.
func (s *service) AcceptFriendRequest(ctx context.Context, userID, requestID string) (*FriendResponse, error) {
	fr, err := s.getPendingRequestAs(ctx, userID, requestID, addressee)
	if err != nil {
		return nil, err
	}

	accepted, err := s.repo.RespondFriendRequest(ctx, RespondFriendRequestParams{ID: fr.ID, Status: statusAccepted})
	if err != nil {
		return nil, fmt.Errorf("accepting friend request: %w", err)
	}

	requester, err := s.repo.GetUserByID(ctx, accepted.RequesterID)
	if err != nil {
		return nil, fmt.Errorf("looking up requester: %w", err)
	}

	return &FriendResponse{
		ID:           requester.ID.String(),
		Username:     requester.Username,
		FriendsSince: accepted.RespondedAt.Time,
	}, nil
}

// RejectFriendRequest see Service.RejectFriendRequest.
func (s *service) RejectFriendRequest(ctx context.Context, userID, requestID string) error {
	fr, err := s.getPendingRequestAs(ctx, userID, requestID, addressee)
	if err != nil {
		return err
	}

	_, err = s.repo.RespondFriendRequest(ctx, RespondFriendRequestParams{ID: fr.ID, Status: statusRejected})
	if err != nil {
		return fmt.Errorf("rejecting friend request: %w", err)
	}
	return nil
}

// CancelFriendRequest see Service.CancelFriendRequest.
func (s *service) CancelFriendRequest(ctx context.Context, userID, requestID string) error {
	fr, err := s.getPendingRequestAs(ctx, userID, requestID, requester)
	if err != nil {
		return err
	}

	_, err = s.repo.RespondFriendRequest(ctx, RespondFriendRequestParams{ID: fr.ID, Status: statusCancelled})
	if err != nil {
		return fmt.Errorf("cancelling friend request: %w", err)
	}
	return nil
}

// ListFriends returns userID's accepted friendships.
func (s *service) ListFriends(ctx context.Context, userID string) ([]FriendResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	rows, err := s.repo.ListFriends(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing friends: %w", err)
	}

	result := make([]FriendResponse, 0, len(rows))
	for i := range rows {
		result = append(result, FriendResponse{
			ID:           rows[i].FriendID.String(),
			Username:     rows[i].FriendUsername,
			FriendsSince: rows[i].FriendsSince.Time,
		})
	}
	return result, nil
}

// RemoveFriend see Service.RemoveFriend.
func (s *service) RemoveFriend(ctx context.Context, userID, friendUserID string) error {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return common.ErrInvalidUser
	}
	friendUID, err := common.ParseUUID(friendUserID)
	if err != nil {
		return ErrInvalidUserID
	}

	if _, err := s.repo.GetAcceptedFriendship(
		ctx, GetAcceptedFriendshipParams{UserA: uid, UserB: friendUID},
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrFriendshipNotFound
		}
		return fmt.Errorf("checking friendship: %w", err)
	}

	if err := s.repo.DeleteFriendship(ctx, DeleteFriendshipParams{UserA: uid, UserB: friendUID}); err != nil {
		return fmt.Errorf("removing friendship: %w", err)
	}
	return nil
}

// side names which party of a friend_requests row a caller is required to be,
// for getPendingRequestAs's authorization check.
type side int

const (
	requester side = iota
	addressee
)

// getPendingRequestAs resolves a pending request by ID, requiring the given
// userID to be its requester or addressee (per as). A malformed ID, a
// nonexistent request, one belonging to someone else, or one no longer
// pending all return the same ErrRequestNotFound/ErrRequestNotPending,
// without distinguishing which case it was -- same "don't reveal" criteria
// as internal/playgroups.getMemberPlaygroup.
func (s *service) getPendingRequestAs(ctx context.Context, userID, requestID string, as side) (*FriendRequest, error) {
	rid, err := common.ParseUUID(requestID)
	if err != nil {
		return nil, ErrRequestNotFound
	}
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, ErrRequestNotFound
	}

	fr, err := s.repo.GetFriendRequestByID(ctx, rid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRequestNotFound
		}
		return nil, fmt.Errorf("looking up friend request: %w", err)
	}

	owner := fr.AddresseeID
	if as == requester {
		owner = fr.RequesterID
	}
	if owner != uid {
		return nil, ErrRequestNotFound
	}
	if fr.Status != statusPending {
		return nil, ErrRequestNotPending
	}

	return &fr, nil
}
