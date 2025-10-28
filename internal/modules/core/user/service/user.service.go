package service

import (
	"errors"
	"math"
	"time"

	"venturo-skeleton-go/internal/modules/core/user/domain"
	"venturo-skeleton-go/internal/modules/core/user/dto"
	"venturo-skeleton-go/internal/modules/core/user/repository"
	roleRepo "venturo-skeleton-go/internal/modules/core/role/repository"
	"venturo-skeleton-go/pkg/crypto"
	"venturo-skeleton-go/pkg/logger"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrRoleNotFound          = errors.New("role not found")
	ErrInvalidRoleID         = errors.New("invalid role ID")
)

type UserService struct {
	userRepo *repository.UserRepository
	roleRepo *roleRepo.RoleRepository
}

func NewUserService(userRepo *repository.UserRepository, roleRepo *roleRepo.RoleRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
		roleRepo: roleRepo,
	}
}

// GetAll gets all users with pagination
func (s *UserService) GetAll(params *dto.UserQueryParams) (*dto.UserListResponse, error) {
	logger.Info("Fetching users list",
		logger.Int("page", params.Page),
		logger.Int("page_size", params.PageSize),
		logger.String("search", params.Search),
		logger.String("full_name", params.FullName),
		logger.String("username", params.Username),
		logger.String("email", params.Email),
	)

	// Calculate offset
	offset := (params.Page - 1) * params.PageSize

	// Get users with roles
	usersWithRoles, err := s.userRepo.FindAllWithRoles(params.PageSize, offset, params.Search, params.FullName, params.Username, params.Email, params.IsActive)
	if err != nil {
		logger.Error("Failed to fetch users", logger.Err(err))
		return nil, err
	}

	// Get total count
	total, err := s.userRepo.Count(params.Search, params.FullName, params.Username, params.Email, params.IsActive)
	if err != nil {
		logger.Error("Failed to count users", logger.Err(err))
		return nil, err
	}

	// Convert to response
	userResponses := make([]dto.UserResponse, len(usersWithRoles))
	for i, userWithRoles := range usersWithRoles {
		userResponses[i] = s.toUserResponseWithRoles(&userWithRoles.User, userWithRoles.Roles)
	}

	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &dto.UserListResponse{
		Users:      userResponses,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetByID gets a user by ID
func (s *UserService) GetByID(id string) (*dto.UserResponse, error) {
	logger.Info("Fetching user by ID", logger.String("user_id", id))

	user, err := s.userRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch user", logger.Err(err))
		return nil, err
	}

	if user == nil {
		logger.Warn("User not found", logger.String("user_id", id))
		return nil, ErrUserNotFound
	}

	response := s.toUserResponse(user)
	return &response, nil
}

// Create creates a new user
func (s *UserService) Create(createdByUserID string, req *dto.CreateUserRequest) (*dto.UserResponse, error) {
	logger.Info("Creating new user", logger.String("email", req.Email))

	// Check if email already exists
	existingUser, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		logger.Error("Failed to check existing email", logger.Err(err))
		return nil, err
	}
	if existingUser != nil {
		logger.Warn("Email already exists", logger.String("email", req.Email))
		return nil, ErrEmailAlreadyExists
	}

	// Check if username already exists
	existingUser, err = s.userRepo.FindByUsername(req.Username)
	if err != nil {
		logger.Error("Failed to check existing username", logger.Err(err))
		return nil, err
	}
	if existingUser != nil {
		logger.Warn("Username already exists", logger.String("username", req.Username))
		return nil, ErrUsernameAlreadyExists
	}

	// Validate role IDs if provided
	if len(req.RoleIDs) > 0 {
		if err := s.validateRoleIDs(req.RoleIDs); err != nil {
			logger.Error("Invalid role IDs", logger.Err(err))
			return nil, err
		}
	}

	// Hash password
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		logger.Error("Failed to hash password", logger.Err(err))
		return nil, err
	}

	// Create user
	userID := uuid.New().String()
	now := time.Now()
	user := &domain.User{
		ID:              userID,
		Email:           req.Email,
		Username:        req.Username,
		PasswordHash:    passwordHash,
		FullName:        req.FullName,
		Phone:           req.Phone,
		IsActive:        true,
		IsEmailVerified: false,
		CreatedAt:       now,
		CreatedBy:       &createdByUserID,
		UpdatedAt:       now,
		UpdatedBy:       &createdByUserID,
	}

	err = s.userRepo.Create(user)
	if err != nil {
		logger.Error("Failed to create user", logger.Err(err))
		return nil, err
	}

	// Assign roles if provided
	if len(req.RoleIDs) > 0 {
		err = s.userRepo.AssignRoles(userID, req.RoleIDs, userID)
		if err != nil {
			logger.Error("Failed to assign roles to user", logger.Err(err))
			// Note: User is already created, log error but continue
		} else {
			logger.Info("Roles assigned to user", logger.String("user_id", userID), logger.Int("role_count", len(req.RoleIDs)))
		}
	}

	logger.Info("User created successfully", logger.String("user_id", user.ID))

	// Fetch user with roles for response
	if len(req.RoleIDs) > 0 {
		usersWithRoles, err := s.userRepo.FindAllWithRoles(1, 0, "", "", "", user.Email, nil)
		if err == nil && len(usersWithRoles) > 0 {
			response := s.toUserResponseWithRoles(&usersWithRoles[0].User, usersWithRoles[0].Roles)
			return &response, nil
		}
	}

	response := s.toUserResponse(user)
	return &response, nil
}

// Update updates a user
func (s *UserService) Update(id string, updatedByUserID string, req *dto.UpdateUserRequest) (*dto.UserResponse, error) {
	logger.Info("Updating user", logger.String("user_id", id))

	// Find user
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch user", logger.Err(err))
		return nil, err
	}
	if user == nil {
		logger.Warn("User not found", logger.String("user_id", id))
		return nil, ErrUserNotFound
	}

	// Validate role IDs if provided
	if req.RoleIDs != nil && len(req.RoleIDs) > 0 {
		if err := s.validateRoleIDs(req.RoleIDs); err != nil {
			logger.Error("Invalid role IDs", logger.Err(err))
			return nil, err
		}
	}

	// Update fields if provided
	if req.Email != nil && *req.Email != user.Email {
		// Check if new email already exists
		existingUser, err := s.userRepo.FindByEmail(*req.Email)
		if err != nil {
			logger.Error("Failed to check existing email", logger.Err(err))
			return nil, err
		}
		if existingUser != nil && existingUser.ID != id {
			logger.Warn("Email already exists", logger.String("email", *req.Email))
			return nil, ErrEmailAlreadyExists
		}
		user.Email = *req.Email
	}

	if req.Username != nil && *req.Username != user.Username {
		// Check if new username already exists
		existingUser, err := s.userRepo.FindByUsername(*req.Username)
		if err != nil {
			logger.Error("Failed to check existing username", logger.Err(err))
			return nil, err
		}
		if existingUser != nil && existingUser.ID != id {
			logger.Warn("Username already exists", logger.String("username", *req.Username))
			return nil, ErrUsernameAlreadyExists
		}
		user.Username = *req.Username
	}

	if req.FullName != nil {
		user.FullName = req.FullName
	}

	if req.Phone != nil {
		user.Phone = req.Phone
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	user.UpdatedAt = time.Now()
	user.UpdatedBy = &updatedByUserID

	// Update user
	err = s.userRepo.Update(user)
	if err != nil {
		logger.Error("Failed to update user", logger.Err(err))
		return nil, err
	}

	// Sync roles if provided
	if req.RoleIDs != nil {
		err = s.userRepo.SyncRoles(id, req.RoleIDs, id)
		if err != nil {
			logger.Error("Failed to sync roles", logger.Err(err))
			// Continue despite role sync failure
		} else {
			logger.Info("Roles synced for user", logger.String("user_id", id), logger.Int("role_count", len(req.RoleIDs)))
		}
	}

	logger.Info("User updated successfully", logger.String("user_id", id))

	// Fetch user with roles for response
	usersWithRoles, err := s.userRepo.FindAllWithRoles(1, 0, "", "", "", user.Email, nil)
	if err == nil && len(usersWithRoles) > 0 {
		response := s.toUserResponseWithRoles(&usersWithRoles[0].User, usersWithRoles[0].Roles)
		return &response, nil
	}

	response := s.toUserResponse(user)
	return &response, nil
}

// Delete deletes a user (soft delete)
func (s *UserService) Delete(id string, deletedBy string) error {
	logger.Info("Deleting user", logger.String("user_id", id))

	// Check if user exists
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch user", logger.Err(err))
		return err
	}
	if user == nil {
		logger.Warn("User not found", logger.String("user_id", id))
		return ErrUserNotFound
	}

	// Soft delete
	err = s.userRepo.Delete(id, deletedBy)
	if err != nil {
		logger.Error("Failed to delete user", logger.Err(err))
		return err
	}

	logger.Info("User deleted successfully", logger.String("user_id", id))
	return nil
}

// Restore restores a soft-deleted user
func (s *UserService) Restore(id string) (*dto.UserResponse, error) {
	logger.Info("Restoring user", logger.String("user_id", id))

	// Restore user
	err := s.userRepo.Restore(id)
	if err != nil {
		logger.Error("Failed to restore user", logger.Err(err))
		return nil, err
	}

	// Fetch restored user
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch restored user", logger.Err(err))
		return nil, err
	}
	if user == nil {
		logger.Warn("User not found after restore", logger.String("user_id", id))
		return nil, ErrUserNotFound
	}

	logger.Info("User restored successfully", logger.String("user_id", id))

	response := s.toUserResponse(user)
	return &response, nil
}

// ChangePassword changes user password
func (s *UserService) ChangePassword(id string, req *dto.ChangePasswordRequest) error {
	logger.Info("Changing password", logger.String("user_id", id))

	// Find user
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		logger.Error("Failed to fetch user", logger.Err(err))
		return err
	}
	if user == nil {
		logger.Warn("User not found", logger.String("user_id", id))
		return ErrUserNotFound
	}

	// Verify old password
	if !crypto.ComparePassword(user.PasswordHash, req.OldPassword) {
		logger.Warn("Invalid old password", logger.String("user_id", id))
		return ErrInvalidPassword
	}

	// Hash new password
	newPasswordHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		logger.Error("Failed to hash new password", logger.Err(err))
		return err
	}

	// Update password
	err = s.userRepo.UpdatePassword(id, newPasswordHash, id)
	if err != nil {
		logger.Error("Failed to update password", logger.Err(err))
		return err
	}

	logger.Info("Password changed successfully", logger.String("user_id", id))
	return nil
}

// Helper: Convert domain.User to dto.UserResponse
func (s *UserService) toUserResponse(user *domain.User) dto.UserResponse {
	return dto.UserResponse{
		ID:              user.ID,
		Email:           user.Email,
		Username:        user.Username,
		FullName:        user.FullName,
		Phone:           user.Phone,
		IsActive:        user.IsActive,
		IsEmailVerified: user.IsEmailVerified,
		LastLoginAt:     user.LastLoginAt,
		Roles:           []dto.RoleInfo{}, // Empty roles for backward compatibility
		CreatedAt:       user.CreatedAt,
		CreatedBy:       user.CreatedBy,
		UpdatedAt:       user.UpdatedAt,
		UpdatedBy:       user.UpdatedBy,
		DeletedAt:       user.DeletedAt,
		DeletedBy:       user.DeletedBy,
	}
}

// AssignRolesToUser assigns roles to a user (adds to existing roles)
func (s *UserService) AssignRolesToUser(userID string, req *dto.AssignRolesRequest, assignedBy string) (*dto.UserResponse, error) {
	logger.Info("Assigning roles to user", logger.String("user_id", userID), logger.Int("role_count", len(req.RoleIDs)))

	// Check if user exists
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		logger.Error("Failed to fetch user", logger.Err(err))
		return nil, err
	}
	if user == nil {
		logger.Warn("User not found", logger.String("user_id", userID))
		return nil, ErrUserNotFound
	}

	// Validate role IDs
	if err := s.validateRoleIDs(req.RoleIDs); err != nil {
		logger.Error("Invalid role IDs", logger.Err(err))
		return nil, err
	}

	// Assign roles
	err = s.userRepo.AssignRoles(userID, req.RoleIDs, assignedBy)
	if err != nil {
		logger.Error("Failed to assign roles", logger.Err(err))
		return nil, err
	}

	logger.Info("Roles assigned successfully", logger.String("user_id", userID))

	// Fetch user with updated roles
	usersWithRoles, err := s.userRepo.FindAllWithRoles(1, 0, "", "", "", user.Email, nil)
	if err == nil && len(usersWithRoles) > 0 {
		response := s.toUserResponseWithRoles(&usersWithRoles[0].User, usersWithRoles[0].Roles)
		return &response, nil
	}

	response := s.toUserResponse(user)
	return &response, nil
}

// RemoveRoleFromUser removes a role from a user
func (s *UserService) RemoveRoleFromUser(userID, roleID, deletedBy string) (*dto.UserResponse, error) {
	logger.Info("Removing role from user", logger.String("user_id", userID), logger.String("role_id", roleID))

	// Check if user exists
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		logger.Error("Failed to fetch user", logger.Err(err))
		return nil, err
	}
	if user == nil {
		logger.Warn("User not found", logger.String("user_id", userID))
		return nil, ErrUserNotFound
	}

	// Validate role exists
	role, err := s.roleRepo.FindByID(roleID)
	if err != nil {
		logger.Error("Failed to fetch role", logger.Err(err))
		return nil, err
	}
	if role == nil {
		logger.Warn("Role not found", logger.String("role_id", roleID))
		return nil, ErrRoleNotFound
	}

	// Remove role
	err = s.userRepo.RemoveRole(userID, roleID, deletedBy)
	if err != nil {
		logger.Error("Failed to remove role", logger.Err(err))
		return nil, err
	}

	logger.Info("Role removed successfully", logger.String("user_id", userID), logger.String("role_id", roleID))

	// Fetch user with updated roles
	usersWithRoles, err := s.userRepo.FindAllWithRoles(1, 0, "", "", "", user.Email, nil)
	if err == nil && len(usersWithRoles) > 0 {
		response := s.toUserResponseWithRoles(&usersWithRoles[0].User, usersWithRoles[0].Roles)
		return &response, nil
	}

	response := s.toUserResponse(user)
	return &response, nil
}

// SyncUserRoles syncs/replaces all user roles
func (s *UserService) SyncUserRoles(userID string, req *dto.SyncRolesRequest, updatedBy string) (*dto.UserResponse, error) {
	logger.Info("Syncing roles for user", logger.String("user_id", userID), logger.Int("role_count", len(req.RoleIDs)))

	// Check if user exists
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		logger.Error("Failed to fetch user", logger.Err(err))
		return nil, err
	}
	if user == nil {
		logger.Warn("User not found", logger.String("user_id", userID))
		return nil, ErrUserNotFound
	}

	// Validate role IDs if not empty
	if len(req.RoleIDs) > 0 {
		if err := s.validateRoleIDs(req.RoleIDs); err != nil {
			logger.Error("Invalid role IDs", logger.Err(err))
			return nil, err
		}
	}

	// Sync roles
	err = s.userRepo.SyncRoles(userID, req.RoleIDs, updatedBy)
	if err != nil {
		logger.Error("Failed to sync roles", logger.Err(err))
		return nil, err
	}

	logger.Info("Roles synced successfully", logger.String("user_id", userID))

	// Fetch user with updated roles
	usersWithRoles, err := s.userRepo.FindAllWithRoles(1, 0, "", "", "", user.Email, nil)
	if err == nil && len(usersWithRoles) > 0 {
		response := s.toUserResponseWithRoles(&usersWithRoles[0].User, usersWithRoles[0].Roles)
		return &response, nil
	}

	response := s.toUserResponse(user)
	return &response, nil
}

// validateRoleIDs validates that all role IDs exist and are active
func (s *UserService) validateRoleIDs(roleIDs []string) error {
	for _, roleID := range roleIDs {
		role, err := s.roleRepo.FindByID(roleID)
		if err != nil {
			return err
		}
		if role == nil {
			logger.Warn("Invalid role ID", logger.String("role_id", roleID))
			return ErrInvalidRoleID
		}
	}
	return nil
}

// Helper: Convert domain.User with roles to dto.UserResponse
func (s *UserService) toUserResponseWithRoles(user *domain.User, roles []repository.RoleInfo) dto.UserResponse {
	roleInfos := make([]dto.RoleInfo, len(roles))
	for i, role := range roles {
		roleInfos[i] = dto.RoleInfo{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			IsSystem:    role.IsSystem,
		}
	}

	return dto.UserResponse{
		ID:              user.ID,
		Email:           user.Email,
		Username:        user.Username,
		FullName:        user.FullName,
		Phone:           user.Phone,
		IsActive:        user.IsActive,
		IsEmailVerified: user.IsEmailVerified,
		LastLoginAt:     user.LastLoginAt,
		Roles:           roleInfos,
		CreatedAt:       user.CreatedAt,
		CreatedBy:       user.CreatedBy,
		UpdatedAt:       user.UpdatedAt,
		UpdatedBy:       user.UpdatedBy,
		DeletedAt:       user.DeletedAt,
		DeletedBy:       user.DeletedBy,
	}
}