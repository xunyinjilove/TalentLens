package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CompanyTalentAttachment 公司人才库候选人附件档案
type CompanyTalentAttachment struct {
	AttachmentID string    `json:"attachment_id"`
	FileName     string    `json:"file_name"`
	FileType     string    `json:"file_type"` // pdf | docx | doc | txt | other
	FileSize     int64     `json:"file_size"`
	FileHash     string    `json:"file_hash"`
	LocalPath    string    `json:"local_path"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// CompanyTalentRecord 公司人才库候选人资料卡
type CompanyTalentRecord struct {
	ID                  string                    `json:"id"`
	Platform            string                    `json:"platform"` // 51job
	Scope               string                    `json:"scope"`    // enterprise or account scope
	PlatformCandidateID string                    `json:"platform_candidate_id"`
	Name                string                    `json:"name"`
	CurrentRole         string                    `json:"current_role"`
	CurrentCompany      string                    `json:"current_company"`
	DetailURL           string                    `json:"detail_url"`
	OriginalFolder      string                    `json:"original_folder,omitempty"`
	OriginalJob         string                    `json:"original_job,omitempty"`
	ProjectID           string                    `json:"project_id"` // 目标项目ID或"unassigned"
	Attachments         []CompanyTalentAttachment `json:"attachments"`
	ImportedAt          time.Time                 `json:"imported_at"`
	Status              string                    `json:"status"` // pending | complete | error
}

// CompanyTalentBatch 导入批次记录
type CompanyTalentBatch struct {
	BatchID      string                `json:"batch_id"`
	Scope        string                `json:"scope"`
	ProjectID    string                `json:"project_id"`
	TotalCount   int                   `json:"total_count"`
	SuccessCount int                   `json:"success_count"`
	Records      []CompanyTalentRecord `json:"records"`
	CreatedAt    time.Time             `json:"created_at"`
}

var talentStoreMutex sync.Mutex

func (a *App) getCompanyTalentDir(scope, candidateKey string) string {
	if scope == "" {
		scope = "default"
	}
	if candidateKey == "" {
		candidateKey = "anonymous"
	}
	return filepath.Join(a.getDataDir(), "company_talent", "51job", scope, candidateKey)
}

// SaveCompanyTalentAttachment 保存公司人才库附件原件，按字节 SHA-256 去重，零 AI 调用
func (a *App) SaveCompanyTalentAttachment(scope, candidateKey, fileName string, content []byte) (*CompanyTalentAttachment, error) {
	talentStoreMutex.Lock()
	defer talentStoreMutex.Unlock()

	if len(content) == 0 {
		return nil, fmt.Errorf("附件内容为空")
	}

	hash := sha256.Sum256(content)
	hashHex := hex.EncodeToString(hash[:])
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		ext = ".pdf"
	}

	dir := a.getCompanyTalentDir(scope, candidateKey)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建公司人才库附件目录失败: %w", err)
	}

	savedPath := filepath.Join(dir, hashHex+ext)
	// 若文件已存在，直接复用已有哈希文件
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		if err := os.WriteFile(savedPath, content, 0644); err != nil {
			return nil, fmt.Errorf("写入附件失败: %w", err)
		}
	}

	fileType := strings.TrimPrefix(ext, ".")
	if fileType == "" {
		fileType = "unknown"
	}

	return &CompanyTalentAttachment{
		AttachmentID: "att_" + hashHex[:12],
		FileName:     fileName,
		FileType:     fileType,
		FileSize:     int64(len(content)),
		FileHash:     hashHex,
		LocalPath:    savedPath,
		DownloadedAt: time.Now(),
	}, nil
}

// IngestCompanyTalentBatch 批量导入公司人才库记录，不隐式调用 AI
func (a *App) IngestCompanyTalentBatch(batch *CompanyTalentBatch) (*CompanyTalentBatch, error) {
	talentStoreMutex.Lock()
	defer talentStoreMutex.Unlock()

	if batch == nil {
		return nil, fmt.Errorf("批次为空")
	}

	batchDir := filepath.Join(a.getDataDir(), "company_talent", "batches")
	_ = os.MkdirAll(batchDir, 0755)

	if batch.BatchID == "" {
		batch.BatchID = fmt.Sprintf("batch_%d", time.Now().UnixNano())
	}
	batch.CreatedAt = time.Now()
	batch.TotalCount = len(batch.Records)
	batch.SuccessCount = 0

	recordsDir := filepath.Join(a.getDataDir(), "company_talent", "records")
	_ = os.MkdirAll(recordsDir, 0755)

	for i := range batch.Records {
		rec := &batch.Records[i]
		if rec.ID == "" {
			rec.ID = fmt.Sprintf("ct_%d_%d", time.Now().UnixNano(), i)
		}
		if rec.Scope == "" {
			rec.Scope = batch.Scope
		}
		if rec.ProjectID == "" {
			rec.ProjectID = batch.ProjectID
		}
		rec.ImportedAt = time.Now()
		rec.Status = "complete"

		data, err := json.MarshalIndent(rec, "", "  ")
		if err == nil {
			_ = os.WriteFile(filepath.Join(recordsDir, rec.ID+".json"), data, 0644)
			batch.SuccessCount++
		}
	}

	batchData, err := json.MarshalIndent(batch, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(batchDir, batch.BatchID+".json"), batchData, 0644)
	}

	return batch, nil
}

// ListCompanyTalentRecords 读取已导入的公司人才库记录
func (a *App) ListCompanyTalentRecords(scope string) ([]CompanyTalentRecord, error) {
	talentStoreMutex.Lock()
	defer talentStoreMutex.Unlock()

	recordsDir := filepath.Join(a.getDataDir(), "company_talent", "records")
	entries, err := os.ReadDir(recordsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []CompanyTalentRecord{}, nil
		}
		return nil, err
	}

	var results []CompanyTalentRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(recordsDir, entry.Name()))
		if err != nil {
			continue
		}
		var rec CompanyTalentRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		if scope == "" || rec.Scope == scope {
			results = append(results, rec)
		}
	}
	return results, nil
}
