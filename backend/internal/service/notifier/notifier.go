package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Service struct {
	webhookURL string
	client     *http.Client
}

func New(webhookURL string) *Service {
	return &Service{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Service) NotifyExpiring(user string, daysLeft int) {
	s.send(fmt.Sprintf("用户 %s 将于 %d 天后到期", user, daysLeft))
}

func (s *Service) NotifyTrafficThreshold(user string, pct int) {
	s.send(fmt.Sprintf("用户 %s 流量已使用 %d%%", user, pct))
}

func (s *Service) NotifySourceFailure(sourceURL, errMsg string) {
	s.send(fmt.Sprintf("订阅源拉取失败: %s — %s", sourceURL, errMsg))
}

func (s *Service) send(msg string) {
	if s.webhookURL == "" {
		log.Printf("[notifier] %s", msg)
		return
	}
	body, _ := json.Marshal(map[string]string{"text": msg})
	req, err := http.NewRequest("POST", s.webhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[notifier] create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[notifier] send: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("[notifier] sent: %s", msg)
}
