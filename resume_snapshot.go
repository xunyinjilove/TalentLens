package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ResumeSnapshotHistoryItem struct {
	ID         string `json:"id"`
	FileName   string `json:"fileName"`
	Status     string `json:"status"`
	Score      int    `json:"score"`
	AnalyzedAt string `json:"analyzedAt"`
	Current    bool   `json:"current"`
}

func (a *App) GetResumeSnapshotHistory(resumeID string) ([]ResumeSnapshotHistoryItem, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return nil, fmt.Errorf("简历编号无效")
	}
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if err != nil {
		return nil, err
	}
	var resume Resume
	if err = json.Unmarshal(data, &resume); err != nil {
		return nil, err
	}
	var group *Resume
	for _, item := range a.GetProjectResumes(resume.ProjectID) {
		if item.ID == resumeID {
			group = item
			break
		}
		for _, id := range item.SnapshotCopyIDs {
			if id == resumeID {
				group = item
				break
			}
		}
	}
	if group == nil {
		return nil, fmt.Errorf("简历不在当前项目列表中")
	}
	items := make([]ResumeSnapshotHistoryItem, 0, len(group.SnapshotCopyIDs)+1)
	for _, id := range append([]string{group.ID}, group.SnapshotCopyIDs...) {
		if !interviewIDPattern.MatchString(id) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", id+".json"))
		if err != nil {
			continue
		}
		var copy Resume
		if json.Unmarshal(data, &copy) != nil {
			continue
		}
		stamp := ""
		if copy.Analysis != nil {
			stamp = copy.Analysis.AnalyzedAt
		}
		items = append(items, ResumeSnapshotHistoryItem{ID: id, FileName: copy.FileName, Status: copy.Status, Score: copy.Score, AnalyzedAt: stamp, Current: id == group.ID})
	}
	return items, nil
}

// A fingerprint identifies a document copy, never a person's identity.
// Candidate profiles, contacts and conversation history remain separate.
func resumeSnapshotFingerprint(resume *Resume) string {
	text := strings.TrimSpace(strings.ReplaceAll(resume.Content, "\r\n", "\n"))
	if text == "" || strings.HasPrefix(text, "[简历文件:") || (resume.HasAttachment && resume.AttachmentContent == "") {
		return ""
	}
	// Generated search metadata is not a candidate fact. Changing a search
	// synonym or collection city must not turn the same document into a new one.
	if strings.HasPrefix(text, "【") && strings.Contains(strings.SplitN(text, "\n", 2)[0], "真实推荐牛人档案】") {
		lines := make([]string, 0)
		for _, line := range strings.Split(text, "\n") {
			metadata := false
			for _, prefix := range []string{"检索岗位：", "分流同义词：", "目标城市：", "在线直达网址：", "平台活跃标记："} {
				if strings.HasPrefix(line, prefix) {
					metadata = true
					break
				}
			}
			if !metadata {
				lines = append(lines, line)
			}
		}
		text = strings.Join(lines, "\n")
	}
	if resume.AttachmentContent != "" {
		text += "\n\x00attachment\n" + strings.TrimSpace(strings.ReplaceAll(resume.AttachmentContent, "\r\n", "\n"))
	}
	digest := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", digest)
}

func resumeSnapshotGroupKey(resume *Resume) string {
	fingerprint := resumeSnapshotDocumentKey(resume)
	if fingerprint == "" {
		return "record:" + resume.ID
	}
	platform, id := resumeSourceIdentity(resume)
	if platform == "legacy" {
		return "document:" + fingerprint
	}
	return platform + ":" + id + ":" + fingerprint
}

func resumeSnapshotDocumentKey(resume *Resume) string {
	hash := resumeSnapshotFingerprint(resume)
	if hash == "" {
		return ""
	}
	return conversationCandidateName(resume) + ":" + hash
}

func snapshotEvaluationTime(resume *Resume) time.Time {
	if resume.Analysis != nil {
		if stamp, err := time.Parse(time.RFC3339, resume.Analysis.AnalyzedAt); err == nil {
			return stamp
		}
	}
	return resume.CreatedAt
}

func foldIdenticalResumeSnapshots(resumes []*Resume) []*Resume {
	result := make([]*Resume, 0, len(resumes))
	positions := map[string]int{}
	identities := map[string]map[string]bool{}
	for _, resume := range resumes {
		hash := resumeSnapshotDocumentKey(resume)
		if platform, id := resumeSourceIdentity(resume); platform != "legacy" && hash != "" {
			if identities[hash] == nil {
				identities[hash] = map[string]bool{}
			}
			identities[hash][platform+":"+id] = true
		}
	}
	for _, resume := range resumes {
		key := resumeSnapshotGroupKey(resume)
		if hash := resumeSnapshotDocumentKey(resume); hash != "" && len(identities[hash]) <= 1 {
			key = "document:" + hash
		}
		pos, exists := positions[key]
		if !exists {
			positions[key] = len(result)
			result = append(result, resume)
			continue
		}
		previous := result[pos]
		// Preserve a verified communication target; otherwise prefer a completed,
		// more recent evaluation. Never pick a snapshot copy by its score.
		newer := (resume.Status == "done" && previous.Status != "done") ||
			(resume.Status == previous.Status && snapshotEvaluationTime(resume).After(snapshotEvaluationTime(previous)))
		if is51JobCandidateDetailURL(resume.URL) != is51JobCandidateDetailURL(previous.URL) {
			newer = is51JobCandidateDetailURL(resume.URL)
		}
		if newer {
			resume.SnapshotCopyIDs = append(append([]string{}, previous.SnapshotCopyIDs...), previous.ID)
			result[pos] = resume
		} else {
			previous.SnapshotCopyIDs = append(previous.SnapshotCopyIDs, resume.ID)
		}
	}
	return result
}
