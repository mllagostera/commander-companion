package moxfieldimport

// JobResponse es el DTO de un job de import masivo enviado al cliente.
type JobResponse struct {
	ID               string  `json:"id"`
	MoxfieldUsername string  `json:"moxfield_username"`
	Status           string  `json:"status"`
	TotalDecks       *int32  `json:"total_decks,omitempty"`
	ImportedCount    int32   `json:"imported_count"`
	FailedCount      int32   `json:"failed_count"`
	ErrorMessage     *string `json:"error_message,omitempty"`
}
