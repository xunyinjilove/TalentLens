package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// analysisVersionKey tracks source and editable job settings, excluding the
// generated radar profile and other derived fields.
func analysisVersionKey(resume *Resume, job *JobConfig) string {
	var config interface{}
	if job != nil {
		copy := *job
		copy.RadarMetrics = nil
		copy.RadarProfileHash = ""
		copy.RadarProfileVersion = 0
		copy.RadarSourceHash = ""
		config = copy
	}
	encoded, _ := json.Marshal([]interface{}{resume.ProjectID, resume.Content, resume.AttachmentContent, resume.AnalysisRuleVersion, config})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
