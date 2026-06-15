package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"time"

	"github.com/lingweicai/cockpit-slurm/bridge/internal/models"
)

type Event struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

type SinfoUpdate struct {
	Added   []models.SinfoPartitionRow `json:"added,omitempty"`
	Removed []models.SinfoPartitionRow `json:"removed,omitempty"`
	Updated []models.SinfoPartitionRow `json:"updated,omitempty"`
	Full    []models.SinfoPartitionRow `json:"full,omitempty"`
}

type SinfoService struct {
	interval time.Duration
	cache    *models.SinfoCache
	eventCh  chan Event
}

func NewSinfoService(interval time.Duration) *SinfoService {
	return &SinfoService{
		interval: interval,
		cache:    &models.SinfoCache{},
		eventCh:  make(chan Event, 8),
	}
}

func (s *SinfoService) EventChannel() <-chan Event {
	return s.eventCh
}

func (s *SinfoService) Cache() *models.SinfoCache {
	return s.cache
}

func (s *SinfoService) Run(ctx context.Context) error {
	if err := s.fetchOnce(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.fetchOnce(ctx); err != nil {
				fmt.Printf("sinfo fetch failed: %v\n", err)
			}
		}
	}
}

func (s *SinfoService) fetchOnce(ctx context.Context) error {
	resp, err := s.fetchSinfo(ctx)
	if err != nil {
		return err
	}

	rows := s.rowsFromResponse(resp)
	oldRows, _ := s.cache.Get()
	if !reflect.DeepEqual(oldRows, rows) {
		update := buildSinfoUpdate(oldRows, rows)
		s.cache.Set(rows)
		s.publishEvent("sinfo.updated", update)
	}

	return nil
}

func (s *SinfoService) fetchSinfo(ctx context.Context) (*models.SinfoResponse, error) {
	cmd := exec.CommandContext(ctx, "sinfo", "--json")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("execute sinfo --json: %v: %s", err, bytes.TrimSpace(raw))
	}

	var resp models.SinfoResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse sinfo JSON: %w", err)
	}
	return &resp, nil
}

func (s *SinfoService) rowsFromResponse(resp *models.SinfoResponse) []models.SinfoPartitionRow {
	rows := make([]models.SinfoPartitionRow, 0, len(resp.Sinfo))
	for i := range resp.Sinfo {
		rows = append(rows, models.NewSinfoPartitionRow(&resp.Sinfo[i]))
	}
	return rows
}

func buildSinfoUpdate(oldRows, newRows []models.SinfoPartitionRow) SinfoUpdate {
	if len(oldRows) == 0 {
		return SinfoUpdate{Full: newRows}
	}

	oldMap := make(map[string]models.SinfoPartitionRow, len(oldRows))
	for _, row := range oldRows {
		oldMap[row.PartitionName] = row
	}

	newMap := make(map[string]models.SinfoPartitionRow, len(newRows))
	for _, row := range newRows {
		newMap[row.PartitionName] = row
	}

	update := SinfoUpdate{}

	for _, row := range newRows {
		existing, ok := oldMap[row.PartitionName]
		if !ok {
			update.Added = append(update.Added, row)
			continue
		}
		if !reflect.DeepEqual(existing, row) {
			update.Updated = append(update.Updated, row)
		}
	}

	for _, row := range oldRows {
		if _, ok := newMap[row.PartitionName]; !ok {
			update.Removed = append(update.Removed, row)
		}
	}

	return update
}

func (s *SinfoService) publishEvent(eventType string, data interface{}) {
	select {
	case s.eventCh <- Event{Type: eventType, Timestamp: time.Now().UTC(), Data: data}:
	default:
		// If the event channel is full, drop the event to avoid blocking the poll loop.
	}
}
