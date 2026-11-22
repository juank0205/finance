package ledger

import (
	"context"
	"slices"
	"sync"
	"time"

	"finance/internal/transaction"
)

// MemoryStore keeps everything in memory: for local development and tests. Data is lost on restart.
type MemoryStore struct {
	mu            sync.Mutex
	categories    []Category
	hasCategories bool
	accounts      []Account
	txs           map[string]transaction.Transaction
	dedup         map[string]string // dedup key -> transaction id
	rules         map[string]Rule
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		txs:   map[string]transaction.Transaction{},
		dedup: map[string]string{},
		rules: map[string]Rule{},
	}
}

func (m *MemoryStore) Categories(context.Context) ([]Category, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.categories), m.hasCategories, nil
}

func (m *MemoryStore) SaveCategories(_ context.Context, list []Category) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.categories, m.hasCategories = slices.Clone(list), true
	return nil
}

func (m *MemoryStore) Accounts(context.Context) ([]Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.accounts), nil
}

func (m *MemoryStore) SaveAccounts(_ context.Context, list []Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts = slices.Clone(list)
	return nil
}

func (m *MemoryStore) TransactionsBetween(_ context.Context, from, to time.Time) ([]transaction.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []transaction.Transaction
	for _, t := range m.txs {
		if !t.OccurredAt.Before(from) && t.OccurredAt.Before(to) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *MemoryStore) Transaction(_ context.Context, id string) (transaction.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.txs[id]
	if !ok {
		return t, ErrNotFound
	}
	return t, nil
}

func (m *MemoryStore) InsertTransaction(_ context.Context, t transaction.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.txs[t.ID]; ok {
		return ErrDuplicate
	}
	if _, ok := m.dedup[t.DedupKey]; ok {
		return ErrDuplicate
	}
	m.txs[t.ID] = t
	m.dedup[t.DedupKey] = t.ID
	return nil
}

func (m *MemoryStore) ReplaceTransaction(_ context.Context, t transaction.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.txs[t.ID]
	if !ok {
		return ErrNotFound
	}
	if id, taken := m.dedup[t.DedupKey]; taken && id != t.ID {
		return ErrDuplicate
	}
	delete(m.dedup, old.DedupKey)
	m.txs[t.ID] = t
	m.dedup[t.DedupKey] = t.ID
	return nil
}

func (m *MemoryStore) DeleteTransaction(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.txs[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.txs, id)
	delete(m.dedup, t.DedupKey)
	return nil
}

func (m *MemoryStore) Rules(context.Context) ([]Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Rule, 0, len(m.rules))
	for _, r := range m.rules {
		out = append(out, r)
	}
	return out, nil
}

func (m *MemoryStore) Rule(_ context.Context, id string) (Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rules[id]
	if !ok {
		return r, ErrNotFound
	}
	return r, nil
}

func (m *MemoryStore) PutRule(_ context.Context, r Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[r.ID] = r
	return nil
}

func (m *MemoryStore) DeleteRule(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[id]; !ok {
		return ErrNotFound
	}
	delete(m.rules, id)
	return nil
}
