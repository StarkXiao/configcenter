package event

import (
	"strconv"
	"sync"
	"time"
)

type Event struct {
	ID          string    `json:"id"`
	Application string    `json:"application"`
	Environment string    `json:"environment"`
	Version     int64     `json:"version"`
	Checksum    string    `json:"checksum"`
	Operation   string    `json:"operation"`
	CreatedAt   time.Time `json:"created_at"`
}

type Subscription struct {
	Events <-chan Event
	close  func()
	once   sync.Once
}

func (s *Subscription) Close() { s.once.Do(s.close) }

type subscriber struct {
	id      uint64
	channel chan Event
}

type Hub struct {
	mu          sync.RWMutex
	groups      map[string]map[uint64]*subscriber
	history     []Event
	historySize int
	nextID      uint64
	eventID     uint64
	closed      bool
}

func NewHub(historySize int) *Hub {
	return &Hub{
		groups:      make(map[string]map[uint64]*subscriber),
		history:     make([]Event, 0, historySize),
		historySize: historySize,
	}
}

func Key(application, environment string) string {
	return application + "\x00" + environment
}

func (h *Hub) Subscribe(application, environment, lastEventID string) (*Subscription, []Event, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		channel := make(chan Event)
		close(channel)
		return &Subscription{Events: channel, close: func() {}}, nil, false
	}
	h.nextID++
	key := Key(application, environment)
	item := &subscriber{id: h.nextID, channel: make(chan Event, 16)}
	if h.groups[key] == nil {
		h.groups[key] = make(map[uint64]*subscriber)
	}
	h.groups[key][item.id] = item
	replay, complete := h.replayLocked(application, environment, lastEventID)
	return &Subscription{
		Events: item.channel,
		close:  func() { h.unsubscribe(key, item.id) },
	}, replay, complete
}

func (h *Hub) replayLocked(application, environment, lastEventID string) ([]Event, bool) {
	if lastEventID == "" {
		return nil, true
	}
	last, err := strconv.ParseUint(lastEventID, 10, 64)
	if err != nil {
		return nil, false
	}
	if len(h.history) > 0 {
		oldest, _ := strconv.ParseUint(h.history[0].ID, 10, 64)
		if last+1 < oldest {
			return nil, false
		}
	}
	items := make([]Event, 0)
	for _, item := range h.history {
		id, _ := strconv.ParseUint(item.ID, 10, 64)
		if id > last && item.Application == application && item.Environment == environment {
			items = append(items, item)
		}
	}
	return items, true
}

func (h *Hub) Publish(item Event) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return 0
	}
	h.eventID++
	item.ID = strconv.FormatUint(h.eventID, 10)
	item.CreatedAt = time.Now().UTC()
	h.history = append(h.history, item)
	if len(h.history) > h.historySize {
		copy(h.history, h.history[len(h.history)-h.historySize:])
		h.history = h.history[:h.historySize]
	}
	key := Key(item.Application, item.Environment)
	dropped := make([]uint64, 0)
	for id, target := range h.groups[key] {
		select {
		case target.channel <- item:
		default:
			dropped = append(dropped, id)
		}
	}
	for _, id := range dropped {
		close(h.groups[key][id].channel)
		delete(h.groups[key], id)
	}
	return len(dropped)
}

func (h *Hub) unsubscribe(key string, id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	group := h.groups[key]
	if item, ok := group[id]; ok {
		close(item.channel)
		delete(group, id)
	}
	if len(group) == 0 {
		delete(h.groups, key)
	}
}

func (h *Hub) Count(application, environment string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.groups[Key(application, environment)])
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, group := range h.groups {
		for _, item := range group {
			close(item.channel)
		}
	}
	h.groups = make(map[string]map[uint64]*subscriber)
}
