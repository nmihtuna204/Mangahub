// Package customlist - Custom Lists Service
// Business logic for user-created manga lists
package customlist

import (
	"context"
	"errors"
	"strings"

	"mangahub/pkg/models"
	"mangahub/pkg/utils"
)

// Service provides custom list business logic
type Service struct {
	repo *Repository
}

// NewService creates a new custom list service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

var errListNotFound = models.NewAppError(models.ErrCodeNotFound, "list not found", 404, ErrNotFound)

func internal(msg string, err error) error {
	return models.NewAppError(models.ErrCodeInternal, msg, 500, err)
}

// CreateList creates a new custom list for a user
func (s *Service) CreateList(ctx context.Context, userID string, req models.CreateListRequest) (*models.CustomList, error) {
	req.Name = strings.TrimSpace(req.Name)
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "invalid list data", 400, err)
	}

	list := &models.CustomList{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
	}
	if err := s.repo.Create(ctx, list); err != nil {
		return nil, internal("failed to create list", err)
	}
	return list, nil
}

// ListsOf returns ownerID's lists as seen by viewerID: all of them for the
// owner, only the public ones for anyone else.
func (s *Service) ListsOf(ctx context.Context, ownerID, viewerID string) (*models.CustomListsResponse, error) {
	lists, err := s.repo.ListByUser(ctx, ownerID, ownerID != viewerID)
	if err != nil {
		return nil, internal("failed to load lists", err)
	}
	return &models.CustomListsResponse{Lists: lists, Total: len(lists)}, nil
}

// GetList returns a list with its items. A private list is visible only to
// its owner; to anyone else it doesn't exist.
func (s *Service) GetList(ctx context.Context, listID, viewerID string) (*models.CustomListWithItems, error) {
	list, err := s.visibleList(ctx, listID, viewerID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.Items(ctx, listID)
	if err != nil {
		return nil, internal("failed to load list items", err)
	}
	return &models.CustomListWithItems{CustomList: *list, Items: items}, nil
}

// UpdateList changes a list's name, description or visibility
func (s *Service) UpdateList(ctx context.Context, listID, userID string, req models.UpdateListRequest) (*models.CustomList, error) {
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "invalid list data", 400, err)
	}
	list, err := s.ownedList(ctx, listID, userID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, models.NewAppError(models.ErrCodeValidation, "name cannot be empty", 400, nil)
		}
		list.Name = name
	}
	if req.Description != nil {
		list.Description = *req.Description
	}
	if req.IsPublic != nil {
		list.IsPublic = *req.IsPublic
	}
	if err := s.repo.Update(ctx, list); err != nil {
		return nil, internal("failed to update list", err)
	}
	return list, nil
}

// DeleteList deletes a list and its items
func (s *Service) DeleteList(ctx context.Context, listID, userID string) error {
	if _, err := s.ownedList(ctx, listID, userID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, listID); err != nil {
		return internal("failed to delete list", err)
	}
	return nil
}

// AddToList adds a manga to a list (or updates its notes if already there)
func (s *Service) AddToList(ctx context.Context, listID, userID string, req models.AddToListRequest) (*models.CustomListItem, error) {
	if err := utils.ValidateStruct(req); err != nil {
		return nil, models.NewAppError(models.ErrCodeValidation, "manga_id is required", 400, err)
	}
	if _, err := s.ownedList(ctx, listID, userID); err != nil {
		return nil, err
	}
	exists, err := s.repo.MangaExists(ctx, req.MangaID)
	if err != nil {
		return nil, internal("failed to look up manga", err)
	}
	if !exists {
		return nil, models.NewAppError(models.ErrCodeNotFound, "manga not found", 404, models.ErrMangaNotFound)
	}

	item, err := s.repo.AddItem(ctx, listID, req.MangaID, req.Notes)
	if err != nil {
		return nil, internal("failed to add manga to list", err)
	}
	return item, nil
}

// RemoveFromList removes a manga from a list
func (s *Service) RemoveFromList(ctx context.Context, listID, mangaID, userID string) error {
	if _, err := s.ownedList(ctx, listID, userID); err != nil {
		return err
	}
	err := s.repo.RemoveItem(ctx, listID, mangaID)
	if errors.Is(err, ErrNotFound) {
		return models.NewAppError(models.ErrCodeNotFound, "manga is not in this list", 404, err)
	}
	if err != nil {
		return internal("failed to remove manga from list", err)
	}
	return nil
}

// ReorderList sets the item order; item_ids must list every item exactly once
func (s *Service) ReorderList(ctx context.Context, listID, userID string, req models.ReorderListRequest) error {
	if _, err := s.ownedList(ctx, listID, userID); err != nil {
		return err
	}
	ok, err := s.repo.Reorder(ctx, listID, req.ItemIDs)
	if err != nil {
		return internal("failed to reorder list", err)
	}
	if !ok {
		return models.NewAppError(models.ErrCodeValidation, "item_ids must contain every item of the list exactly once", 400, nil)
	}
	return nil
}

// visibleList returns the list if viewerID may see it
func (s *Service) visibleList(ctx context.Context, listID, viewerID string) (*models.CustomList, error) {
	list, err := s.repo.Get(ctx, listID)
	if errors.Is(err, ErrNotFound) {
		return nil, errListNotFound
	}
	if err != nil {
		return nil, internal("failed to load list", err)
	}
	if !list.IsPublic && list.UserID != viewerID {
		return nil, errListNotFound // don't reveal that a private list exists
	}
	return list, nil
}

// ownedList returns the list if userID owns it
func (s *Service) ownedList(ctx context.Context, listID, userID string) (*models.CustomList, error) {
	list, err := s.visibleList(ctx, listID, userID)
	if err != nil {
		return nil, err
	}
	if list.UserID != userID {
		return nil, models.NewAppError(models.ErrCodeForbidden, "only the list owner can change it", 403, models.ErrForbidden)
	}
	return list, nil
}
