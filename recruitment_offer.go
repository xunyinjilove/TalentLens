package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// OfferCard is an internal planning record. It never sends a message or email.
type OfferCard struct {
	ProjectCandidateID string `json:"projectCandidateId"`
	ProjectID          string `json:"projectId"`
	CandidateName      string `json:"candidateName"`
	ProjectName        string `json:"projectName"`
	JobTitle           string `json:"jobTitle"`
	SalaryPackage      string `json:"salaryPackage"`
	WorkLocation       string `json:"workLocation"`
	PlannedStartDate   string `json:"plannedStartDate"`
	ValidUntil         string `json:"validUntil"`
	Note               string `json:"note"`
	Owner              string `json:"owner"`
	Status             string `json:"status"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type OfferHistoryItem struct {
	ID           string `json:"id"`
	Action       string `json:"action"`
	SnapshotJSON string `json:"snapshotJson"`
	Actor        string `json:"actor"`
	CreatedAt    string `json:"createdAt"`
}

func (a *App) ListOfferCards(projectID string) ([]OfferCard, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT pc.id,pc.project_id,c.display_name,p.name,p.job_title,
		COALESCE(o.salary_package,''),COALESCE(o.work_location,''),COALESCE(o.planned_start_date,''),
		COALESCE(o.valid_until,''),COALESCE(o.note,''),COALESCE(o.owner,pc.owner),
		COALESCE(o.status,'draft'),COALESCE(o.created_at,''),COALESCE(o.updated_at,'')
		FROM project_candidates pc JOIN candidates c ON c.id=pc.candidate_id
		JOIN projects p ON p.id=pc.project_id LEFT JOIN offer_cards o ON o.project_candidate_id=pc.id
		WHERE pc.archived_at='' AND (pc.stage='offer' OR o.project_candidate_id IS NOT NULL)
		AND (?='' OR pc.project_id=?) ORDER BY pc.updated_at DESC,pc.id`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]OfferCard, 0)
	for rows.Next() {
		var card OfferCard
		if err := rows.Scan(&card.ProjectCandidateID, &card.ProjectID, &card.CandidateName, &card.ProjectName, &card.JobTitle,
			&card.SalaryPackage, &card.WorkLocation, &card.PlannedStartDate, &card.ValidUntil,
			&card.Note, &card.Owner, &card.Status, &card.CreatedAt, &card.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, card)
	}
	return result, rows.Err()
}

func (a *App) SaveOfferCard(card OfferCard, actor string) error {
	if strings.TrimSpace(card.ProjectCandidateID) == "" {
		return fmt.Errorf("请选择候选人")
	}
	if card.PlannedStartDate != "" {
		if _, err := time.Parse("2006-01-02", card.PlannedStartDate); err != nil {
			return fmt.Errorf("拟入职日期无效")
		}
	}
	if card.ValidUntil != "" {
		if _, err := time.Parse("2006-01-02", card.ValidUntil); err != nil {
			return fmt.Errorf("Offer 有效期无效")
		}
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
	var stage, archived string
	if err := tx.QueryRow(`SELECT stage,archived_at FROM project_candidates WHERE id=?`, card.ProjectCandidateID).Scan(&stage, &archived); err != nil {
		return err
	}
	if archived != "" {
		return fmt.Errorf("已归档人选不能编辑 Offer")
	}
	if stage != "offer" && stage != "hired" {
		return fmt.Errorf("请先将候选人调整到 Offer 阶段")
	}
	var currentStatus string
	err = tx.QueryRow(`SELECT status FROM offer_cards WHERE project_candidate_id=?`, card.ProjectCandidateID).Scan(&currentStatus)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		currentStatus = "draft"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	card.SalaryPackage = strings.TrimSpace(card.SalaryPackage)
	card.WorkLocation = strings.TrimSpace(card.WorkLocation)
	card.Note = strings.TrimSpace(card.Note)
	card.Owner = strings.TrimSpace(card.Owner)
	_, err = tx.Exec(`INSERT INTO offer_cards(project_candidate_id,salary_package,work_location,planned_start_date,valid_until,note,owner,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(project_candidate_id) DO UPDATE SET salary_package=excluded.salary_package,
		work_location=excluded.work_location,planned_start_date=excluded.planned_start_date,valid_until=excluded.valid_until,
		note=excluded.note,owner=excluded.owner,updated_at=excluded.updated_at`, card.ProjectCandidateID, card.SalaryPackage,
		card.WorkLocation, card.PlannedStartDate, card.ValidUntil, card.Note, card.Owner, currentStatus, now, now)
	if err != nil {
		return err
	}
	card.Status = currentStatus
	card.UpdatedAt = now
	if err = insertOfferHistory(tx, card, "saved", actor, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) SetOfferStatus(projectCandidateID, status, actor string) error {
	allowed := map[string]map[string]bool{
		"draft":     {"confirmed": true, "withdrawn": true},
		"confirmed": {"accepted": true, "declined": true, "withdrawn": true},
		"declined":  {"draft": true}, "withdrawn": {"draft": true},
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
	var card OfferCard
	err = tx.QueryRow(`SELECT project_candidate_id,salary_package,work_location,planned_start_date,valid_until,note,owner,status,created_at,updated_at
		FROM offer_cards WHERE project_candidate_id=?`, projectCandidateID).Scan(&card.ProjectCandidateID, &card.SalaryPackage,
		&card.WorkLocation, &card.PlannedStartDate, &card.ValidUntil, &card.Note, &card.Owner, &card.Status, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return fmt.Errorf("请先保存 Offer 草稿")
	}
	if !allowed[card.Status][status] {
		return fmt.Errorf("Offer 状态不能从 %s 变为 %s", card.Status, status)
	}
	if status == "confirmed" && (card.SalaryPackage == "" || card.PlannedStartDate == "") {
		return fmt.Errorf("确认前请填写薪酬方案与拟入职日期")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.Exec(`UPDATE offer_cards SET status=?,updated_at=? WHERE project_candidate_id=?`, status, now, projectCandidateID); err != nil {
		return err
	}
	card.Status = status
	card.UpdatedAt = now
	if err = insertOfferHistory(tx, card, "status:"+status, actor, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) ListOfferHistory(projectCandidateID string) ([]OfferHistoryItem, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id,action,snapshot_json,actor,created_at FROM offer_history WHERE project_candidate_id=? ORDER BY created_at DESC,id DESC`, projectCandidateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]OfferHistoryItem, 0)
	for rows.Next() {
		var item OfferHistoryItem
		if err := rows.Scan(&item.ID, &item.Action, &item.SnapshotJSON, &item.Actor, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func insertOfferHistory(tx *sql.Tx, card OfferCard, action, actor, now string) error {
	snapshot, err := json.Marshal(card)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO offer_history(id,project_candidate_id,action,snapshot_json,actor,created_at) VALUES(?,?,?,?,?,?)`,
		recruitmentID("offer_history"), card.ProjectCandidateID, action, string(snapshot), strings.TrimSpace(actor), now)
	return err
}

func ensureAcceptedOffer(tx *sql.Tx, projectCandidateID string) error {
	var status string
	if err := tx.QueryRow(`SELECT status FROM offer_cards WHERE project_candidate_id=?`, projectCandidateID).Scan(&status); err != nil || status != "accepted" {
		return fmt.Errorf("请先在 Offer 管理中将该候选人的 Offer 标记为已接受")
	}
	return nil
}
