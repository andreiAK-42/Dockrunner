package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Как устроено логирование:
//
// - Все логи (Info и выше, уровень — через LOG_LEVEL) пишутся в файл logs/app.log.
// - В терминал попадает только важное:
//   - Warn и выше (ошибки, падения, варнинги),
//   - Info, помеченные через Important (запуск/остановка сервера).
//
// Шумные Info (HTTP-запросы, heartbeat) в терминал не попадают, только в файл.
// Ротация файла не делается — отдана logrotate/ОС (файл открывается на дозапись).

const (
	// ImportantName — имя под-логгера, чьи Info тоже идут в терминал.
	ImportantName = "important"

	// AppFileName — имя файла со всеми логами внутри LogDir.
	AppFileName = "app.log"
)

// Config — настройки логгера.
type Config struct {
	// Env — "production" или другое. В production в файл идёт JSON.
	Env string
	// Dir — каталог для app.log. Пусто — файлы не пишутся, всё как раньше в терминал.
	Dir string
	// Level — минимальный уровень для файла. Пусто — Info.
	// Перекрывается переменной окружения LOG_LEVEL, если она задана.
	Level string
}

// New — как раньше: только терминал, без файлов.
// Оставлен для совместимости.
func New(env string) (*zap.Logger, error) {
	return NewWithConfig(Config{Env: env})
}

// NewWithConfig собирает логгер: файл (всё) + терминал (только важное).
func NewWithConfig(cfg Config) (*zap.Logger, error) {
	level := parseLevel(cfg.Level)

	fileEncoder := buildFileEncoder(cfg.Env)
	consoleEncoder := buildConsoleEncoder(cfg.Env)

	if cfg.Dir == "" {
		return buildConsoleOnly(consoleEncoder, level), nil
	}

	fileWriter, err := openAppLog(cfg.Dir)
	if err != nil {
		return nil, err
	}

	fileCore := zapcore.NewCore(fileEncoder, fileWriter, level)
	consoleCore := &consoleFilterCore{
		Core:          zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stderr), zapcore.DebugLevel),
		importantName: ImportantName,
	}

	tee := zapcore.NewTee(fileCore, consoleCore)
	return zap.New(tee, zap.AddCaller()), nil
}

// Important возвращает под-логгер, чьи Info тоже попадают в терминал.
// Использовать только для событий запуска/остановки.
func Important(log *zap.Logger) *zap.Logger {
	return log.Named(ImportantName)
}

func parseLevel(level string) zapcore.Level {
	if level == "" {
		level = os.Getenv("LOG_LEVEL")
	}
	if level == "" {
		return zapcore.InfoLevel
	}
	parsed, err := zapcore.ParseLevel(level)
	if err != nil {
		return zapcore.InfoLevel
	}
	return parsed
}

func buildFileEncoder(env string) zapcore.Encoder {
	if env == "production" {
		fileCfg := zap.NewProductionEncoderConfig()
		fileCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		return zapcore.NewJSONEncoder(fileCfg)
	}
	return zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
}

func buildConsoleEncoder(env string) zapcore.Encoder {
	if env == "production" {
		consoleCfg := zap.NewProductionEncoderConfig()
		consoleCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		return zapcore.NewConsoleEncoder(consoleCfg)
	}
	devCfg := zap.NewDevelopmentEncoderConfig()
	devCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zapcore.NewConsoleEncoder(devCfg)
}

func buildConsoleOnly(encoder zapcore.Encoder, level zapcore.Level) *zap.Logger {
	return zap.New(
		zapcore.NewCore(encoder, zapcore.Lock(os.Stderr), level),
		zap.AddCaller(),
	)
}

func openAppLog(dir string) (zapcore.WriteSyncer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, AppFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return zapcore.AddSync(file), nil
}

// consoleFilterCore пропускает в терминал только важное:
// Warn и выше всегда, Info — только от под-логгера Important.
type consoleFilterCore struct {
	zapcore.Core
	importantName string
}

func (c *consoleFilterCore) With(fields []zapcore.Field) zapcore.Core {
	return &consoleFilterCore{Core: c.Core.With(fields), importantName: c.importantName}
}

func (c *consoleFilterCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if entry.Level >= zapcore.WarnLevel || entry.LoggerName == c.importantName {
		return c.Core.Check(entry, checked)
	}
	return checked
}
