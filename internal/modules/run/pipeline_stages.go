package run

import (
	"encoding/json"

	"github.com/google/uuid"
)

func encodePipelineStages(stages []string) string {
	if len(stages) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(stages)
	return string(b)
}

func decodePipelineStages(raw string) []string {
	if raw == "" || raw == "null" {
		return nil
	}
	var stages []string
	if err := json.Unmarshal([]byte(raw), &stages); err != nil {
		return nil
	}
	return stages
}

func sandboxLockKey(projectID uuid.UUID, repositoryID *uuid.UUID) string {
	if repositoryID == nil || *repositoryID == uuid.Nil {
		return projectID.String()
	}
	return projectID.String() + "/" + repositoryID.String()
}
