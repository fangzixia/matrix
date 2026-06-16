package worker

import "os"

func ID() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "embedded"
}
