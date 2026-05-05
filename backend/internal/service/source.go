package service

import (
	"nodeforge/internal/model"
	"nodeforge/internal/repository"
)

type SourceService struct {
	repo *repository.Repo
}

func NewSourceService(repo *repository.Repo) *SourceService {
	return &SourceService{repo: repo}
}

func (s *SourceService) Create(userID int64, req *model.CreateSourceRequest) (*model.SubscriptionSource, error) {
	return s.repo.CreateSource(userID, req.Type, req.URL, req.Name, req.LocalNodes)
}

func (s *SourceService) GetByID(id, userID int64) (*model.SubscriptionSource, error) {
	return s.repo.GetSourceByID(id, userID)
}

func (s *SourceService) List(userID int64) ([]model.SubscriptionSource, error) {
	return s.repo.ListSources(userID)
}

func (s *SourceService) Update(id, userID int64, req *model.UpdateSourceRequest) (*model.SubscriptionSource, error) {
	return s.repo.UpdateSource(id, userID, req.Type, req.URL, req.Name, req.LocalNodes)
}

func (s *SourceService) Delete(id, userID int64) error {
	return s.repo.DeleteSource(id, userID)
}
