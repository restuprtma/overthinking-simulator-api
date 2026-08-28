// Package authz centralises the runtime lookup of a user's effective
// permission set.
//
// Overthinking Simulator is single-user and has no Redis dependency:
// permissions are read straight from the database on each request via
// the PermissionFetcher seam. The Invalidate* methods remain for API
// compatibility with the role service but are no-ops (there is no cache
// to invalidate).
package authz

import (
	"context"
	"fmt"
)

// PermissionFetcher is the narrow seam the authz package needs from the
// role layer. role.RoleRepository already implements this signature — we
// accept an interface so the authz package stays decoupled from the
// rest of the role module and is trivial to fake in tests.
type PermissionFetcher interface {
	GetUserPermissions(ctx context.Context, userID string, companyID *string) ([]string, error)
}

// Service resolves a user's effective permission set directly from the
// database (via PermissionFetcher). There is no cache layer.
type Service struct {
	fetcher PermissionFetcher
}

// NewService wires a PermissionFetcher (the DB source of truth) into a
// ready-to-use authz.Service.
func NewService(fetcher PermissionFetcher) *Service {
	return &Service{fetcher: fetcher}
}

// GetPermissions returns the effective permission codes for (userID,
// companyID), read directly from the database.
func (s *Service) GetPermissions(ctx context.Context, userID, companyID string) ([]string, error) {
	if userID == "" {
		return nil, fmt.Errorf("authz: empty userID")
	}

	var cid *string
	if companyID != "" {
		cid = &companyID
	}

	perms, err := s.fetcher.GetUserPermissions(ctx, userID, cid)
	if err != nil {
		return nil, fmt.Errorf("authz: fetch user permissions: %w", err)
	}
	if perms == nil {
		// Normalise nil vs empty for a consistent return shape.
		perms = []string{}
	}
	return perms, nil
}

// Has is a convenience predicate over GetPermissions.
func (s *Service) Has(ctx context.Context, userID, companyID, permission string) (bool, error) {
	perms, err := s.GetPermissions(ctx, userID, companyID)
	if err != nil {
		return false, err
	}
	for _, p := range perms {
		if p == permission {
			return true, nil
		}
	}
	return false, nil
}

// HasAny returns true if the user has at least one of the given permissions.
func (s *Service) HasAny(ctx context.Context, userID, companyID string, permissions ...string) (bool, error) {
	perms, err := s.GetPermissions(ctx, userID, companyID)
	if err != nil {
		return false, err
	}
	set := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	for _, need := range permissions {
		if _, ok := set[need]; ok {
			return true, nil
		}
	}
	return false, nil
}

// HasAll returns true only if every required permission is present.
func (s *Service) HasAll(ctx context.Context, userID, companyID string, permissions ...string) (bool, error) {
	perms, err := s.GetPermissions(ctx, userID, companyID)
	if err != nil {
		return false, err
	}
	set := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	for _, need := range permissions {
		if _, ok := set[need]; !ok {
			return false, nil
		}
	}
	return true, nil
}

// InvalidateUser is a no-op — there is no cache to invalidate.
func (s *Service) InvalidateUser(ctx context.Context, userID string) error {
	return nil
}

// InvalidateUserCompany is a no-op — there is no cache to invalidate.
func (s *Service) InvalidateUserCompany(ctx context.Context, userID, companyID string) error {
	return nil
}

// InvalidateAll is a no-op — there is no cache to invalidate.
func (s *Service) InvalidateAll(ctx context.Context) error {
	return nil
}
