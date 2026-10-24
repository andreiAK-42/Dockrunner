package integrations

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/admin"
	"dockrunner_backend/internal/modules/integrations/engine"
	"dockrunner_backend/internal/modules/integrations/mongo"
	"dockrunner_backend/internal/modules/integrations/ollama"
	"dockrunner_backend/internal/modules/integrations/postgres"
	"dockrunner_backend/internal/modules/integrations/redis"
)

var (
	ErrUnknownKind   = errors.New("неизвестная интеграция")
	ErrQuotaExceeded = errors.New("лимит места тарифа исчерпан")
	ErrNotFound      = errors.New("учётка не найдена")
	ErrAlreadyExists = errors.New("учётка для этой интеграции уже создана")
)

type Settings struct {
	PostgresHost     string
	PostgresPort     int
	PostgresName     string
	PostgresAdminURL string
	MongoHost        string
	MongoPort        int
	MongoName        string
	MongoAdminURL    string
	RedisHost        string
	RedisPort        int
	RedisPassword    string
	OllamaHost       string
	OllamaPort       int
	OllamaURL        string
	OllamaAPIKey     string
}

type Kind struct {
	ID          string `json:"id"`
	Group       string `json:"group"`
	Name        string `json:"name"`
	DefaultPort string `json:"defaultPort"`
	Description string `json:"description"`
	HasQuota    bool   `json:"hasQuota"`
	Enabled     bool   `json:"enabled"`
}

type CreatedAccount struct {
	Account          Account
	Password         string
	Rows             []CredentialRow
	ConnectionString string
}

type CredentialRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Service struct {
	repo     *Repository
	cfg      Settings
	managers map[string]engine.Manager
	logger   *zap.Logger
}

func NewService(repo *Repository, cfg Settings, logger *zap.Logger) *Service {
	return &Service{
		repo: repo,
		cfg:  cfg,
		managers: map[string]engine.Manager{
			"postgres": postgres.Manager{AdminURL: cfg.PostgresAdminURL},
			"mongo":    mongo.Manager{AdminURL: cfg.MongoAdminURL},
			"redis":    redis.Manager{Addr: fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort), Password: cfg.RedisPassword},
			"ollama":   ollama.Manager{BaseURL: cfg.OllamaURL},
		},
		logger: logger,
	}
}

func (s *Service) Catalog() []Kind {
	return []Kind{
		{ID: "postgres", Group: "database", Name: "PostgreSQL", DefaultPort: fmt.Sprint(s.cfg.PostgresPort), Description: "Реляционная база для приложений.", HasQuota: true, Enabled: s.cfg.PostgresAdminURL != ""},
		{ID: "mongo", Group: "database", Name: "MongoDB", DefaultPort: fmt.Sprint(s.cfg.MongoPort), Description: "Документная база для гибких схем.", HasQuota: true, Enabled: s.cfg.MongoAdminURL != ""},
		{ID: "redis", Group: "database", Name: "Redis", DefaultPort: fmt.Sprint(s.cfg.RedisPort), Description: "Кэш и очереди сообщений.", HasQuota: true, Enabled: s.cfg.RedisHost != ""},
		{ID: "ollama", Group: "ai", Name: "Ollama", DefaultPort: fmt.Sprint(s.cfg.OllamaPort), Description: "Локальные языковые модели по HTTP API.", HasQuota: false, Enabled: s.cfg.OllamaURL != ""},
	}
}

const passwordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

func randomPassword(length int) (string, error) {
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordAlphabet))))
		if err != nil {
			return "", err
		}
		out[i] = passwordAlphabet[n.Int64()]
	}
	return string(out), nil
}

func sanitizeLogin(login string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(login) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "user"
	}
	out := b.String()
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

func (s *Service) Usage(ctx context.Context, owner string) (usedGB, limitGB float64) {
	plan := admin.FindPlan(s.repo.UserPlan(ctx, owner))
	return s.repo.DbUsageGB(ctx, owner), plan.DbStorageGB
}

func (s *Service) Mine(ctx context.Context, owner string) ([]Account, error) {
	return s.repo.ListByOwner(ctx, owner)
}

func (s *Service) Create(ctx context.Context, owner, kind string) (*CreatedAccount, error) {
	manager, ok := s.managers[kind]
	if !ok {
		return nil, ErrUnknownKind
	}
	if existing, err := s.repo.GetByOwnerKind(ctx, owner, kind); err == nil && existing != nil {
		return nil, ErrAlreadyExists
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if kind != "ollama" {
		plan := admin.FindPlan(s.repo.UserPlan(ctx, owner))
		if used := s.repo.StorageUsageGB(ctx, owner); used >= plan.DbStorageGB {
			return nil, ErrQuotaExceeded
		}
	}

	username := "dr_" + sanitizeLogin(owner)
	resource := username
	password, err := randomPassword(20)
	if err != nil {
		return nil, err
	}
	if kind == "ollama" && s.cfg.OllamaAPIKey != "" {
		password = s.cfg.OllamaAPIKey
	}

	host, port := s.endpoint(kind)
	if kind == "postgres" {
		resource = s.cfg.PostgresName + "_" + sanitizeLogin(owner)
	}
	if kind == "mongo" {
		resource = s.cfg.MongoName + "_" + sanitizeLogin(owner)
	}
	if kind == "ollama" {
		resource = "default"
		username = owner
	}

	if err := manager.CreateUser(ctx, resource, username, password); err != nil {
		return nil, err
	}
	rec := &Account{
		ID: uuid.New(), OwnerLogin: owner, Kind: kind,
		Resource: resource, Username: username, Host: host, Port: port, SizeGB: 0,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		if dropErr := manager.DropUser(ctx, resource, username); dropErr != nil {
			s.logger.Warn("integration rollback failed", zap.String("kind", kind), zap.String("user", username), zap.Error(dropErr))
		}
		return nil, err
	}
	return s.created(*rec, password), nil
}

func (s *Service) ResetPassword(ctx context.Context, owner string, id uuid.UUID) (*CreatedAccount, error) {
	rec, err := s.owned(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	manager, ok := s.managers[rec.Kind]
	if !ok {
		return nil, ErrUnknownKind
	}
	password, err := randomPassword(20)
	if err != nil {
		return nil, err
	}
	if rec.Kind == "ollama" {
		if s.cfg.OllamaAPIKey != "" {
			password = s.cfg.OllamaAPIKey
		}
		return s.created(*rec, password), nil
	}
	if err := manager.DropUser(ctx, rec.Resource, rec.Username); err != nil {
		s.logger.Warn("drop before reset failed", zap.Error(err))
	}
	if err := manager.CreateUser(ctx, rec.Resource, rec.Username, password); err != nil {
		return nil, err
	}
	return s.created(*rec, password), nil
}

func (s *Service) Delete(ctx context.Context, owner string, id uuid.UUID) error {
	rec, err := s.owned(ctx, owner, id)
	if err != nil {
		return err
	}
	manager, ok := s.managers[rec.Kind]
	if !ok {
		return ErrUnknownKind
	}
	if err := manager.DropUser(ctx, rec.Resource, rec.Username); err != nil {
		s.logger.Warn("drop user failed, record kept", zap.String("user", rec.Username), zap.Error(err))
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) owned(ctx context.Context, owner string, id uuid.UUID) (*Account, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if rec.OwnerLogin != owner {
		return nil, ErrNotFound
	}
	return rec, nil
}

func (s *Service) created(rec Account, password string) *CreatedAccount {
	return &CreatedAccount{
		Account:          rec,
		Password:         password,
		Rows:             buildRows(rec.Kind, rec.Host, rec.Port, rec.Resource, rec.Username, password),
		ConnectionString: buildConnString(rec.Kind, rec.Host, rec.Port, rec.Resource, rec.Username, password),
	}
}

func (s *Service) endpoint(kind string) (string, int) {
	switch kind {
	case "postgres":
		return s.cfg.PostgresHost, s.cfg.PostgresPort
	case "mongo":
		return s.cfg.MongoHost, s.cfg.MongoPort
	case "redis":
		return s.cfg.RedisHost, s.cfg.RedisPort
	case "ollama":
		return s.cfg.OllamaHost, s.cfg.OllamaPort
	default:
		return "", 0
	}
}

func buildRows(kind, host string, port int, resource, username, password string) []CredentialRow {
	rows := []CredentialRow{
		{Label: "Адрес", Value: host},
		{Label: "Порт", Value: fmt.Sprint(port)},
		{Label: "Пользователь", Value: username},
		{Label: "Пароль", Value: password},
	}
	switch kind {
	case "postgres", "mongo":
		rows = append(rows[:2], append([]CredentialRow{{Label: "База данных", Value: resource}}, rows[2:]...)...)
	case "ollama":
		rows = []CredentialRow{
			{Label: "Адрес", Value: host},
			{Label: "Порт", Value: fmt.Sprint(port)},
			{Label: "Base URL", Value: fmt.Sprintf("http://%s:%d", host, port)},
		}
		if password != "" {
			rows = append(rows, CredentialRow{Label: "API-ключ", Value: password})
		}
	}
	return rows
}

func buildConnString(kind, host string, port int, resource, username, password string) string {
	switch kind {
	case "postgres":
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", username, password, host, port, resource)
	case "mongo":
		return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", username, password, host, port, resource)
	case "redis":
		return fmt.Sprintf("redis://%s:%s@%s:%d/0", username, password, host, port)
	case "ollama":
		return fmt.Sprintf("http://%s:%d", host, port)
	default:
		return ""
	}
}
