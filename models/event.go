package models

type (
	// Event - the etcd changed event
	Event struct {
		Type  string `json:"type"`
		Group string `json:"group"`
	}
)
