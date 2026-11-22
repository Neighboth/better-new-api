package model

import (
	"strings"
)

// NotebookDeployment stores deployments launched on Google Colab or Kaggle.
type NotebookDeployment struct {
	Id                 int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Provider           string `json:"provider" gorm:"type:varchar(32);index;not null"` // colab | kaggle
	Name               string `json:"name" gorm:"type:varchar(128);not null"`
	ModelRepo          string `json:"model_repo" gorm:"type:varchar(256);not null"`
	ModelFile          string `json:"model_file" gorm:"type:varchar(256)"`
	TaskType             string `json:"task_type" gorm:"type:varchar(32);not null"` // text | image | video | audio | embeddings
	Accelerator          string `json:"accelerator" gorm:"type:varchar(64);not null"`
	FallbackAccelerators string `json:"fallback_accelerators" gorm:"type:text"` // JSON array of fallback accelerators
	IdleTimeoutSeconds   int    `json:"idle_timeout_seconds" gorm:"default:300"`
	Status             string `json:"status" gorm:"type:varchar(32);not null"`
	RemoteID           string `json:"remote_id" gorm:"type:varchar(256)"`
	RemoteURL          string `json:"remote_url" gorm:"type:text"`
	EndpointURL        string `json:"endpoint_url" gorm:"type:text"`
	Script             string `json:"script" gorm:"type:text"`
	ErrorMessage       string `json:"error_message" gorm:"type:text"`
	CreatedTime        int64  `json:"created_time" gorm:"bigint;index"`
	UpdatedTime        int64  `json:"updated_time" gorm:"bigint"`
}

func ListNotebookDeployments(provider string) ([]NotebookDeployment, error) {
	var items []NotebookDeployment
	q := DB.Order("id desc")
	if p := strings.TrimSpace(strings.ToLower(provider)); p != "" {
		q = q.Where("provider = ?", p)
	}
	err := q.Find(&items).Error
	return items, err
}

func GetNotebookDeploymentByID(id int) (*NotebookDeployment, error) {
	var item NotebookDeployment
	if err := DB.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func CreateNotebookDeployment(item *NotebookDeployment) error {
	return DB.Create(item).Error
}

func UpdateNotebookDeployment(item *NotebookDeployment) error {
	return DB.Save(item).Error
}

func DeleteNotebookDeployment(id int) error {
	return DB.Delete(&NotebookDeployment{}, id).Error
}
