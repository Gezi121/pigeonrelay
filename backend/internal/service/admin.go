package service

import (
	"time"

	"golang.org/x/crypto/bcrypt"

	"nodeforge/internal/model"
	"nodeforge/internal/repository"
)

type AdminService struct{ repo *repository.Repo }

func NewAdminService(repo *repository.Repo) *AdminService {
	return &AdminService{repo: repo}
}

func (s *AdminService) GetRepo() *repository.Repo { return s.repo }

func (s *AdminService) ListUsers() ([]model.User, error) { return s.repo.ListUsers() }

func (s *AdminService) UpdateUser(id int64, quota *int64, expireAt *string) error {
	if quota != nil {
		if err := s.repo.UpdateUserQuota(id, *quota); err != nil {
			return err
		}
	}
	if expireAt != nil {
		t, err := time.Parse(time.RFC3339, *expireAt)
		if err != nil {
			return err
		}
		if err := s.repo.UpdateUserExpire(id, t); err != nil {
			return err
		}
	}
	return nil
}

func (s *AdminService) DeleteUser(id int64) error { return s.repo.DeleteUser(id) }
func (s *AdminService) ResetTraffic(id int64) error { return s.repo.ResetUserTraffic(id) }

func (s *AdminService) GetSetting(key string) (string, error) { return s.repo.GetSetting(key) }
func (s *AdminService) SetSetting(key, value string) error   { return s.repo.SetSetting(key, value) }

func (s *AdminService) UpdateAdminAccount(username, passwordHash string) error {
	return s.repo.UpdateAdminAccount(username, passwordHash)
}

func (s *AdminService) ListSpeedClients() ([]model.SpeedTestClient, error) {
	return s.repo.ListSpeedClients()
}

func (s *AdminService) DeleteClientRecords(clientID string) (int64, error) {
	return s.repo.DeleteClientRecords(clientID)
}

func (s *AdminService) UpsertSpeedClient(clientID, name, notes string) error {
	return s.repo.UpsertSpeedClient(clientID, name, notes)
}

func (s *AdminService) CreateAlgorithm(name, algoType, clientFilter string) (*model.CustomAlgorithm, error) {
	return s.repo.CreateAlgorithm(name, algoType, clientFilter)
}

func (s *AdminService) ListAlgorithms() ([]model.CustomAlgorithm, error) {
	return s.repo.ListAlgorithms()
}

func (s *AdminService) UpdateAlgorithm(id int64, name, algoType, clientFilter *string) error {
	return s.repo.UpdateAlgorithm(id, name, algoType, clientFilter)
}

func (s *AdminService) DeleteAlgorithm(id int64) error {
	return s.repo.DeleteAlgorithm(id)
}

func (s *AdminService) UpdateAdminWithPassword(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdateAdminAccount(username, string(hash))
}
