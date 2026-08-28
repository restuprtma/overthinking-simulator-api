package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	roleDomain "venturo-skeleton-go/internal/modules/core/role/domain"
	"venturo-skeleton-go/internal/modules/core/user/domain"
	"venturo-skeleton-go/internal/modules/core/user/dto"
	"venturo-skeleton-go/internal/modules/core/user/repository"
	"venturo-skeleton-go/pkg/crypto"
	jwtpkg "venturo-skeleton-go/pkg/jwt"
	"venturo-skeleton-go/pkg/logger"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrRoleNotAllowed        = errors.New("role is not allowed for caller")
	ErrCompanyNotFound       = errors.New("company not found")
	ErrCompaniesRequired     = errors.New("user must be assigned to at least one company")
)

type TenantScope struct {
	CompanyID    string
	CallerUserID string
}

type RoleLookup interface {
	FindByID(ctx context.Context, id string) (*roleDomain.Role, error)
}

type UserService struct {
	userRepo   *repository.UserRepository
	roleLookup RoleLookup
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
	}
}

func (s *UserService) SetRoleLookup(lookup RoleLookup) {
	s.roleLookup = lookup
}

func (s *UserService) validateRoleForScope(ctx context.Context, roleID *string, scope *TenantScope) error {
	if roleID == nil {
		return nil
	}
	if s.roleLookup == nil {
		return nil
	}
	role, err := s.roleLookup.FindByID(ctx, *roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return ErrRoleNotAllowed
	}
	if role.Code == jwtpkg.RoleSuperAdmin && scope != nil {
		return ErrRoleNotAllowed
	}
	return nil
}

func (s *UserService) GetAll(ctx context.Context, params *dto.UserQueryParams, scope *TenantScope) (*dto.UserListResponse, error) {
	if params.Page == 0 {
		params.Page = 1
	}
	if params.Limit == 0 {
		params.Limit = 10
	}

	users, total, err := s.userRepo.FindAll(ctx, params)
	if err != nil {
		return nil, err
	}

	userResponses := make([]dto.UserResponse, len(users))
	for i, user := range users {
		userResponses[i] = s.toUserResponse(&user)
	}

	return &dto.UserListResponse{
		Users: userResponses,
		Total: total,
		Page:  params.Page,
		Limit: params.Limit,
	}, nil
}

func (s *UserService) GetByID(ctx context.Context, id string, scope *TenantScope) (*dto.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	response := s.toUserResponse(user)
	return &response, nil
}

func (s *UserService) Create(ctx context.Context, req *dto.CreateUserRequest, createdBy string, scope *TenantScope) (*dto.UserResponse, error) {
	if err := s.validateRoleForScope(ctx, req.RoleID, scope); err != nil {
		return nil, err
	}

	exists, err := s.userRepo.EmailExists(ctx, req.Email, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	exists, err = s.userRepo.UsernameExists(ctx, req.Username, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameAlreadyExists
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		logger.Error("Failed to hash password", logger.Err(err))
		return nil, err
	}

	now := time.Now()
	user := &domain.User{
		ID:              uuid.New().String(),
		Email:           req.Email,
		Username:        req.Username,
		PasswordHash:    passwordHash,
		FullName:        req.FullName,
		Phone:           req.Phone,
		IsActive:        true,
		IsEmailVerified: true,
		CreatedAt:       now,
		CreatedBy:       &createdBy,
		UpdatedAt:       now,
		UpdatedBy:       &createdBy,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	response := s.toUserResponse(user)
	return &response, nil
}

func (s *UserService) Update(ctx context.Context, id string, req *dto.UpdateUserRequest, updatedBy string, scope *TenantScope) (*dto.UserResponse, error) {
	if err := s.validateRoleForScope(ctx, req.RoleID, scope); err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	if req.Email != nil && *req.Email != user.Email {
		exists, err := s.userRepo.EmailExists(ctx, *req.Email, &id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrEmailAlreadyExists
		}
		user.Email = *req.Email
	}

	if req.Username != nil && *req.Username != user.Username {
		exists, err := s.userRepo.UsernameExists(ctx, *req.Username, &id)
		if err != nil {
			return nil, err
		}
		if exists {
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
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	user.UpdatedAt = time.Now()
	user.UpdatedBy = &updatedBy

	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return nil, err
	}

	response := s.toUserResponse(user)
	return &response, nil
}

func (s *UserService) Delete(ctx context.Context, id, deletedBy string, scope *TenantScope) error {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	return s.userRepo.SoftDelete(ctx, id, deletedBy)
}

func (s *UserService) GetMe(ctx context.Context, userID string) (*dto.UserResponse, error) {
	return s.GetByID(ctx, userID, nil)
}

func (s *UserService) UpdateMe(ctx context.Context, userID string, req *dto.UpdateMeRequest) (*dto.UserResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	if req.FullName != nil {
		user.FullName = req.FullName
	}
	if req.Phone != nil {
		user.Phone = req.Phone
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}

	user.UpdatedAt = time.Now()
	user.UpdatedBy = &userID

	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return nil, err
	}

	response := s.toUserResponse(user)
	return &response, nil
}

func (s *UserService) ChangePassword(ctx context.Context, userID string, req *dto.ChangePasswordRequest) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	if !crypto.ComparePassword(user.PasswordHash, req.CurrentPassword) {
		return ErrInvalidPassword
	}

	newPasswordHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		logger.Error("Failed to hash new password", logger.Err(err))
		return err
	}

	return s.userRepo.UpdatePassword(ctx, userID, newPasswordHash, userID)
}

func (s *UserService) toUserResponse(user *domain.User) dto.UserResponse {
	return dto.UserResponse{
		ID:              user.ID,
		Email:           user.Email,
		Username:        user.Username,
		FullName:        user.FullName,
		Phone:           user.Phone,
		AvatarURL:       user.AvatarURL,
		IsActive:        user.IsActive,
		IsEmailVerified: user.IsEmailVerified,
		LastLoginAt:     user.LastLoginAt,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}
