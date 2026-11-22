package model

import (
	"strings"
)

// NotebookDeployment stores deployments launched on Google Colab, Kaggle, Modal, etc.
type NotebookDeployment struct {
	Id                   int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Provider             string `json:"provider" gorm:"type:varchar(32);index;not null"` // colab | kaggle | modal | huggingface | ...
	AccountID            string `json:"account_id" gorm:"type:varchar(64)"`
	AccountLabel         string `json:"account_label" gorm:"type:varchar(128)"`
	Name                 string `json:"name" gorm:"type:varchar(128);not null"`
	ModelRepo            string `json:"model_repo" gorm:"type:varchar(256);not null"`
	ModelFile            string `json:"model_file" gorm:"type:varchar(256)"`
	ModelFiles           string `json:"model_files" gorm:"type:text"` // JSON array of {repo, file, role}
	TaskType             string `json:"task_type" gorm:"type:varchar(32);not null"` // text | image | video | audio | embeddings
	Accelerator          string `json:"accelerator" gorm:"type:varchar(64);not null"`
	FallbackAccelerators string `json:"fallback_accelerators" gorm:"type:text"` // JSON array of fallback accelerators
	Group                string `json:"group" gorm:"type:varchar(256);column:group_name"`
	Priority             int64  `json:"priority" gorm:"default:0"`
	Weight               uint   `json:"weight" gorm:"default:1"`
	Enabled              int    `json:"enabled" gorm:"default:1"` // 1 = enabled, 2 = disabled
	IdleTimeoutSeconds   int    `json:"idle_timeout_seconds" gorm:"default:300"`
	Status               string `json:"status" gorm:"type:varchar(32);not null"`
	StatusMessage        string `json:"status_message" gorm:"type:text"`
	RemoteID             string `json:"remote_id" gorm:"type:varchar(256)"`
	RemoteURL            string `json:"remote_url" gorm:"type:text"`
	ProxyToken           string `json:"-" gorm:"type:text"`
	EndpointURL          string `json:"endpoint_url" gorm:"type:text"`
	CallbackToken        string `json:"-" gorm:"type:varchar(64)"`
	Script               string `json:"script" gorm:"type:text"`
	Logs                 string `json:"logs" gorm:"type:text"`
	ErrorMessage         string `json:"error_message" gorm:"type:text"`
	CreatedTime          int64  `json:"created_time" gorm:"bigint;index"`
	UpdatedTime          int64  `json:"updated_time" gorm:"bigint"`
}

func normalizeNotebookDeployment(d *NotebookDeployment) {
	if d == nil {
		return
	}
	if d.Enabled == 0 {
		d.Enabled = 1
	}
	if strings.TrimSpace(d.Group) == "" {
		d.Group = "default"
	}
	if d.Weight == 0 {
		d.Weight = 1
	}
}

func ListNotebookDeployments(provider string) ([]NotebookDeployment, error) {
	var items []NotebookDeployment
	q := DB.Order("priority desc, id desc")
	if p := strings.TrimSpace(strings.ToLower(provider)); p != "" {
		q = q.Where("provider = ?", p)
	}
	err := q.Find(&items).Error
	for i := range items {
		normalizeNotebookDeployment(&items[i])
	}
	return items, err
}

func GetNotebookDeploymentByID(id int) (*NotebookDeployment, error) {
	var item NotebookDeployment
	if err := DB.First(&item, id).Error; err != nil {
		return nil, err
	}
	normalizeNotebookDeployment(&item)
	return &item, nil
}

func CreateNotebookDeployment(item *NotebookDeployment) error {
	normalizeNotebookDeployment(item)
	return DB.Create(item).Error
}

func UpdateNotebookDeployment(item *NotebookDeployment) error {
	return DB.Save(item).Error
}

func UpdateNotebookDeploymentFields(id int, fields map[string]any) error {
	return DB.Model(&NotebookDeployment{}).Where("id = ?", id).Updates(fields).Error
}

func DeleteNotebookDeployment(id int) error {
	return DB.Delete(&NotebookDeployment{}, id).Error
}
