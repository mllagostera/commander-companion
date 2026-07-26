package playgroups

// CreatePlaygroupRequest es el payload para crear un grupo de juego.
type CreatePlaygroupRequest struct {
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
}
