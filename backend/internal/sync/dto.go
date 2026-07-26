package sync

// Request es el payload para iniciar una sincronización con Moxfield.
type Request struct {
	MoxfieldID string `json:"moxfield_id"`
}

// Response es el DTO del estado de un job de sincronización.
type Response struct {
	Status string `json:"status"`
	JobID  string `json:"job_id"`
}
