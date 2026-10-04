package playgroups

// CreatePlaygroupRequest is the payload for creating a playgroup.
type CreatePlaygroupRequest struct {
	Name string `json:"name"`
}

// UpdatePlaygroupRequest is the payload for renaming a playgroup.
type UpdatePlaygroupRequest struct {
	Name string `json:"name"`
}

// PlaygroupResponse is the DTO of a playgroup sent to the client.
type PlaygroupResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// CreatedBy is the creator's user ID, the only one allowed to delete the group.
	// Null for a group whose creator's account no longer exists.
	CreatedBy *string `json:"created_by"`
	// InviteCode is the group's active invite code, null without one. Every
	// endpoint returning this DTO is membership-scoped, so only members see it.
	InviteCode *string                   `json:"invite_code"`
	Members    []PlaygroupMemberResponse `json:"members,omitempty"`
}

// PlaygroupInviteResponse is the newly generated invite code of a playgroup.
type PlaygroupInviteResponse struct {
	InviteCode string `json:"invite_code"`
}

// PlaygroupInvitePreviewResponse is what an invite code reveals before joining:
// the group's name and size, never its roster.
type PlaygroupInvitePreviewResponse struct {
	PlaygroupID string `json:"playgroup_id"`
	Name        string `json:"name"`
	MemberCount int64  `json:"member_count"`
	IsMember    bool   `json:"is_member"`
}

// PlaygroupListResponse is a page of the authenticated user's playgroups (see
// ListPlaygroupsPage). NextCursor is the opaque cursor for the next page,
// null on the last page — same shape as games.GameListResponse/decks.DeckListResponse.
type PlaygroupListResponse struct {
	Items      []PlaygroupResponse `json:"items"`
	NextCursor *string             `json:"next_cursor"`
}

// AddMemberRequest is the payload for adding a member to a playgroup.
type AddMemberRequest struct {
	UserID string `json:"user_id"`
}

// PlaygroupMemberResponse is the DTO of the relationship between a group and a member user.
type PlaygroupMemberResponse struct {
	PlaygroupID string `json:"playgroup_id"`
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
}

// DeckResponse is the DTO of another user's deck sent to the client (same shape as
// decks.DeckResponse, see GET /playgroups/{id}/members/{userId}/decks). It lives here
// instead of importing internal/decks because the authorization for this list is a
// playgroups decision (shared membership), not a decks one.
type DeckResponse struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
}
