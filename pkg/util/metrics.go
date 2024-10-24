package util

import "time"

func MakePromSubsystem(subsystem string) string {
	return SubsystemPrefix + "_" + subsystem
}

type Status struct {
	Push    *PushStatus     `json:"push"`
	Poll    *PollStatus     `json:"poll"`
	Updates []*UpdateStatus `json:"updates"`
}

type PushStatus struct {
	Last      time.Time `json:"last"`
	Succeeded bool      `json:"succeeded"`
}

type PollStatus struct {
	Last      time.Time `json:"last"`
	Succeeded bool      `json:"succeeded"`
}

type UpdateStatus struct {
	Last      time.Time `json:"last"`
	Domain    string    `json:"domain"`
	IpVersion uint8     `json:"ipVersion"`
	Succeeded bool      `json:"succeeded"`
}
