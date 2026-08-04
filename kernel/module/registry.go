// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package module

import (
	"fmt"
	"strings"
	"sync"
)

// Registry нь суусан модулиудын бүртгэл + идэвхийн төлөв. Concurrency-safe:
// gate middleware нь хүсэлт бүр дээр уншдаг тул RWMutex ашиглана.
//
// Одоогийн шатанд төлөв нь зөвхөн санах ойд (boot үед env-ээс) байдаг;
// дараагийн шатанд platform_modules хүснэгттэй синк хийгдэнэ.
type Registry struct {
	mu        sync.RWMutex
	manifests map[string]Manifest
	enabled   map[string]bool
	// byPrefix — route угтвар → модулийн ID (давхцал үүсгэхгүй нь New-д шалгагдсан).
	byPrefix map[string]string
}

// New нь манифестуудаас Registry үүсгэнэ. Бүх модуль эхэндээ идэвхтэй.
// Дараах зөрчлүүдэд алдаа буцаана: давхардсан ID, огт бүртгэлгүй модулиас
// хамаарах, хамаарлын цикл, давхардсан route угтвар.
func New(manifests ...Manifest) (*Registry, error) {
	r := &Registry{
		manifests: make(map[string]Manifest, len(manifests)),
		enabled:   make(map[string]bool, len(manifests)),
		byPrefix:  make(map[string]string),
	}
	for _, m := range manifests {
		if err := m.validate(); err != nil {
			return nil, err
		}
		if _, dup := r.manifests[m.ID]; dup {
			return nil, fmt.Errorf("module: %q давхардсан ID", m.ID)
		}
		r.manifests[m.ID] = m
		r.enabled[m.ID] = true
	}
	for _, id := range sortedIDs(r.manifests) {
		m := r.manifests[id]
		for _, dep := range m.DependsOn {
			if _, ok := r.manifests[dep]; !ok {
				return nil, fmt.Errorf("module: %q нь бүртгэлгүй %q-ээс хамаарч байна", id, dep)
			}
		}
		for _, p := range m.RoutePrefixes {
			if owner, taken := r.byPrefix[p]; taken {
				return nil, fmt.Errorf("module: угтвар %q-ийг %q ба %q хоёул эзэмшиж байна", p, owner, id)
			}
			r.byPrefix[p] = id
		}
	}
	if err := r.checkCycles(); err != nil {
		return nil, err
	}
	return r, nil
}

// MustNew нь New-ийн panic хувилбар — builtin манифест эвдэрхий бол boot
// дээр шууд унагаана (программын алдаа тул нуухгүй).
func MustNew(manifests ...Manifest) *Registry {
	r, err := New(manifests...)
	if err != nil {
		panic(err)
	}
	return r
}

// checkCycles — DFS-ээр хамаарлын цикл илрүүлнэ.
func (r *Registry) checkCycles() error {
	const (
		white = 0 // үзээгүй
		gray  = 1 // одоогийн замд
		black = 2 // дууссан
	)
	color := make(map[string]int, len(r.manifests))
	var visit func(id string, path []string) error
	visit = func(id string, path []string) error {
		switch color[id] {
		case gray:
			return fmt.Errorf("module: хамаарлын цикл: %s", strings.Join(append(path, id), " → "))
		case black:
			return nil
		}
		color[id] = gray
		for _, dep := range r.manifests[id].DependsOn {
			if err := visit(dep, append(path, id)); err != nil {
				return err
			}
		}
		color[id] = black
		return nil
	}
	for _, id := range sortedIDs(r.manifests) {
		if err := visit(id, nil); err != nil {
			return err
		}
	}
	return nil
}

// Get нь манифестийг буцаана.
func (r *Registry) Get(id string) (Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.manifests[id]
	return m, ok
}

// Enabled нь модуль идэвхтэй эсэхийг буцаана. Бүртгэлгүй ID → false.
func (r *Registry) Enabled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled[id]
}

// Status — List-ийн нэг мөр.
type Status struct {
	Manifest Manifest
	Enabled  bool
}

// List нь бүх модулийг ID-ийн дарааллаар буцаана.
func (r *Registry) List() []Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Status, 0, len(r.manifests))
	for _, id := range sortedIDs(r.manifests) {
		out = append(out, Status{Manifest: r.manifests[id], Enabled: r.enabled[id]})
	}
	return out
}

// Disable нь business модулийг унтраана. Core модуль, бүртгэлгүй модуль,
// эсвэл идэвхтэй өөр модуль түүнээс хамаарч байвал алдаа буцаана
// (хамаарагчийг нь ЭХЭЛЖ унтраах ёстой — далд каскад хийхгүй).
func (r *Registry) Disable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.manifests[id]
	if !ok {
		return fmt.Errorf("module: %q бүртгэлгүй", id)
	}
	if m.Kind == KindCore {
		return fmt.Errorf("module: %q нь core модуль тул унтраах боломжгүй", id)
	}
	if !r.enabled[id] {
		return nil
	}
	for _, otherID := range sortedIDs(r.manifests) {
		if otherID == id || !r.enabled[otherID] {
			continue
		}
		for _, dep := range r.manifests[otherID].DependsOn {
			if dep == id {
				return fmt.Errorf("module: %q-ийг унтраахын өмнө түүнээс хамаардаг %q-ийг унтраана уу", id, otherID)
			}
		}
	}
	r.enabled[id] = false
	return nil
}

// Enable нь модулийг асаана. Хамаарлууд нь идэвхгүй бол алдаа буцаана.
func (r *Registry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.manifests[id]
	if !ok {
		return fmt.Errorf("module: %q бүртгэлгүй", id)
	}
	for _, dep := range m.DependsOn {
		if !r.enabled[dep] {
			return fmt.Errorf("module: %q-ийг асаахын өмнө хамаарал %q-ийг асаана уу", id, dep)
		}
	}
	r.enabled[id] = true
	return nil
}

// ApplyDisabledList нь "gov,relay, gspace" маягийн CSV жагсаалтаар модулиудыг
// унтраана (MODULES_DISABLED env). Хамаарлын дарааллыг өөрөө шийднэ:
// хамаарагч нь мөн жагсаалтад байгаа бол эхэлж унтардаг. Хоосон жагсаалт OK.
func (r *Registry) ApplyDisabledList(csv string) error {
	var ids []string
	for _, raw := range strings.Split(csv, ",") {
		id := strings.TrimSpace(raw)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	// Fixed-point: дараалал таарахгүйгээс болж бүтэлгүйтсэнийг дараагийн
	// давталтад дахин оролдоно; ямар ч ахиц гарахгүй бол жинхэнэ алдаа.
	pending := ids
	for len(pending) > 0 {
		var next []string
		var lastErr error
		for _, id := range pending {
			if err := r.Disable(id); err != nil {
				next = append(next, id)
				lastErr = err
			}
		}
		if len(next) == len(pending) {
			return lastErr
		}
		pending = next
	}
	return nil
}
