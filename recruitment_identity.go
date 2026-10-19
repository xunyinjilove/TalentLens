package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SuspectedDuplicate struct {
	CandidateID      string `json:"candidateId"`
	OtherCandidateID string `json:"otherCandidateId"`
	Name             string `json:"name"`
	Reason           string `json:"reason"`
}

func (a *App) ListSuspectedDuplicates() ([]SuspectedDuplicate, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT a.id,b.id,a.display_name FROM candidates a
		JOIN candidates b ON a.id<b.id AND lower(trim(a.display_name))=lower(trim(b.display_name))
		LEFT JOIN dismissed_duplicate_hints d ON d.candidate_id=a.id AND d.other_candidate_id=b.id
		WHERE trim(a.display_name)<>'' AND a.display_name<>'候选人' AND d.candidate_id IS NULL
		ORDER BY a.display_name,a.id,b.id LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SuspectedDuplicate, 0)
	for rows.Next() {
		var item SuspectedDuplicate
		if err := rows.Scan(&item.CandidateID, &item.OtherCandidateID, &item.Name); err != nil {
			return nil, err
		}
		item.Reason = "姓名相同，仅为疑似重复；需 HR 核对平台身份或联系方式"
		out = append(out, item)
	}
	return out, rows.Err()
}

// DismissSuspectedDuplicate removes only the review hint. Both profiles remain.
func (a *App) DismissSuspectedDuplicate(candidateID, otherCandidateID string) error {
	if candidateID == "" || otherCandidateID == "" || candidateID == otherCandidateID {
		return fmt.Errorf("请选择有效的疑似重复记录")
	}
	if candidateID > otherCandidateID {
		candidateID, otherCandidateID = otherCandidateID, candidateID
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	result, err := db.Exec(`INSERT OR IGNORE INTO dismissed_duplicate_hints(candidate_id,other_candidate_id,dismissed_at)
		SELECT a.id,b.id,? FROM candidates a JOIN candidates b ON a.id=? AND b.id=?
		WHERE lower(trim(a.display_name))=lower(trim(b.display_name)) AND trim(a.display_name)<>''`, time.Now().UTC().Format(time.RFC3339Nano), candidateID, otherCandidateID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		var exists int
		if err := db.QueryRow("SELECT 1 FROM dismissed_duplicate_hints WHERE candidate_id=? AND other_candidate_id=?", candidateID, otherCandidateID).Scan(&exists); err != nil {
			return fmt.Errorf("疑似重复记录已不存在")
		}
	}
	return nil
}

// ClearSuspectedDuplicates dismisses all currently detected pairs, not people.
func (a *App) ClearSuspectedDuplicates() (int64, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return 0, err
	}
	result, err := db.Exec(`INSERT OR IGNORE INTO dismissed_duplicate_hints(candidate_id,other_candidate_id,dismissed_at)
		SELECT a.id,b.id,? FROM candidates a JOIN candidates b
		ON a.id<b.id AND lower(trim(a.display_name))=lower(trim(b.display_name))
		WHERE trim(a.display_name)<>'' AND a.display_name<>'候选人'`, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (a *App) MergeCandidates(primaryID, duplicateID, actor, reason string) error {
	if primaryID == "" || duplicateID == "" || primaryID == duplicateID {
		return fmt.Errorf("请选择两个不同的人才档案")
	}
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(reason) == "" {
		return fmt.Errorf("请填写确认人及合并依据")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range []string{primaryID, duplicateID} {
		var present int
		if err := tx.QueryRow("SELECT 1 FROM candidates WHERE id=?", id).Scan(&present); err != nil {
			return err
		}
	}
	var emailA, phoneA, emailB, phoneB string
	if err := tx.QueryRow("SELECT verified_email,verified_phone FROM candidates WHERE id=?", primaryID).Scan(&emailA, &phoneA); err != nil {
		return err
	}
	if err := tx.QueryRow("SELECT verified_email,verified_phone FROM candidates WHERE id=?", duplicateID).Scan(&emailB, &phoneB); err != nil {
		return err
	}
	if emailA != "" && emailB != "" && emailA != emailB {
		return fmt.Errorf("两个档案的已核实邮箱冲突，请先核对")
	}
	if phoneA != "" && phoneB != "" && phoneA != phoneB {
		return fmt.Errorf("两个档案的已核实电话冲突，请先核对")
	}
	rows, err := tx.Query("SELECT id,project_id FROM project_candidates WHERE candidate_id=?", duplicateID)
	if err != nil {
		return err
	}
	type link struct{ id, projectID string }
	links := make([]link, 0)
	for rows.Next() {
		var item link
		if err := rows.Scan(&item.id, &item.projectID); err != nil {
			rows.Close()
			return err
		}
		links = append(links, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range links {
		var existing string
		err = tx.QueryRow("SELECT id FROM project_candidates WHERE project_id=? AND candidate_id=?", item.projectID, primaryID).Scan(&existing)
		if err == sql.ErrNoRows {
			if _, err = tx.Exec("UPDATE project_candidates SET candidate_id=? WHERE id=?", primaryID, item.id); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO project_candidate_resumes(project_candidate_id,resume_version_id,created_at) SELECT ?,resume_version_id,created_at FROM project_candidate_resumes WHERE project_candidate_id=?", existing, item.id); err != nil {
			return err
		}
		for _, table := range []string{"evaluations", "follow_up_events", "interviews", "outreach_items"} {
			if _, err = tx.Exec("UPDATE "+table+" SET project_candidate_id=? WHERE project_candidate_id=?", existing, item.id); err != nil {
				return err
			}
		}
		if _, err = tx.Exec("DELETE FROM project_candidate_resumes WHERE project_candidate_id=?", item.id); err != nil {
			return err
		}
		if _, err = tx.Exec("DELETE FROM project_candidates WHERE id=?", item.id); err != nil {
			return err
		}
	}
	for _, table := range []string{"candidate_sources", "resume_versions", "applications"} {
		if _, err = tx.Exec("UPDATE "+table+" SET candidate_id=? WHERE candidate_id=?", primaryID, duplicateID); err != nil {
			return err
		}
	}
	if emailA == "" {
		emailA = emailB
	}
	if phoneA == "" {
		phoneA = phoneB
	}
	if _, err = tx.Exec("DELETE FROM candidates WHERE id=?", duplicateID); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE candidates SET verified_email=?,verified_phone=?,updated_at=? WHERE id=?", emailA, phoneA, time.Now().UTC().Format(time.RFC3339Nano), primaryID); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO candidate_merge_events(id,primary_candidate_id,duplicate_candidate_id,actor,reason,created_at) VALUES(?,?,?,?,?,?)", recruitmentID("merge"), primaryID, duplicateID, actor, reason, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) VerifyCandidateContact(candidateID, kind, value, actor string) (string, error) {
	value = strings.TrimSpace(value)
	if kind == "email" {
		value = strings.ToLower(value)
		if !strings.Contains(value, "@") {
			return "", fmt.Errorf("邮箱格式无效")
		}
	}
	if kind == "phone" {
		value = strings.NewReplacer(" ", "", "-", "").Replace(value)
		if len(value) < 7 {
			return "", fmt.Errorf("电话格式无效")
		}
	}
	if kind != "email" && kind != "phone" {
		return "", fmt.Errorf("联系方式类型无效")
	}
	if strings.TrimSpace(actor) == "" {
		return "", fmt.Errorf("请填写核实人")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return "", err
	}
	column := "verified_email"
	if kind == "phone" {
		column = "verified_phone"
	}
	var existing string
	err = db.QueryRow("SELECT id FROM candidates WHERE "+column+"=? AND id<>?", value, candidateID).Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if existing != "" {
		if err := a.MergeCandidates(existing, candidateID, actor, "已核实相同"+kind); err != nil {
			return "", err
		}
		candidateID = existing
	}
	result, err := db.Exec("UPDATE candidates SET "+column+"=?,updated_at=? WHERE id=?", value, time.Now().UTC().Format(time.RFC3339Nano), candidateID)
	if err != nil {
		return "", err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return "", sql.ErrNoRows
	}
	return candidateID, nil
}
