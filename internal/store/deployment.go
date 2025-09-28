package store

import pb "github.com/nlewo/comin/internal/protobuf"

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

func StringToStatus(statusStr string) Status {
	switch statusStr {
	case "init":
		return Init
	case "running":
		return Running
	case "done":
		return Done
	case "failed":
		return Failed
	}
	return Init
}

func IsTesting(d *pb.Deployment) bool {
	return d.Operation == "test"
}
