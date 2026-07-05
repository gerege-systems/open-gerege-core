//go:build integration

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Байгууллагын repository-ийн RLS integration тест (non-superuser app_user).
// Migration 14-ийн self-subquery policy нь infinite recursion (42P17) өгдөг
// байсныг migration 17 (SECURITY DEFINER app_is_org_member) засна — энэ тест
// тухайн урсгалыг (гишүүн хэрэглэгч өөрийн байгууллага + гишүүдээ уншина)
// бодит RLS дор шалгаж регрессээс хамгаална.
package org_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/business/domain"
	orgpg "template/internal/datasources/repositories/postgres/org"
	"template/internal/datasources/rls"
	"template/internal/test/testenv"
)

func TestOrgRepositoryRLS_NoRecursion(t *testing.T) {
	admin := testenv.StartPostgres(t)
	app := testenv.AppUserPool(t, admin)
	ctx := context.Background()

	// app_user-т org хүснэгтүүд + users дээр эрх (production initdb script шиг).
	for _, g := range []string{
		`GRANT SELECT, INSERT, UPDATE, DELETE ON organizations TO app_user`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON organization_memberships TO app_user`,
	} {
		_, err := admin.Exec(ctx, g)
		require.NoError(t, err)
	}

	// FK-д зориулж жинхэнэ хэрэглэгч мөр (superuser-ээр, RLS тойрч).
	const userID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	_, err := admin.Exec(ctx,
		`INSERT INTO users(id, username, active, role_id, created_at)
		 VALUES ($1, 'eid_test', true, 4, now())`, userID)
	require.NoError(t, err)

	repo := orgpg.NewOrgRepository(app)

	// Байгууллага үүсгэнэ — CreateOrg нь creator-ыг автоматаар owner гишүүн болгоно.
	org, err := repo.CreateOrg(ctx, &domain.Organization{RegNo: "1234567", Name: "Тест ХХК", CreatedBy: userID})
	require.NoError(t, err)
	require.NotEmpty(t, org.ID)

	userCtx := rls.WithUser(ctx, userID)

	t.Run("ListOrgsForUser returns member org (no 42P17 recursion)", func(t *testing.T) {
		orgs, err := repo.ListOrgsForUser(userCtx, userID)
		require.NoError(t, err, "RLS recursion (42P17) байвал энд унана")
		require.Len(t, orgs, 1)
		assert.Equal(t, org.ID, orgs[0].ID)
	})

	t.Run("ListMembers returns co-members (no recursion)", func(t *testing.T) {
		members, err := repo.ListMembers(userCtx, org.ID)
		require.NoError(t, err)
		require.Len(t, members, 1)
		assert.Equal(t, userID, members[0].UserID)
	})

	t.Run("non-member sees no orgs", func(t *testing.T) {
		otherCtx := rls.WithUser(ctx, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
		orgs, err := repo.ListOrgsForUser(otherCtx, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
		require.NoError(t, err)
		assert.Empty(t, orgs)
	})
}
