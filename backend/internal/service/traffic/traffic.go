package traffic

import (
	"time"

	"nodeforge/internal/model"
	"nodeforge/internal/repository"
)

type Service struct{ repo *repository.Repo }

func New(repo *repository.Repo) *Service { return &Service{repo: repo} }

func (s *Service) Record(userID int64, nodeID *int64, bytesUp, bytesDown int64) (*model.TrafficLog, error) {
	return s.repo.CreateTrafficLog(userID, nodeID, bytesUp, bytesDown)
}

func (s *Service) Overview(userID int64) (struct {
	UsedBytes    int64 `json:"used_bytes"`
	QuotaBytes   int64 `json:"quota_bytes"`
	NodeTraffic  []model.TrafficLog `json:"node_traffic"`
}, error) {
	u, err := s.repo.GetUserByID(userID)
	if err != nil {
		return struct {
			UsedBytes   int64 `json:"used_bytes"`
			QuotaBytes  int64 `json:"quota_bytes"`
			NodeTraffic []model.TrafficLog `json:"node_traffic"`
		}{}, err
	}
	today := time.Now().Format("2006-01-02")
	logs, _ := s.repo.GetTrafficByUserDate(userID, today)
	return struct {
		UsedBytes   int64 `json:"used_bytes"`
		QuotaBytes  int64 `json:"quota_bytes"`
		NodeTraffic []model.TrafficLog `json:"node_traffic"`
	}{UsedBytes: u.UsedBytes, QuotaBytes: u.MonthlyQuotaBytes, NodeTraffic: logs}, nil
}

func (s *Service) AggregateAll() error {
	return s.repo.AggregateTraffic()
}

func (s *Service) ResetMonthly() error {
	return s.repo.ResetMonthlyTraffic()
}
