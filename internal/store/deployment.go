package store

import "time"

type Status int64

const (
	Init Status = iota
	Running
	Done
	Failed
)

func StatusToString(status Status) string {
	switch status {
	case Init:
		return "init"
	case Running:
		return "running"
	case Done:
		return "done"
	case Failed:
		return "failed"
	}
	return ""
}

type Deployment struct {
	UUID       string     `json:"uuid"`
	Generation Generation `json:"generation"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    time.Time  `json:"ended_at"`
	// It is ignored in the JSON marshaling
	Err          error  `json:"-"`
	ErrorMsg     string `json:"error_msg"`
	RestartComin bool   `json:"restart_comin"`
	ProfilePath  string `json:"profile_path"`
	Status       Status `json:"status"`
	Operation    string `json:"operation"`
}

func (d Deployment) IsTesting() bool {
	return d.Operation == "test"
}
