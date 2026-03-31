package context

import (
	"sync"
	"time"
)

const (
	maxEventsPerType = 1000
	maxEventsPerUE   = 100
)

// amfRingBuffer is a fixed-size ring buffer for AMF events.
type amfRingBuffer struct {
	buf   [maxEventsPerType]ReceivedAmfEvent
	head  int
	count int
}

func (r *amfRingBuffer) add(e ReceivedAmfEvent) {
	r.buf[r.head%maxEventsPerType] = e
	r.head++
	if r.count < maxEventsPerType {
		r.count++
	}
}

func (r *amfRingBuffer) all() []ReceivedAmfEvent {
	out := make([]ReceivedAmfEvent, r.count)
	start := 0
	if r.count == maxEventsPerType {
		start = r.head % maxEventsPerType
	}
	for i := 0; i < r.count; i++ {
		out[i] = r.buf[(start+i)%maxEventsPerType]
	}
	return out
}

// smfRingBuffer is a fixed-size ring buffer for SMF events.
type smfRingBuffer struct {
	buf   [maxEventsPerType]ReceivedSmfEvent
	head  int
	count int
}

func (r *smfRingBuffer) add(e ReceivedSmfEvent) {
	r.buf[r.head%maxEventsPerType] = e
	r.head++
	if r.count < maxEventsPerType {
		r.count++
	}
}

func (r *smfRingBuffer) all() []ReceivedSmfEvent {
	out := make([]ReceivedSmfEvent, r.count)
	start := 0
	if r.count == maxEventsPerType {
		start = r.head % maxEventsPerType
	}
	for i := 0; i < r.count; i++ {
		out[i] = r.buf[(start+i)%maxEventsPerType]
	}
	return out
}

// ueEventEntry is a union type used for per-UE history.
type ueEventEntry struct {
	AmfEvent *ReceivedAmfEvent
	SmfEvent *ReceivedSmfEvent
}

// EventStore stores event history using ring buffers per event type and per-UE slices.
type EventStore struct {
	mu sync.RWMutex

	// Ring buffer per AMF event type
	amfByType map[string]*amfRingBuffer

	// Ring buffer per SMF event type
	smfByType map[string]*smfRingBuffer

	// Per-UE event history (last maxEventsPerUE entries)
	ueHistory map[string][]ueEventEntry

	// Total counters
	totalAmf int
	totalSmf int
}

// NewEventStore creates and returns a new, empty EventStore.
func NewEventStore() *EventStore {
	return &EventStore{
		amfByType: make(map[string]*amfRingBuffer),
		smfByType: make(map[string]*smfRingBuffer),
		ueHistory: make(map[string][]ueEventEntry),
	}
}

// AddAmfEvent stores an AMF event in the ring buffer for its type and, if a SUPI
// is present, in the per-UE history as well.
func (s *EventStore) AddAmfEvent(event ReceivedAmfEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rb, ok := s.amfByType[event.EventType]
	if !ok {
		rb = &amfRingBuffer{}
		s.amfByType[event.EventType] = rb
	}
	rb.add(event)
	s.totalAmf++

	if event.Supi != "" {
		entry := ueEventEntry{AmfEvent: &event}
		s.appendUEEntry(event.Supi, entry)
	}
}

// AddSmfEvent stores an SMF event in the ring buffer for its type and, if a SUPI
// is present, in the per-UE history as well.
func (s *EventStore) AddSmfEvent(event ReceivedSmfEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rb, ok := s.smfByType[event.EventType]
	if !ok {
		rb = &smfRingBuffer{}
		s.smfByType[event.EventType] = rb
	}
	rb.add(event)
	s.totalSmf++

	if event.Supi != "" {
		entry := ueEventEntry{SmfEvent: &event}
		s.appendUEEntry(event.Supi, entry)
	}
}

// appendUEEntry appends an entry to the per-UE history, capping at maxEventsPerUE.
// Caller must hold s.mu.
func (s *EventStore) appendUEEntry(supi string, entry ueEventEntry) {
	hist := s.ueHistory[supi]
	hist = append(hist, entry)
	if len(hist) > maxEventsPerUE {
		hist = hist[len(hist)-maxEventsPerUE:]
	}
	s.ueHistory[supi] = hist
}

// GetAmfEventsByType returns all stored AMF events for the given event type.
func (s *EventStore) GetAmfEventsByType(eventType string) []ReceivedAmfEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rb, ok := s.amfByType[eventType]
	if !ok {
		return nil
	}
	return rb.all()
}

// GetSmfEventsByType returns all stored SMF events for the given event type.
func (s *EventStore) GetSmfEventsByType(eventType string) []ReceivedSmfEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rb, ok := s.smfByType[eventType]
	if !ok {
		return nil
	}
	return rb.all()
}

// GetAmfEventsForSupi returns the AMF events in the per-UE history for the given SUPI.
func (s *EventStore) GetAmfEventsForSupi(supi string) []ReceivedAmfEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []ReceivedAmfEvent
	for _, entry := range s.ueHistory[supi] {
		if entry.AmfEvent != nil {
			out = append(out, *entry.AmfEvent)
		}
	}
	return out
}

// GetSmfEventsForSupi returns the SMF events in the per-UE history for the given SUPI.
func (s *EventStore) GetSmfEventsForSupi(supi string) []ReceivedSmfEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []ReceivedSmfEvent
	for _, entry := range s.ueHistory[supi] {
		if entry.SmfEvent != nil {
			out = append(out, *entry.SmfEvent)
		}
	}
	return out
}

// GetEventRate returns the number of AMF or SMF events of the given type that were
// received within the given time window, expressed as events per second.
// eventType must match an AmfEventType or SmfEvent string value.
func (s *EventStore) GetEventRate(eventType string, window time.Duration) float64 {
	if window <= 0 {
		return 0
	}

	cutoff := time.Now().Add(-window)
	count := 0

	s.mu.RLock()
	defer s.mu.RUnlock()

	if rb, ok := s.amfByType[eventType]; ok {
		for _, ev := range rb.all() {
			if ev.Timestamp.After(cutoff) {
				count++
			}
		}
	}

	if rb, ok := s.smfByType[eventType]; ok {
		for _, ev := range rb.all() {
			if ev.Timestamp.After(cutoff) {
				count++
			}
		}
	}

	return float64(count) / window.Seconds()
}

// GetUniqueSupis returns a deduplicated list of all SUPIs that have per-UE history.
func (s *EventStore) GetUniqueSupis() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	supis := make([]string, 0, len(s.ueHistory))
	for supi := range s.ueHistory {
		supis = append(supis, supi)
	}
	return supis
}

// GetTotalEventCount returns the total number of AMF + SMF events ever stored.
func (s *EventStore) GetTotalEventCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalAmf + s.totalSmf
}

// GetEventCount returns the total number of events stored for the given event type
// across both AMF and SMF ring buffers.
func (s *EventStore) GetEventCount(eventType string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	if rb, ok := s.amfByType[eventType]; ok {
		count += rb.count
	}
	if rb, ok := s.smfByType[eventType]; ok {
		count += rb.count
	}
	return count
}
