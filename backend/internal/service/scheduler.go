package service

import (
	"fmt"
	"log"
	"time"

	"nodeforge/internal/repository"
	"nodeforge/internal/service/cfdns"
	"nodeforge/internal/service/notifier"
	"nodeforge/internal/service/subfetcher"
)

type Scheduler struct {
	repo     *repository.Repo
	notifier *notifier.Service
	cfdns    *cfdns.Service
}

func NewScheduler(repo *repository.Repo, notifier *notifier.Service, cfdnsSvc *cfdns.Service) *Scheduler {
	return &Scheduler{repo: repo, notifier: notifier, cfdns: cfdnsSvc}
}

func (s *Scheduler) Start() {
	go s.loopEvery(5*time.Minute, s.aggregateTraffic)
	go s.loopEvery(1*time.Hour, s.updatePeakHourLatency)
	go s.cfdnsLoop()
	go s.loopEvery(1*time.Hour, s.fetchAllSources)
	go s.loopEvery(24*time.Hour, s.dailyCheck)
	log.Println("[scheduler] started: traffic/5m, peak-hour/1h, cfdns/dynamic, fetch/1h, daily/24h")
}

func (s *Scheduler) loopEvery(interval time.Duration, fn func()) {
	fn()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		fn()
	}
}

func (s *Scheduler) updatePeakHourLatency() {
	if err := s.repo.UpdatePeakHourLatencyBatch(); err != nil {
		log.Printf("[scheduler] peak-hour latency: %v", err)
	}
	if err := s.repo.RefreshSamplesCounts(); err != nil {
		log.Printf("[scheduler] refresh sample counts: %v", err)
	}
}

func (s *Scheduler) aggregateTraffic() {
	if err := s.repo.AggregateTraffic(); err != nil {
		log.Printf("[scheduler] aggregate traffic: %v", err)
	}
}

func (s *Scheduler) fetchAllSources() {
	users, err := s.repo.ListUsers()
	if err != nil {
		log.Printf("[scheduler] list users: %v", err)
		return
	}
	for _, u := range users {
		sources, err := s.repo.ListSources(u.ID)
		if err != nil {
			continue
		}
		for _, src := range sources {
			if src.URL == "" {
				continue
			}
			result, err := subfetcher.Fetch(src.URL)
			status := "ok"
			if err != nil {
				status = "error: " + err.Error()
				s.notifier.NotifySourceFailure(src.URL, err.Error())
			}
			if err := s.repo.UpdateSourceFetchStatus(src.ID, status, ""); err != nil {
				log.Printf("[scheduler] update source %d status: %v", src.ID, err)
			}
			if result != nil {
				_ = result.NodeCount
			}
		}
	}
}

func (s *Scheduler) cfdnsLoop() {
	// First run immediately, then loop with configurable interval
	s.updateCFDNS()
	for {
		interval := s.cfInterval()
		time.Sleep(interval)
		s.updateCFDNS()
	}
}

func (s *Scheduler) cfInterval() time.Duration {
	if v, err := s.repo.GetSetting("cf_update_minutes"); err == nil && v != "" {
		var mins int
		if _, e := fmt.Sscanf(v, "%d", &mins); e == nil && mins >= 5 {
			return time.Duration(mins) * time.Minute
		}
	}
	return 30 * time.Minute
}

func (s *Scheduler) updateCFDNS() {
	if s.cfdns != nil {
		s.cfdns.UpdateBestIPs()
	}
}

func (s *Scheduler) dailyCheck() {
	// Update per-client quality bias
	s.repo.UpdateAllClientBias()

	// Cleanup old latency records (keep 7 days)
	if n, err := s.repo.CleanupLatencyRecords(7); err == nil && n > 0 {
		log.Printf("[scheduler] cleaned %d old latency records", n)
	}

	users, err := s.repo.ListUsers()
	if err != nil {
		log.Printf("[scheduler] daily list: %v", err)
		return
	}
	now := time.Now()
	for _, u := range users {
		if !u.ExpireAt.IsZero() {
			daysLeft := int(u.ExpireAt.Sub(now).Hours() / 24)
			if daysLeft <= 7 && daysLeft >= 0 {
				s.notifier.NotifyExpiring(u.Username, daysLeft)
			}
		}
		if u.MonthlyQuotaBytes > 0 {
			pct := int(float64(u.UsedBytes) / float64(u.MonthlyQuotaBytes) * 100)
			if pct >= 80 {
				s.notifier.NotifyTrafficThreshold(u.Username, pct)
			}
		}
	}
	if err := s.repo.ResetMonthlyTraffic(); err != nil {
		log.Printf("[scheduler] reset monthly: %v", err)
	}
}
