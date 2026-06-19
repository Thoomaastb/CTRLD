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
// Läuft bis der Context abgebrochen wird.
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(collectInterval)
	defer ticker.Stop()

	// Ersten Snapshot sofort sammeln
	s.collect(false)

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("metrics service beendet")
			return
		case <-ticker.C:
			s.collect(false)
		}
	}
}

// collect sammelt einen Snapshot und legt ihn in den Buffer.
func (s *Service) collect(includeProcesses bool) {
	snap, err := s.collector.Collect(includeProcesses)
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

// History gibt die gesamte History zurück (max. 60 Einträge).
func (s *Service) History() []*Snapshot {
	return s.buffer.History()
}

// CollectProcesses sammelt einmalig die Prozessliste (teurer Aufruf).
func (s *Service) CollectProcesses() ([]Process, error) {
	ram, err := collectRAM()
	if err != nil {
		return nil, err
	}
	return collectProcesses(ram.TotalBytes)
}
