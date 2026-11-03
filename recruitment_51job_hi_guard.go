package main

import "strings"

// known51JobHardFailure blocks outreach only when the saved profile contains
// affirmative evidence of a failed project rule. Unknown data is not a reject.
func (a *App) known51JobHardFailure(resume *Resume) string {
	if resume == nil || resume.SourcePlatform != "51job" || resume.ProjectID == "" {
		return ""
	}
	criteria, err := a.GetTalentManagementStrictCriteria(resume.ProjectID)
	if err != nil || criteria == nil {
		return ""
	}
	profile := strings.TrimSpace(resume.RawProfileJSON)
	if profile == "" {
		profile = resume.Content
	}
	_, checks := EvaluateCandidateTriState(profile, *criteria)
	for _, check := range checks {
		if check.Status == "fail" {
			return check.Evidence
		}
	}
	return ""
}
