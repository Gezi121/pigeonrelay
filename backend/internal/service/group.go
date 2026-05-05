package service

import (
	"encoding/json"
	"fmt"

	"nodeforge/internal/model"
	"nodeforge/internal/repository"
	"nodeforge/internal/service/converter"
	"nodeforge/internal/service/generator"
)

type GroupService struct {
	repo *repository.Repo
}

func NewGroupService(repo *repository.Repo) *GroupService {
	return &GroupService{repo: repo}
}

func (s *GroupService) Create(userID int64, name, sourceType string, sourceID *int64) (*model.NodeGroup, error) {
	return s.repo.CreateGroup(userID, name, sourceType, sourceID)
}

func (s *GroupService) GetByID(id, userID int64) (*model.NodeGroup, error) {
	return s.repo.GetGroupByID(id, userID)
}

func (s *GroupService) List(userID int64) ([]model.NodeGroup, error) {
	return s.repo.ListGroups(userID)
}

func (s *GroupService) UpdateConfig(id, userID int64, namePrefix, groupType string, replacements []model.GroupReplacement, routings []model.GroupRouting) error {
	return s.repo.UpdateGroupConfig(id, userID, namePrefix, groupType, replacements, routings)
}

func (s *GroupService) GenerateConfig(id, userID int64, outputType string) (string, error) {
	g, err := s.repo.GetGroupByID(id, userID)
	if err != nil {
		return "", fmt.Errorf("group not found: %w", err)
	}

	var sources []string
	if g.SourceID != nil {
		src, err := s.repo.GetSourceByID(*g.SourceID, userID)
		if err != nil {
			return "", fmt.Errorf("source not found: %w", err)
		}
		sources = append(sources, src.URL)
	}

	req := &generator.GenerateRequest{
		Sources:    sources,
		OutputType: outputType,
	}
	if g.UnifiedConfig != "" {
		var uc converter.UnifiedConfig
		if err := json.Unmarshal([]byte(g.UnifiedConfig), &uc); err == nil {
			req.UnifiedConfig = &uc
		}
	}
	if g.AdvancedPrefer != "" {
		var pc converter.PreferConfig
		if err := json.Unmarshal([]byte(g.AdvancedPrefer), &pc); err == nil {
			req.PreferConfig = &pc
		}
	}

	return generator.Generate(req)
}
