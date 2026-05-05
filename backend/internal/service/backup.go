package service

import (
	"nodeforge/internal/model"
	"nodeforge/internal/repository"
)

type BackupService struct {
	repo          *repository.Repo
	retentionCount int
}

func NewBackupService(repo *repository.Repo, retentionCount int) *BackupService {
	return &BackupService{repo: repo, retentionCount: retentionCount}
}

func (s *BackupService) Save(userID int64, backupType, content string) (*model.ConfigBackup, error) {
	b, err := s.repo.CreateBackup(userID, backupType, content)
	if err != nil {
		return nil, err
	}
	s.repo.DeleteOldBackups(userID, s.retentionCount)
	return b, nil
}

func (s *BackupService) List(userID int64) ([]model.ConfigBackup, error) {
	return s.repo.ListBackups(userID, s.retentionCount)
}

func (s *BackupService) Restore(id, userID int64) (string, error) {
	return s.repo.GetBackupContent(id, userID)
}
