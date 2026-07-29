package playgroups

// CreatePlaygroupRequest es el payload para crear un grupo de juego.
type CreatePlaygroupRequest struct {
	Name string `json:"name"`
}

// UpdatePlaygroupRequest es el payload para renombrar un grupo de juego.
type UpdatePlaygroupRequest struct {
	Name string `json:"name"`
}

// PlaygroupResponse es el DTO de un grupo de juego enviado al cliente.
type PlaygroupResponse struct {
	ID      string                    `json:"id"`
	Name    string                    `json:"name"`
	Members []PlaygroupMemberResponse `json:"members,omitempty"`
}

// AddMemberRequest es el payload para añadir un miembro a un grupo de juego.
type AddMemberRequest struct {
	UserID string `json:"user_id"`
}

// PlaygroupMemberResponse es el DTO de la relación entre un grupo y un usuario miembro.
type PlaygroupMemberResponse struct {
	PlaygroupID string `json:"playgroup_id"`
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
}

// DeckResponse es el DTO de un deck ajeno enviado al cliente (mismo shape que
// decks.DeckResponse, ver GET /playgroups/{id}/members/{userId}/decks). Vive acá
// en vez de importar internal/decks porque la autorización de esta lista es una
// decisión de playgroups (membresía compartida), no de decks.
type DeckResponse struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
}
