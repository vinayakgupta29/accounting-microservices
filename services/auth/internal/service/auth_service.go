package service

import (
	"context"
	"errors"
	"strings"

	authv1 "github.com/accounting-microservices/gen/go/auth/v1"
	"github.com/accounting-microservices/services/auth/internal/crypto"
	"github.com/accounting-microservices/services/auth/internal/repository"
	"github.com/accounting-microservices/services/auth/internal/token"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthServiceServer implements the gRPC auth.v1.AuthServiceServer interface.
type AuthServiceServer struct {
	authv1.UnimplementedAuthServiceServer
	repo         repository.UserRepository
	tokenManager *token.Manager
}

// NewAuthServiceServer creates a new gRPC AuthService server instance.
func NewAuthServiceServer(repo repository.UserRepository, tm *token.Manager) *AuthServiceServer {
	return &AuthServiceServer{
		repo:         repo,
		tokenManager: tm,
	}
}

// Register provisions a new user with Argon2id encrypted credentials.
func (s *AuthServiceServer) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	if username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}
	password := req.GetPassword()
	if len(password) < 6 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 6 characters")
	}

	// Check if user already exists
	existing, err := s.repo.FindByUsername(ctx, username)
	if err == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "username already exists")
	}

	// Hash password using Argon2id
	hashedPassword, err := crypto.HashPassword(password)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to hash password: %v", err)
	}

	newUser := &repository.User{
		Name:         strings.TrimSpace(req.GetName()),
		Username:     username,
		Email:        strings.TrimSpace(req.GetEmail()),
		PasswordHash: hashedPassword,
		GSTIN:        strings.TrimSpace(req.GetGstin()),
		PANCard:      strings.TrimSpace(req.GetPan()),
		Aadhaar:      strings.TrimSpace(req.GetAadhaar()),
		Phone:        strings.TrimSpace(req.GetPhone()),
		Address:      strings.TrimSpace(req.GetAddress()),
	}

	if err := s.repo.Create(ctx, newUser); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	// Generate JWT token
	jwtToken, err := s.tokenManager.GenerateToken(newUser.ID, newUser.Username)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token: %v", err)
	}

	return &authv1.RegisterResponse{
		UserId:   newUser.ID,
		Username: newUser.Username,
		Token:    jwtToken,
		Message:  "User registered successfully",
	}, nil
}

// Login verifies user credentials against the Argon2id hash and issues a JWT token.
func (s *AuthServiceServer) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	username := strings.TrimSpace(req.GetUsername())
	if username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}

	user, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, status.Error(codes.Unauthenticated, "invalid username or password")
		}
		return nil, status.Errorf(codes.Internal, "database query error: %v", err)
	}

	// Verify Argon2id password hash
	match, err := crypto.VerifyPassword(req.GetPassword(), user.PasswordHash)
	if err != nil || !match {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	jwtToken, err := s.tokenManager.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token: %v", err)
	}

	return &authv1.LoginResponse{
		UserId:   user.ID,
		Username: user.Username,
		Token:    jwtToken,
		Message:  "Login successful",
	}, nil
}

// ValidateToken verifies JWT validity and returns claims for inter-service authentication.
func (s *AuthServiceServer) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	claims, err := s.tokenManager.ValidateToken(req.GetToken())
	if err != nil {
		return &authv1.ValidateTokenResponse{
			Valid:        false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &authv1.ValidateTokenResponse{
		Valid:    true,
		UserId:   claims.UserID,
		Username: claims.Username,
	}, nil
}

// GetUserProfile retrieves metadata for the requested user profile.
func (s *AuthServiceServer) GetUserProfile(ctx context.Context, req *authv1.GetUserProfileRequest) (*authv1.UserProfile, error) {
	var user *repository.User
	var err error

	if req.GetUserId() != "" {
		user, err = s.repo.FindByID(ctx, req.GetUserId())
	} else if req.GetUsername() != "" {
		user, err = s.repo.FindByUsername(ctx, req.GetUsername())
	} else {
		return nil, status.Error(codes.InvalidArgument, "user_id or username required")
	}

	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "database error: %v", err)
	}

	return &authv1.UserProfile{
		UserId:    user.ID,
		Name:      user.Name,
		Username:  user.Username,
		Email:     user.Email,
		Gstin:     user.GSTIN,
		Pan:       user.PANCard,
		Aadhaar:   user.Aadhaar,
		Phone:     user.Phone,
		Address:   user.Address,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
