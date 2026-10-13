package databases

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/admin"
)

var (
	ErrUnknownEngine = errors.New("unknown database engine (postgres, mongo, redis)")
	ErrQuotaExceeded = errors.New("лимит места тарифа исчерпан")
	ErrNotFound      = errors.New("database user not found")
)

// DBConfig — хосты/порты для строк подключения и admin-доступ для провижининга.
// Берётся из .env (см. .env.example).
type DBConfig struct {
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
}

type Engine struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DefaultPort string `json:"defaultPort"`
	Description string `json:"description"`
}

type CreatedUser struct {
	DbUser           DbUser
	Password         string
	Rows             []CredentialRow
	ConnectionString string
}

type CredentialRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Service struct {
	repo      *Repository
	cfg       DBConfig
	providers map[string]Provider
	logger    *zap.Logger
}

func NewService(repo *Repository, cfg DBConfig, logger *zap.Logger) *Service {
	return &Service{
		repo: repo,
		cfg:  cfg,
		providers: map[string]Provider{
			"postgres": postgresProvider{adminURL: cfg.PostgresAdminURL},
			"mongo":    mongoProvider{adminURL: cfg.MongoAdminURL},
			"redis":    redisProvider{addr: fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort), password: cfg.RedisPassword},
		},
		logger: logger,
	}
}

func (s *Service) Engines() []Engine {
	return []Engine{
		{ID: "postgres", Name: "PostgreSQL", DefaultPort: fmt.Sprint(s.cfg.PostgresPort), Description: "Реляционная база для приложений и сервисов."},
		{ID: "mongo", Name: "MongoDB", DefaultPort: fmt.Sprint(s.cfg.MongoPort), Description: "Документная база для гибких схем."},
		{ID: "redis", Name: "Redis", DefaultPort: fmt.Sprint(s.cfg.RedisPort), Description: "Кэш и очереди сообщений."},
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

func randomSuffix(n int) (string, error) {
	const digits = "0123456789"
	out := make([]byte, n)
	for i := range out {
		d, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		out[i] = digits[d.Int64()]
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
	return b.String()
}

// Usage возвращает занятое место в СУБД и лимит тарифа.
func (s *Service) Usage(ctx context.Context, owner string) (usedGB, limitGB float64) {
	plan := admin.FindPlan(s.repo.UserPlan(ctx, owner))
	return s.repo.DbUsageGB(ctx, owner), plan.DbStorageGB
}

func (s *Service) Mine(ctx context.Context, owner string) ([]DbUser, error) {
	return s.repo.ListByOwner(ctx, owner)
}

// Create заводит пользователя в живой СУБД: новый логин + случайный пароль.
// Строки подключения строятся из хостов/портов .env.
func (s *Service) Create(ctx context.Context, owner, dbType string) (*CreatedUser, error) {
	provider, ok := s.providers[dbType]
	if !ok {
		return nil, ErrUnknownEngine
	}
	plan := admin.FindPlan(s.repo.UserPlan(ctx, owner))
	if used := s.repo.StorageUsageGB(ctx, owner); used >= plan.DbStorageGB {
		return nil, ErrQuotaExceeded
	}
	suffix, err := randomSuffix(4)
	if err != nil {
		return nil, err
	}
	base := sanitizeLogin(owner)
	username := fmt.Sprintf("u_%s_%s", base, suffix)
	dbName := username
	password, err := randomPassword(20)
	if err != nil {
		return nil, err
	}
	var host string
	var port int
	switch dbType {
	case "postgres":
		host, port, dbName = s.cfg.PostgresHost, s.cfg.PostgresPort, s.cfg.PostgresName+"_"+base+"_"+suffix
	case "mongo":
		host, port, dbName = s.cfg.MongoHost, s.cfg.MongoPort, s.cfg.MongoName+"_"+base+"_"+suffix
	case "redis":
		host, port = s.cfg.RedisHost, s.cfg.RedisPort
	}
	if err := provider.CreateUser(ctx, dbName, username, password); err != nil {
		return nil, err
	}
	rec := &DbUser{
		ID: uuid.New(), OwnerLogin: owner, DbType: dbType,
		DbName: dbName, Username: username, Host: host, Port: port, SizeGB: 0,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		// Запись не сохранилась — откатываем пользователя в СУБД.
		if dropErr := provider.DropUser(ctx, dbName, username); dropErr != nil {
			s.logger.Warn("db user rollback failed", zap.String("db", dbType), zap.String("user", username), zap.Error(dropErr))
		}
		return nil, err
	}
	return &CreatedUser{DbUser: *rec, Password: password,
		Rows: buildRows(dbType, host, port, dbName, username, password),
		ConnectionString: buildConnString(dbType, host, port, dbName, username, password)}, nil
}

func buildRows(dbType, host string, port int, dbName, username, password string) []CredentialRow {
	switch dbType {
	case "redis":
		return []CredentialRow{
			{Label: "Хост", Value: host},
			{Label: "Порт", Value: fmt.Sprint(port)},
			{Label: "Пользователь", Value: username},
			{Label: "Пароль", Value: password},
		}
	default:
		return []CredentialRow{
			{Label: "Хост", Value: host},
			{Label: "Порт", Value: fmt.Sprint(port)},
			{Label: "База данных", Value: dbName},
			{Label: "Пользователь", Value: username},
			{Label: "Пароль", Value: password},
		}
	}
}

func buildConnString(dbType, host string, port int, dbName, username, password string) string {
	switch dbType {
	case "postgres":
		return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", username, password, host, port, dbName)
	case "mongo":
		return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", username, password, host, port, dbName)
	case "redis":
		return fmt.Sprintf("redis://%s:%s@%s:%d/0", username, password, host, port)
	default:
		return ""
	}
}

// Delete удаляет пользователя из СУБД и запись из каталога.
func (s *Service) Delete(ctx context.Context, owner string, id uuid.UUID) error {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if rec.OwnerLogin != owner {
		return ErrNotFound
	}
	provider, ok := s.providers[rec.DbType]
	if !ok {
		return ErrUnknownEngine
	}
	if err := provider.DropUser(ctx, rec.DbName, rec.Username); err != nil {
		s.logger.Warn("db drop user failed, record kept", zap.String("user", rec.Username), zap.Error(err))
		return err
	}
	return s.repo.Delete(ctx, id)
}
