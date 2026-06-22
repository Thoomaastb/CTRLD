package metrics

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

const collectInterval = time.Second

// Service sammelt periodisch Metriken und hält sie im Buffer.
type Service struct {
	collector *Collector
	buffer    *Buffer
	log       zerolog.Logger
}

// NewService erstellt einen neuen Metrics-Service.
func NewService(log zerolog.Logger) *Service {
	return &Service{
		collector: NewCollector(),
		buffer:    NewBuffer(),
		log:       log,
	}
}

// Start startet die periodische Metrik-Sammlung.
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(collectInterval)
	defer ticker.Stop()

	s.collect()

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("metrics service beendet")
			return
		case <-ticker.C:
			s.collect()
		}
	}
}

func (s *Service) collect() {
	snap, err := s.collector.Collect(false)
	if err != nil {
		s.log.Error().Err(err).Msg("metriken sammeln fehlgeschlagen")
		return
	}
	s.buffer.Push(snap)
}

// Latest gibt den neuesten Snapshot zurück.
func (s *Service) Latest() *Snapshot {
	return s.buffer.Latest()
}

// History gibt die gesamte History zurück.
func (s *Service) History() []*Snapshot {
	return s.buffer.History()
}

// CollectProcesses sammelt einmalig die Prozessliste.
func (s *Service) CollectProcesses() ([]Process, error) {
	return s.collector.CollectProcesses()
}

// CollectSystemInfo gibt die System-Inventarisierung zurück.
func (s *Service) CollectSystemInfo(ctx context.Context) (*SystemInfo, error) {
	return CollectSystemInfo(ctx)
}
