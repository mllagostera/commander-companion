package sync

// SyncRequest es el payload para iniciar una sincronización con Moxfield.
type SyncRequest struct {
	MoxfieldID string `json:"moxfield_id"`
}

// SyncResponse es el DTO del estado de un job de sincronización.
type SyncResponse struct {
	Status string `json:"status"`
	JobID  string `json:"job_id"`
}
