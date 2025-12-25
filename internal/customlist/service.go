// Package customlist - Custom Lists Service
// Business logic for user-created manga lists
package customlist

import (
	"context"
	"fmt"

	"mangahub/pkg/database"
	"mangahub/pkg/models"
)

// Service provides custom list business logic
type Service struct {
	repo *Repository
	db   *database.DB
}

// NewService creates a new custom list service
func NewService(db *database.DB) *Service {
	return &Service{
		repo: NewRepository(db),
		db:   db,
	}
}

// CreateList creates a new custom list for a user
func (s *Service) CreateList(ctx context.Context, userID string, req *models.CreateListRequest) (*models.CustomList, error) {
	// Validate
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	list := &models.CustomList{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
	}

	if err := s.repo.CreateList(list); err != nil {
		return nil, err
	}

	return list, nil
}

// GetUserLists returns all lists for a user
func (s *Service) GetUserLists(ctx context.Context, userID string) (*models.CustomListsResponse, error) {
	lists, err := s.repo.GetUserLists(userID)
	if err != nil {
		return nil, err
	}

	return &models.CustomListsResponse{
		Lists: lists,
		Total: len(lists),
	}, nil
}

// GetList returns a list by ID (with permission check)
func (s *Service) GetList(ctx context.Context, listID, userID string) (*models.CustomList, error) {
	list, err := s.repo.GetList(listID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, fmt.Errorf("list not found")
	}

	// Check permission
	if list.UserID != userID && !list.IsPublic {
		return nil, fmt.Errorf("unauthorized")
	}

	return list, nil
}

// GetListWithItems returns a list with all its items
func (s *Service) GetListWithItems(ctx context.Context, listID, userID string) (*models.CustomListWithItems, error) {
	list, err := s.GetList(ctx, listID, userID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.GetListItems(listID)
	if err != nil {
		return nil, err
	}

	return &models.CustomListWithItems{
		CustomList: *list,
		Items:      items,
	}, nil
}

// UpdateList updates a custom list
func (s *Service) UpdateList(ctx context.Context, listID, userID string, req *models.UpdateListRequest) (*models.CustomList, error) {
	list, err := s.repo.GetList(listID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, fmt.Errorf("list not found")
	}
	if list.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	// Apply updates
	if req.Name != nil {
		list.Name = *req.Name
	}
	if req.Description != nil {
		list.Description = *req.Description
	}
	if req.IsPublic != nil {
		list.IsPublic = *req.IsPublic
	}

	if err := s.repo.UpdateList(list); err != nil {
		return nil, err
	}

	return list, nil
}

// DeleteList deletes a custom list
func (s *Service) DeleteList(ctx context.Context, listID, userID string) error {
	return s.repo.DeleteList(listID, userID)
}

// AddToList adds a manga to a list
func (s *Service) AddToList(ctx context.Context, listID, userID string, req *models.AddToListRequest) error {
	if req.MangaID == "" {
		return fmt.Errorf("manga_id is required")
	}
	return s.repo.AddMangaToList(listID, req.MangaID, userID, req.Notes)
}

// RemoveFromList removes a manga from a list
func (s *Service) RemoveFromList(ctx context.Context, listID, mangaID, userID string) error {
	return s.repo.RemoveMangaFromList(listID, mangaID, userID)
}

// ReorderList reorders items in a list
func (s *Service) ReorderList(ctx context.Context, listID, userID string, req *models.ReorderListRequest) error {
	if len(req.ItemIDs) == 0 {
		return fmt.Errorf("item_ids is required")
	}
	return s.repo.ReorderListItems(listID, userID, req.ItemIDs)
}
