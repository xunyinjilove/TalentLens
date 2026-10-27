package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// GetProjectAssessmentResumes resolves the assessment page from the same
// active project-candidate links shown in the talent library. This avoids a
// blank assessment when the legacy project ResumeIDs index is stale.
func (a *App) GetProjectAssessmentResumes(projectID string) ([]*Resume, error) {
	project := a.GetProject(projectID)
	if project == nil {
		return nil, fmt.Errorf("招聘职位不存在")
	}
	candidates, err := a.ListTalentCandidates(projectID)
	if err != nil {
		return nil, err
	}
	result := make([]*Resume, 0, len(candidates))
	var db *sql.DB
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.ResumeID)
		if id == "" {
			result = append(result, &Resume{ID: candidate.ProjectCandidateID, ProjectID: projectID,
				FileName: candidate.ResumeFileName, Status: "pending_document"})
			continue
		}
		var resume Resume
		data, readErr := readJSONFile(filepath.Join(a.getDataDir(), "resumes", id+".json"))
		if readErr == nil {
			readErr = json.Unmarshal(data, &resume)
		}
		if readErr != nil {
			if db == nil {
				db, err = a.getRecruitmentDB()
				if err != nil {
					return nil, err
				}
			}
			var raw, content, fileName string
			err = db.QueryRow(`SELECT COALESCE(raw_json,''),COALESCE(content,''),COALESCE(file_name,'')
				FROM resume_versions WHERE legacy_resume_id=? ORDER BY created_at DESC LIMIT 1`, id).Scan(&raw, &content, &fileName)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if raw != "" {
				_ = json.Unmarshal([]byte(raw), &resume)
			}
			if resume.Content == "" {
				resume.Content = content
			}
			if resume.FileName == "" {
				resume.FileName = fileName
			}
			if resume.Status == "" {
				resume.Status = "pending_document"
			}
			resume.ErrorMessage = "原始简历文件未找到，请在人才库核对资料；分析前需补齐原件"
		}
		resume.ID, resume.ProjectID = id, projectID
		if resume.FileName == "" {
			resume.FileName = candidate.ResumeFileName
		}
		if resume.Status == "analyzing" && (a.activeAnalyses == nil || !a.activeAnalyses[id]) {
			resume.Status = "pending"
		}
		a.prepareResumeDecision(&resume, &project.JobConfig)
		result = append(result, &resume)
	}
	return result, nil
}
