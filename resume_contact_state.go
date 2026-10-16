package main

import "net/url"

func merge51JobContactStates(resumes []*Resume) {
	byIdentity := map[string]*Resume{}
	identity := func(resume *Resume) string {
		if !is51JobCandidateDetailURL(resume.URL) {
			return ""
		}
		parsed, _ := url.Parse(resume.URL)
		return parsed.Query().Get("resumeId")
	}
	for _, resume := range resumes {
		id := identity(resume)
		if id == "" || resume.ContactStatus == "" {
			continue
		}
		previous := byIdentity[id]
		if previous == nil || resume.ContactUpdatedAt > previous.ContactUpdatedAt {
			byIdentity[id] = resume
		}
	}
	for _, resume := range resumes {
		if latest := byIdentity[identity(resume)]; latest != nil {
			resume.ContactStatus, resume.ContactUpdatedAt = latest.ContactStatus, latest.ContactUpdatedAt
			resume.ContactAttachmentBaseline = append([]string{}, latest.ContactAttachmentBaseline...)
		}
	}
}
