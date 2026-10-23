package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// The original JSON files remain intact. SQLite owns the new cross-project
// recruitment data; a one-time import makes existing projects visible there.
const recruitmentSchema = `
CREATE TABLE IF NOT EXISTS migration_meta (
  key TEXT PRIMARY KEY, value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, department TEXT NOT NULL DEFAULT '',
  recruiter TEXT NOT NULL DEFAULT '', headcount INTEGER NOT NULL DEFAULT 1,
  job_title TEXT NOT NULL DEFAULT '', legacy_json TEXT NOT NULL,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS candidates (
  id TEXT PRIMARY KEY, display_name TEXT NOT NULL, verified_email TEXT NOT NULL DEFAULT '',
  verified_phone TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS candidates_verified_email ON candidates(lower(verified_email)) WHERE verified_email<>'';
CREATE UNIQUE INDEX IF NOT EXISTS candidates_verified_phone ON candidates(verified_phone) WHERE verified_phone<>'';
CREATE TABLE IF NOT EXISTS candidate_merge_events (
  id TEXT PRIMARY KEY, primary_candidate_id TEXT NOT NULL,
  duplicate_candidate_id TEXT NOT NULL, actor TEXT NOT NULL,
  reason TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS dismissed_duplicate_hints (
  candidate_id TEXT NOT NULL, other_candidate_id TEXT NOT NULL,
  dismissed_at TEXT NOT NULL,
  PRIMARY KEY(candidate_id, other_candidate_id)
);
CREATE TABLE IF NOT EXISTS candidate_sources (
  id TEXT PRIMARY KEY, candidate_id TEXT NOT NULL REFERENCES candidates(id),
  platform TEXT NOT NULL, platform_account TEXT NOT NULL DEFAULT '',
  platform_candidate_id TEXT NOT NULL DEFAULT '', profile_url TEXT NOT NULL DEFAULT '',
  source_kind TEXT NOT NULL, first_seen_at TEXT NOT NULL, last_seen_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS candidate_source_identity
  ON candidate_sources(platform, platform_account, platform_candidate_id)
  WHERE platform_candidate_id <> '';
CREATE TABLE IF NOT EXISTS resume_versions (
  id TEXT PRIMARY KEY, candidate_id TEXT NOT NULL REFERENCES candidates(id),
  source_id TEXT REFERENCES candidate_sources(id), legacy_resume_id TEXT UNIQUE,
  file_path TEXT NOT NULL DEFAULT '', file_name TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '', content_sha256 TEXT NOT NULL,
  raw_json TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS project_candidates (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id),
  candidate_id TEXT NOT NULL REFERENCES candidates(id),
  stage TEXT NOT NULL DEFAULT 'new', owner TEXT NOT NULL DEFAULT '',
  next_action_at TEXT NOT NULL DEFAULT '', archived_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL, UNIQUE(project_id, candidate_id)
);
CREATE TABLE IF NOT EXISTS project_candidate_resumes (
  project_candidate_id TEXT NOT NULL REFERENCES project_candidates(id),
  resume_version_id TEXT NOT NULL REFERENCES resume_versions(id),
  created_at TEXT NOT NULL,
  PRIMARY KEY(project_candidate_id,resume_version_id)
);
CREATE INDEX IF NOT EXISTS project_candidates_project_stage
  ON project_candidates(project_id, stage);
CREATE TABLE IF NOT EXISTS evaluations (
  id TEXT PRIMARY KEY, project_candidate_id TEXT NOT NULL REFERENCES project_candidates(id),
  resume_version_id TEXT NOT NULL REFERENCES resume_versions(id),
  rule_version INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL,
  ability_score INTEGER NOT NULL DEFAULT 0, redline_status TEXT NOT NULL DEFAULT '',
  confirmed_bonus_count INTEGER NOT NULL DEFAULT 0, analysis_json TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS job_rules (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id),
  kind TEXT NOT NULL, criterion TEXT NOT NULL, source TEXT NOT NULL,
  status TEXT NOT NULL, version INTEGER NOT NULL, approved_by TEXT NOT NULL DEFAULT '',
  approved_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS search_tasks (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id),
  config_json TEXT NOT NULL, status TEXT NOT NULL, target_count INTEGER NOT NULL,
  captured_count INTEGER NOT NULL DEFAULT 0, analyzed_count INTEGER NOT NULL DEFAULT 0,
  failed_count INTEGER NOT NULL DEFAULT 0, current_platform TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS analysis_jobs (
  id TEXT PRIMARY KEY, project_candidate_id TEXT NOT NULL REFERENCES project_candidates(id),
  resume_version_id TEXT NOT NULL REFERENCES resume_versions(id),
  rule_version INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  UNIQUE(project_candidate_id, resume_version_id, rule_version)
);
CREATE TABLE IF NOT EXISTS platform_job_mappings (
  platform TEXT NOT NULL, platform_account TEXT NOT NULL DEFAULT '',
  platform_job_id TEXT NOT NULL, project_id TEXT NOT NULL REFERENCES projects(id),
  PRIMARY KEY(platform, platform_account, platform_job_id)
);
CREATE TABLE IF NOT EXISTS applications (
  id TEXT PRIMARY KEY, platform TEXT NOT NULL, platform_account TEXT NOT NULL DEFAULT '',
  platform_application_id TEXT NOT NULL, platform_job_id TEXT NOT NULL DEFAULT '',
  candidate_id TEXT REFERENCES candidates(id), project_id TEXT REFERENCES projects(id),
  status TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT '', raw_json TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  UNIQUE(platform, platform_account, platform_application_id)
);
CREATE TABLE IF NOT EXISTS import_runs (
  id TEXT PRIMARY KEY, platform TEXT NOT NULL, status TEXT NOT NULL,
  cursor TEXT NOT NULL DEFAULT '', added_count INTEGER NOT NULL DEFAULT 0,
  duplicate_count INTEGER NOT NULL DEFAULT 0, unassigned_count INTEGER NOT NULL DEFAULT 0,
  failed_count INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS outreach_batches (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id),
  status TEXT NOT NULL, created_at TEXT NOT NULL, confirmed_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS outreach_items (
  id TEXT PRIMARY KEY, batch_id TEXT NOT NULL REFERENCES outreach_batches(id),
  project_candidate_id TEXT NOT NULL REFERENCES project_candidates(id),
  platform TEXT NOT NULL, platform_candidate_id TEXT NOT NULL DEFAULT '',
  platform_job_id TEXT NOT NULL DEFAULT '', message TEXT NOT NULL,
  status TEXT NOT NULL, result_json TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS outreach_authorizations (
  batch_id TEXT PRIMARY KEY REFERENCES outreach_batches(id) ON DELETE CASCADE,
  job_id TEXT NOT NULL, job_title TEXT NOT NULL,
  per_person_limit INTEGER NOT NULL, total_limit INTEGER NOT NULL,
  reserved_points INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '', confirmed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS candidate_ai_outreach_messages (
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  candidate_key TEXT NOT NULL,
  message TEXT NOT NULL,
  evidence TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project_id, candidate_key)
);
CREATE TABLE IF NOT EXISTS follow_up_events (
  id TEXT PRIMARY KEY, project_candidate_id TEXT NOT NULL REFERENCES project_candidates(id),
  event_type TEXT NOT NULL, from_stage TEXT NOT NULL DEFAULT '',
  to_stage TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '',
  actor TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS follow_up_events_candidate_created
  ON follow_up_events(project_candidate_id, created_at DESC);
CREATE TABLE IF NOT EXISTS interviews (
  id TEXT PRIMARY KEY, project_candidate_id TEXT NOT NULL REFERENCES project_candidates(id),
  starts_at TEXT NOT NULL, ends_at TEXT NOT NULL, interviewer TEXT NOT NULL DEFAULT '',
  method TEXT NOT NULL DEFAULT '', location TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'scheduled',
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS interviews_candidate_schedule
  ON interviews(project_candidate_id, status, starts_at);
CREATE TABLE IF NOT EXISTS today_todos (
  id TEXT PRIMARY KEY, title TEXT NOT NULL,
  created_at TEXT NOT NULL, completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS today_todos_completed_at ON today_todos(completed_at);
CREATE TABLE IF NOT EXISTS communication_jobs (
  id TEXT PRIMARY KEY,
  platform TEXT NOT NULL,
  scope_account TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL,
  company_id TEXT NOT NULL DEFAULT '',
  company_name TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  work_area TEXT NOT NULL DEFAULT '',
  sources TEXT NOT NULL DEFAULT '',
  publish_status TEXT NOT NULL DEFAULT '',
  is_valid INTEGER NOT NULL DEFAULT 1,
  last_synced_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_comm_jobs_unique ON communication_jobs(platform, scope_account, job_id, company_id);
CREATE TABLE IF NOT EXISTS project_communication_jobs (
  project_id TEXT NOT NULL REFERENCES projects(id),
  platform TEXT NOT NULL,
  scope_account TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL,
  job_title TEXT NOT NULL,
  company_id TEXT NOT NULL DEFAULT '',
  confirmed_at TEXT NOT NULL,
  PRIMARY KEY (project_id, platform, scope_account)
);
CREATE TABLE IF NOT EXISTS communication_job_sync_snapshots (
  id TEXT PRIMARY KEY,
  platform TEXT NOT NULL,
  scope_account TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  total_jobs INTEGER NOT NULL DEFAULT 0,
  error_message TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS talent_management_filter_runs (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  job_title TEXT NOT NULL DEFAULT '',
  mode TEXT NOT NULL DEFAULT 'dry_run',
  status TEXT NOT NULL DEFAULT 'running',
  filter_json TEXT NOT NULL DEFAULT '{}',
  total_scanned INTEGER NOT NULL DEFAULT 0,
  unfit_count INTEGER NOT NULL DEFAULT 0,
  keep_matched_count INTEGER NOT NULL DEFAULT 0,
  keep_unknown_count INTEGER NOT NULL DEFAULT 0,
  whitelist_skip_count INTEGER NOT NULL DEFAULT 0,
  imported_count INTEGER NOT NULL DEFAULT 0,
  message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS talent_management_filter_items (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES talent_management_filter_runs(id) ON DELETE CASCADE,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  candidate_key TEXT NOT NULL DEFAULT '',
  resume_id TEXT NOT NULL DEFAULT '',
  candidate_name TEXT NOT NULL DEFAULT '',
  decision TEXT NOT NULL DEFAULT '',
  primary_reason_code TEXT NOT NULL DEFAULT '',
  reason_label TEXT NOT NULL DEFAULT '',
  clicked_reason_button TEXT NOT NULL DEFAULT '',
  action_status TEXT NOT NULL DEFAULT '',
  card_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tm_filter_items_run ON talent_management_filter_items(run_id);
`

func (a *App) getRecruitmentDB() (*sql.DB, error) {
	a.recruitmentDBMu.Lock()
	defer a.recruitmentDBMu.Unlock()
	dir := a.getDataDir()
	if a.recruitmentDB != nil && a.recruitmentDir == dir {
		return a.recruitmentDB, nil
	}
	if a.recruitmentDB != nil {
		_ = a.recruitmentDB.Close()
		a.recruitmentDB = nil
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "recruitment.sqlite"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err == nil {
		_, err = db.Exec(recruitmentSchema)
	}
	if err == nil {
		err = ensureTalentArchiveColumn(db)
	}
	if err == nil {
		_, err = db.Exec(`INSERT OR IGNORE INTO project_candidate_resumes(project_candidate_id,resume_version_id,created_at)
			SELECT pc.id,rv.id,rv.created_at FROM project_candidates pc JOIN resume_versions rv ON rv.candidate_id=pc.candidate_id
			WHERE json_valid(rv.raw_json) AND json_extract(rv.raw_json,'$.project_id')=pc.project_id`)
	}
	if err == nil {
		err = migrateLegacyRecruitmentData(db, dir)
	}
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	a.recruitmentDB = db
	a.recruitmentDir = dir
	return db, nil
}

func ensureTalentArchiveColumn(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(project_candidates)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var id, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err = rows.Scan(&id, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == "archived_at" {
			found = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil || found {
		return err
	}
	_, err = db.Exec("ALTER TABLE project_candidates ADD COLUMN archived_at TEXT NOT NULL DEFAULT ''")
	return err
}

func (a *App) closeRecruitmentDB() {
	a.recruitmentDBMu.Lock()
	defer a.recruitmentDBMu.Unlock()
	if a.recruitmentDB != nil {
		_ = a.recruitmentDB.Close()
		a.recruitmentDB = nil
	}
}

func (a *App) activeRecruitmentDB() *sql.DB {
	a.recruitmentDBMu.Lock()
	defer a.recruitmentDBMu.Unlock()
	return a.recruitmentDB
}

func resumeSourceIdentity(resume *Resume) (string, string) {
	platform := strings.TrimSpace(resume.SourcePlatform)
	id := strings.TrimSpace(resume.PlatformCandidateID)
	if platform == "51job" && id == "" && is51JobCandidateDetailURL(resume.URL) {
		parsed, _ := url.Parse(resume.URL)
		id = parsed.Query().Get("resumeId")
	}
	if platform == "" && is51JobCandidateDetailURL(resume.URL) {
		parsed, _ := url.Parse(resume.URL)
		platform, id = "51job", parsed.Query().Get("resumeId")
	}
	if platform == "" || id == "" {
		return "legacy", resume.ID
	}
	return platform, id
}

func (a *App) isKnownProjectSource(projectID string, resume *Resume) (bool, error) {
	platform, id := resumeSourceIdentity(resume)
	if platform == "legacy" {
		if resumeSnapshotFingerprint(resume) != "" {
			for _, existing := range a.GetProjectResumes(projectID) {
				if resumeSnapshotDocumentKey(existing) == resumeSnapshotDocumentKey(resume) {
					return true, nil
				}
			}
		}
		return false, nil
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return false, err
	}
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM candidate_sources cs JOIN project_candidates pc ON pc.candidate_id=cs.candidate_id
		WHERE pc.project_id=? AND cs.platform=? AND cs.platform_candidate_id=?`, projectID, platform, id).Scan(&count)
	return count > 0, err
}

func upsertRecruitmentCandidate(tx *sql.Tx, resume *Resume, stamp, now string) (string, string, error) {
	platform, platformID := resumeSourceIdentity(resume)
	candidateID := "legacy_candidate_" + resume.ID
	sourceID := "legacy_source_" + resume.ID
	var existingCandidate, existingSource string
	err := tx.QueryRow(`SELECT candidate_id,id FROM candidate_sources WHERE platform=? AND platform_account='' AND platform_candidate_id=?`,
		platform, platformID).Scan(&existingCandidate, &existingSource)
	if err == nil {
		candidateID, sourceID = existingCandidate, existingSource
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}
	name := strings.TrimSpace(resume.FileName)
	if resume.Analysis != nil && strings.TrimSpace(resume.Analysis.CandidateName) != "" {
		name = strings.TrimSpace(resume.Analysis.CandidateName)
	}
	if name == "" {
		name = "候选人"
	}
	if _, err = tx.Exec(`INSERT OR IGNORE INTO candidates(id,display_name,created_at,updated_at) VALUES(?,?,?,?)`, candidateID, name, stamp, now); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(`INSERT INTO candidate_sources
		(id,candidate_id,platform,platform_candidate_id,profile_url,source_kind,first_seen_at,last_seen_at)
		VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		profile_url=CASE WHEN excluded.profile_url<>'' THEN excluded.profile_url ELSE candidate_sources.profile_url END,
		last_seen_at=excluded.last_seen_at`,
		sourceID, candidateID, platform, platformID, resume.URL, "legacy_import", stamp, now); err != nil {
		return "", "", err
	}
	return candidateID, sourceID, nil
}

// The legacy screens still write JSON. While they are being moved to SQLite,
// mirror successful legacy writes so the new workbench never shows stale data.
func (a *App) syncRecruitmentProject(project *Project) {
	db := a.activeRecruitmentDB()
	if db == nil || project == nil {
		return
	}
	raw, err := json.Marshal(project)
	if err != nil {
		return
	}
	_, err = db.Exec(`INSERT INTO projects
		(id,name,department,recruiter,headcount,job_title,legacy_json,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		name=excluded.name, department=excluded.department, recruiter=excluded.recruiter,
		headcount=excluded.headcount, job_title=excluded.job_title,
		legacy_json=excluded.legacy_json, updated_at=excluded.updated_at`,
		project.ID, project.Name, project.Department, project.Recruiter, project.Headcount,
		project.JobConfig.Title, string(raw), project.CreatedAt.Format(time.RFC3339Nano),
		project.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[RecruitmentStore] project sync failed: %v\n", err)
	}
}

func (a *App) deleteRecruitmentProject(projectID string) error {
	db, err := a.getRecruitmentDB()
	if err != nil || projectID == "" {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queries := []string{
		`DELETE FROM interviews WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM follow_up_events WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM outreach_items WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM outreach_items WHERE batch_id IN (SELECT id FROM outreach_batches WHERE project_id = ?)`,
		`DELETE FROM outreach_batches WHERE project_id = ?`,
		`DELETE FROM candidate_ai_outreach_messages WHERE project_id = ?`,
		`DELETE FROM talent_management_filter_items WHERE project_id = ?`,
		`DELETE FROM talent_management_filter_runs WHERE project_id = ?`,
		`DELETE FROM analysis_jobs WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM evaluations WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM project_candidate_resumes WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM project_candidates WHERE project_id = ?`,
		`DELETE FROM search_tasks WHERE project_id = ?`,
		`DELETE FROM job_rules WHERE project_id = ?`,
		`DELETE FROM platform_job_mappings WHERE project_id = ?`,
		`UPDATE applications SET project_id = NULL WHERE project_id = ?`,
		`DELETE FROM projects WHERE id = ?`,
	}
	for _, q := range queries {
		if _, err := tx.Exec(q, projectID); err != nil {
			fmt.Fprintf(os.Stderr, "[RecruitmentStore] delete project query failed (%s): %v\n", q, err)
			return err
		}
	}
	return tx.Commit()
}

func (a *App) clearRecruitmentProjectResumes(projectID string) error {
	db, err := a.getRecruitmentDB()
	if err != nil || projectID == "" {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queries := []string{
		`DELETE FROM interviews WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM follow_up_events WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM outreach_items WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM outreach_items WHERE batch_id IN (SELECT id FROM outreach_batches WHERE project_id = ?)`,
		`DELETE FROM outreach_batches WHERE project_id = ?`,
		`DELETE FROM candidate_ai_outreach_messages WHERE project_id = ?`,
		`DELETE FROM talent_management_filter_items WHERE project_id = ?`,
		`DELETE FROM talent_management_filter_runs WHERE project_id = ?`,
		`DELETE FROM analysis_jobs WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM evaluations WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM project_candidate_resumes WHERE project_candidate_id IN (SELECT id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM resume_versions WHERE candidate_id IN (SELECT candidate_id FROM project_candidates WHERE project_id = ?)`,
		`DELETE FROM project_candidates WHERE project_id = ?`,
		`UPDATE applications SET project_id = NULL WHERE project_id = ?`,
	}
	for _, q := range queries {
		if _, err := tx.Exec(q, projectID); err != nil {
			fmt.Fprintf(os.Stderr, "[RecruitmentStore] clear project resumes query failed (%s): %v\n", q, err)
			return err
		}
	}
	return tx.Commit()
}

func (a *App) deleteRecruitmentResume(resumeID string) error {
	db, err := a.getRecruitmentDB()
	if err != nil || resumeID == "" {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	versionID := "legacy_resume_" + resumeID
	execs := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM evaluations WHERE resume_version_id = ? OR resume_version_id IN (SELECT id FROM resume_versions WHERE legacy_resume_id = ?)`, []any{versionID, resumeID}},
		{`DELETE FROM analysis_jobs WHERE resume_version_id = ? OR resume_version_id IN (SELECT id FROM resume_versions WHERE legacy_resume_id = ?)`, []any{versionID, resumeID}},
		{`DELETE FROM project_candidate_resumes WHERE resume_version_id = ? OR resume_version_id IN (SELECT id FROM resume_versions WHERE legacy_resume_id = ?)`, []any{versionID, resumeID}},
		{`DELETE FROM resume_versions WHERE legacy_resume_id = ? OR id = ?`, []any{resumeID, versionID}},
	}
	for _, item := range execs {
		if _, err := tx.Exec(item.query, item.args...); err != nil {
			fmt.Fprintf(os.Stderr, "[RecruitmentStore] delete resume query failed (%s): %v\n", item.query, err)
			return err
		}
	}
	return tx.Commit()
}

func (a *App) syncRecruitmentResume(resume *Resume) {
	db := a.activeRecruitmentDB()
	if db == nil || resume == nil {
		return
	}
	if resume.ProjectID != "" {
		if proj := a.GetProject(resume.ProjectID); proj != nil {
			a.syncRecruitmentProject(proj)
		}
	}
	raw, err := json.Marshal(resume)
	if err != nil {
		return
	}
	tx, err := db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	versionID := "legacy_resume_" + resume.ID
	var existingVersionID string
	if err := tx.QueryRow(`SELECT id FROM resume_versions WHERE legacy_resume_id=? LIMIT 1`, resume.ID).Scan(&existingVersionID); err == nil && existingVersionID != "" {
		versionID = existingVersionID
	}
	stamp := resume.CreatedAt.Format(time.RFC3339Nano)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	candidateID, sourceID, identityErr := upsertRecruitmentCandidate(tx, resume, stamp, now)
	if identityErr != nil {
		fmt.Fprintf(os.Stderr, "[RecruitmentStore] candidate sync failed: %v\n", identityErr)
		return
	}
	hash := sha256.Sum256([]byte(resume.Content))
	if _, err = tx.Exec(`INSERT INTO resume_versions
		(id,candidate_id,source_id,legacy_resume_id,file_path,file_name,content,content_sha256,raw_json,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		candidate_id=excluded.candidate_id,source_id=excluded.source_id,legacy_resume_id=excluded.legacy_resume_id,
		file_path=excluded.file_path,file_name=excluded.file_name,content=excluded.content,
		content_sha256=excluded.content_sha256,raw_json=excluded.raw_json`,
		versionID, candidateID, sourceID, resume.ID, resume.FilePath, resume.FileName,
		resume.Content, hex.EncodeToString(hash[:]), string(raw), stamp); err != nil {
		fmt.Fprintf(os.Stderr, "[RecruitmentStore] resume_versions sync failed: %v\n", err)
		return
	}
	if resume.ProjectID != "" {
		_, _ = tx.Exec(`INSERT OR IGNORE INTO projects(id,name,legacy_json,created_at,updated_at) VALUES(?,?,'{}',?,?)`,
			resume.ProjectID, resume.ProjectID, stamp, now)
		linkID := "legacy_link_" + resume.ProjectID + "_" + resume.ID
		if _, err = tx.Exec(`INSERT OR IGNORE INTO project_candidates
			(id,project_id,candidate_id,stage,created_at,updated_at)
			VALUES(?,?,?,?,?,?)`, linkID, resume.ProjectID, candidateID, "new", stamp, now); err != nil {
			fmt.Fprintf(os.Stderr, "[RecruitmentStore] project_candidates sync failed: %v\n", err)
			return
		}
		if err = tx.QueryRow(`SELECT id FROM project_candidates WHERE project_id=? AND candidate_id=?`, resume.ProjectID, candidateID).Scan(&linkID); err != nil {
			return
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO project_candidate_resumes(project_candidate_id,resume_version_id,created_at) VALUES(?,?,?)`, linkID, versionID, stamp); err != nil {
			return
		}
		if resume.Analysis != nil {
			ruleVersion := resume.AnalysisRuleVersion
			analysis, encodeErr := json.Marshal(resume.Analysis)
			if encodeErr != nil {
				return
			}
			evalID := "legacy_eval_" + resume.ProjectID + "_" + resume.ID
			if ruleVersion > 0 {
				evalID = fmt.Sprintf("%s_v%d", evalID, ruleVersion)
			}
			_, err = tx.Exec(`INSERT INTO evaluations
				(id,project_candidate_id,resume_version_id,rule_version,status,ability_score,redline_status,
				 confirmed_bonus_count,analysis_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)
				 ON CONFLICT(id) DO UPDATE SET status=excluded.status,
				 ability_score=excluded.ability_score,redline_status=excluded.redline_status,
				 confirmed_bonus_count=excluded.confirmed_bonus_count,analysis_json=excluded.analysis_json`,
				evalID, linkID, versionID, ruleVersion,
				resume.Status, resume.Analysis.AbilityScore, resume.Analysis.RedLineStatus,
				resume.Analysis.ConfirmedBonusCount, string(analysis), now)
			if err != nil {
				return
			}
		}
	}
	if err = tx.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "[RecruitmentStore] resume sync failed: %v\n", err)
	}
}

func migrationDone(db *sql.DB) (bool, error) {
	var value string
	err := db.QueryRow("SELECT value FROM migration_meta WHERE key = 'legacy_json_v1'").Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return value == "complete", err
}

func migrateLegacyRecruitmentData(db *sql.DB, dir string) error {
	done, err := migrationDone(db)
	if err != nil || done {
		return err
	}
	projectFiles, err := filepath.Glob(filepath.Join(dir, "projects", "*.json"))
	if err != nil {
		return err
	}
	resumeFiles, err := filepath.Glob(filepath.Join(dir, "resumes", "*.json"))
	if err != nil {
		return err
	}
	sort.Strings(projectFiles)
	sort.Strings(resumeFiles)
	if len(projectFiles)+len(resumeFiles) > 0 {
		if err := backupLegacyFiles(dir, projectFiles, resumeFiles); err != nil {
			return err
		}
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	projects := make(map[string]*Project)
	for _, path := range projectFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var project Project
		if err := json.Unmarshal(data, &project); err != nil {
			return fmt.Errorf("读取旧项目 %s: %w", path, err)
		}
		if project.ID == "" {
			return fmt.Errorf("旧项目 %s 缺少 ID", path)
		}
		projects[project.ID] = &project
		if _, err := tx.Exec(`INSERT INTO projects
			(id,name,department,recruiter,headcount,job_title,legacy_json,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?)`, project.ID, project.Name, project.Department,
			project.Recruiter, project.Headcount, project.JobConfig.Title, string(data),
			project.CreatedAt.Format(time.RFC3339Nano), project.UpdatedAt.Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("导入旧项目 %s: %w", project.ID, err)
		}
		for _, group := range []struct {
			kind  string
			items []string
		}{{"redline", project.JobConfig.RedLines}, {"core", project.JobConfig.CoreCriteria}, {"bonus", project.JobConfig.BonusPoints}} {
			for _, criterion := range group.items {
				if strings.TrimSpace(criterion) == "" {
					continue
				}
				if _, err := tx.Exec("INSERT INTO job_rules(id,project_id,kind,criterion,source,status,version,created_at) VALUES(?,?,?,?,?,'source_unconfirmed',0,?)",
					recruitmentID("rule"), project.ID, group.kind, criterion, "legacy", project.CreatedAt.Format(time.RFC3339Nano)); err != nil {
					return err
				}
			}
		}
	}
	resumes := make(map[string]*Resume)
	for _, path := range resumeFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var resume Resume
		if err := json.Unmarshal(data, &resume); err != nil {
			return fmt.Errorf("读取旧简历 %s: %w", path, err)
		}
		if resume.ID == "" {
			return fmt.Errorf("旧简历 %s 缺少 ID", path)
		}
		resumes[resume.ID] = &resume
		if err := importLegacyResume(tx, &resume, string(data)); err != nil {
			return fmt.Errorf("导入旧简历 %s: %w", resume.ID, err)
		}
	}
	for _, project := range projects {
		for _, resumeID := range project.ResumeIDs {
			resume := resumes[resumeID]
			if resume == nil {
				// 旧项目引用的历史简历在磁盘中已被清理或不存在，跳过该引用以保证数据库正常初始化
				continue
			}
			if err := linkLegacyProjectResume(tx, project.ID, resume); err != nil {
				return err
			}
		}
	}
	for _, resume := range resumes {
		if resume.ProjectID != "" && projects[resume.ProjectID] != nil {
			if err := linkLegacyProjectResume(tx, resume.ProjectID, resume); err != nil {
				return err
			}
		}
	}
	var importedProjects, importedResumes int
	if err := tx.QueryRow("SELECT COUNT(*) FROM projects").Scan(&importedProjects); err != nil {
		return err
	}
	if err := tx.QueryRow("SELECT COUNT(*) FROM resume_versions").Scan(&importedResumes); err != nil {
		return err
	}
	if importedProjects != len(projectFiles) || importedResumes != len(resumeFiles) {
		return fmt.Errorf("旧数据校验失败：项目 %d/%d，简历 %d/%d", importedProjects, len(projectFiles), importedResumes, len(resumeFiles))
	}
	if _, err := tx.Exec("INSERT INTO migration_meta(key,value) VALUES('legacy_json_v1','complete')"); err != nil {
		return err
	}
	return tx.Commit()
}

func importLegacyResume(tx *sql.Tx, resume *Resume, raw string) error {
	versionID := "legacy_resume_" + resume.ID
	stamp := resume.CreatedAt.Format(time.RFC3339Nano)
	candidateID, sourceID, err := upsertRecruitmentCandidate(tx, resume, stamp, stamp)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(resume.Content))
	_, err = tx.Exec(`INSERT INTO resume_versions
		(id,candidate_id,source_id,legacy_resume_id,file_path,file_name,content,content_sha256,raw_json,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, versionID, candidateID, sourceID, resume.ID,
		resume.FilePath, resume.FileName, resume.Content, hex.EncodeToString(hash[:]), raw, stamp)
	return err
}

func linkLegacyProjectResume(tx *sql.Tx, projectID string, resume *Resume) error {
	linkID := "legacy_link_" + projectID + "_" + resume.ID
	stamp := resume.CreatedAt.Format(time.RFC3339Nano)
	var candidateID string
	if err := tx.QueryRow(`SELECT candidate_id FROM resume_versions WHERE legacy_resume_id=?`, resume.ID).Scan(&candidateID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO project_candidates
		(id,project_id,candidate_id,stage,created_at,updated_at)
		VALUES(?,?,?,?,?,?)`, linkID, projectID, candidateID, "new", stamp, stamp); err != nil {
		return err
	}
	if err := tx.QueryRow(`SELECT id FROM project_candidates WHERE project_id=? AND candidate_id=?`, projectID, candidateID).Scan(&linkID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO project_candidate_resumes(project_candidate_id,resume_version_id,created_at) VALUES(?,?,?)`, linkID, "legacy_resume_"+resume.ID, stamp); err != nil {
		return err
	}
	if resume.Analysis == nil {
		return nil
	}
	data, err := json.Marshal(resume.Analysis)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT OR IGNORE INTO evaluations
		(id,project_candidate_id,resume_version_id,status,ability_score,redline_status,
		 confirmed_bonus_count,analysis_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		"legacy_eval_"+projectID+"_"+resume.ID, linkID, "legacy_resume_"+resume.ID,
		resume.Status, resume.Analysis.AbilityScore, resume.Analysis.RedLineStatus,
		resume.Analysis.ConfirmedBonusCount, string(data), stamp)
	return err
}

func backupLegacyFiles(dir string, projectFiles, resumeFiles []string) error {
	backupRoot := filepath.Join(dir, "legacy-backup-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	groups := map[string][]string{"projects": projectFiles, "resumes": resumeFiles}
	configPath := filepath.Join(dir, "config.json")
	if _, err := os.Stat(configPath); err == nil {
		groups["."] = []string{configPath}
	} else if !os.IsNotExist(err) {
		return err
	}
	for group, paths := range groups {
		if len(paths) == 0 {
			continue
		}
		targetDir := filepath.Join(backupRoot, group)
		if err := os.MkdirAll(targetDir, 0700); err != nil {
			return err
		}
		for _, source := range paths {
			in, err := os.Open(source)
			if err != nil {
				return err
			}
			target := filepath.Join(targetDir, filepath.Base(source))
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				in.Close()
				return err
			}
			_, copyErr := io.Copy(out, in)
			closeErr := out.Close()
			in.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return nil
}

// GetRecruitmentMigrationStatus allows the new workbench to surface a failed
// import without preventing the old JSON-backed screens from opening.
func (a *App) GetRecruitmentMigrationStatus() map[string]interface{} {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return map[string]interface{}{"ready": false, "error": err.Error()}
	}
	var projects, candidates int
	if err = db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&projects); err == nil {
		err = db.QueryRow("SELECT COUNT(*) FROM candidates").Scan(&candidates)
	}
	if err != nil {
		return map[string]interface{}{"ready": false, "error": err.Error()}
	}
	return map[string]interface{}{"ready": true, "projects": projects, "candidates": candidates}
}
