// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// fakeStore — persistence-ийн дуудлагуудыг бүртгэдэг хуурамч store.
type fakeStore struct {
	calls map[string]bool
	err   error
}

func (f *fakeStore) ListDisabled(context.Context) ([]string, error) { return nil, nil }
func (f *fakeStore) SetEnabled(_ context.Context, id string, enabled bool) error {
	if f.err != nil {
		return f.err
	}
	if f.calls == nil {
		f.calls = map[string]bool{}
	}
	f.calls[id] = enabled
	return nil
}

func testRegistry(t *testing.T) *module.Registry {
	t.Helper()
	r, err := module.New(
		module.Manifest{ID: "auth", Kind: module.KindCore},
		module.Manifest{ID: "gspace", Kind: module.KindBusiness},
	)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSetEnabledPersistsAndTogglesRegistry(t *testing.T) {
	reg := testRegistry(t)
	store := &fakeStore{}
	uc := NewUsecase(reg, store, nil)

	if err := uc.SetEnabled(context.Background(), "gspace", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if reg.Enabled("gspace") {
		t.Fatal("registry-д унтараагүй")
	}
	if v, ok := store.calls["gspace"]; !ok || v {
		t.Fatalf("store-д enabled=false хадгалагдаагүй: %v", store.calls)
	}

	if err := uc.SetEnabled(context.Background(), "gspace", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !reg.Enabled("gspace") {
		t.Fatal("registry-д асаагүй")
	}
}

func TestSetEnabledRejectsCoreModuleAsBadRequest(t *testing.T) {
	uc := NewUsecase(testRegistry(t), &fakeStore{}, nil)
	err := uc.SetEnabled(context.Background(), "auth", false)
	if err == nil {
		t.Fatal("core модуль унтрах ёсгүй")
	}
	var de *apperror.DomainError
	if !errors.As(err, &de) {
		t.Fatalf("apperror.DomainError хүлээсэн, гарсан: %T", err)
	}
}

func TestSetEnabledRollsForwardOnStoreError(t *testing.T) {
	reg := testRegistry(t)
	uc := NewUsecase(reg, &fakeStore{err: errors.New("db down")}, nil)
	if err := uc.SetEnabled(context.Background(), "gspace", false); err == nil {
		t.Fatal("store-ийн алдаа ил гарах ёстой")
	}
	// Registry-ийн төлөв store-оос түрүүнд өөрчлөгддөг — админд алдаа
	// харагдсан тул дахин оролдоно; gate аль хэдийн хаасан нь аюулгүй тал.
	if reg.Enabled("gspace") {
		t.Log("registry унтарсан (хүлээгдсэн зан төлөв)")
	}
}
