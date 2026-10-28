package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Deployment struct {
	ID        string
	AppID     string
	AppName   string
	Owner     string
	RepoURL   string
	Name      string
	Image     string
	Container string
	Status    string
	Ports     string
	Volumes   string
	Env       string
	CPUCores  float64
	MemoryGB  float64
	DiskGB    float64 // лимит диска (overlay2.size)
	// ParentID/ParentApp/Service — для сервис-контейнеров compose
	// (одиночные деплои хранят пустые значения).
	ParentID  string
	ParentApp string
	Service   string
	Branch    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ErrNoDeployment means the deployment is unknown to this agent —
// retrying the task cannot fix it.
var ErrNoDeployment = errors.New("deployment not found")

func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureColumns(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS deployments (
	id TEXT PRIMARY KEY,
	app_id TEXT NOT NULL,
	app_name TEXT NOT NULL,
	owner TEXT NOT NULL DEFAULT '',
	repo_url TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	image TEXT NOT NULL,
	container TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'created',
	ports TEXT NOT NULL DEFAULT '[]',
	volumes TEXT NOT NULL DEFAULT '[]',
	env TEXT NOT NULL DEFAULT '{}',
	branch TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_deployments_app ON deployments(app_id);
`

func ensureColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(deployments)`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return err
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, col := range []string{"owner", "repo_url", "name"} {
		if !cols[col] {
			if _, err := db.Exec(`ALTER TABLE deployments ADD COLUMN ` + col + ` TEXT NOT NULL DEFAULT ''`); err != nil {
				return err
			}
		}
	}
	for _, col := range []string{"cpu_cores", "memory_gb", "disk_gb"} {
		if !cols[col] {
			if _, err := db.Exec(`ALTER TABLE deployments ADD COLUMN ` + col + ` REAL NOT NULL DEFAULT 0`); err != nil {
				return err
			}
		}
	}
	for _, col := range []string{"parent_id", "parent_app", "service"} {
		if !cols[col] {
			if _, err := db.Exec(`ALTER TABLE deployments ADD COLUMN ` + col + ` TEXT NOT NULL DEFAULT ''`); err != nil {
				return err
			}
		}
	}
	for _, col := range []string{"branch"} {
		if !cols[col] {
			if _, err := db.Exec(`ALTER TABLE deployments ADD COLUMN ` + col + ` TEXT NOT NULL DEFAULT ''`); err != nil {
				return err
			}
		}
	}
	return nil
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func SaveDeployment(db *sql.DB, d Deployment) error {
	_, err := db.Exec(`INSERT INTO deployments
		(id, app_id, app_name, owner, repo_url, name, image, container, status, ports, volumes, env, cpu_cores, memory_gb, disk_gb, parent_id, parent_app, service, branch, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		app_name=excluded.app_name, owner=excluded.owner, repo_url=excluded.repo_url,
		name=excluded.name, image=excluded.image, container=excluded.container,
		status=excluded.status, ports=excluded.ports, volumes=excluded.volumes,
		env=excluded.env, cpu_cores=excluded.cpu_cores, memory_gb=excluded.memory_gb, disk_gb=excluded.disk_gb,
		parent_id=excluded.parent_id, parent_app=excluded.parent_app, service=excluded.service, branch=excluded.branch,
		updated_at=excluded.updated_at`,
		d.ID, d.AppID, d.AppName, d.Owner, d.RepoURL, d.Name, d.Image, d.Container, d.Status,
		d.Ports, d.Volumes, d.Env, d.CPUCores, d.MemoryGB, d.DiskGB,
		d.ParentID, d.ParentApp, d.Service, d.Branch, now(), now())
	return err
}

func scanDeployment(row interface {
	Scan(dest ...any) error
}, d *Deployment, created, updated *string) error {
	return row.Scan(&d.ID, &d.AppID, &d.AppName, &d.Owner, &d.RepoURL, &d.Name,
		&d.Image, &d.Container, &d.Status, &d.Ports, &d.Volumes, &d.Env,
		&d.CPUCores, &d.MemoryGB, &d.DiskGB, &d.ParentID, &d.ParentApp, &d.Service, &d.Branch, created, updated)
}

func parseDeploymentTime(d *Deployment, created, updated string) {
	d.CreatedAt, _ = time.Parse(time.RFC3339, created)
	d.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
}

func GetDeployment(db *sql.DB, id string) (*Deployment, error) {
	var d Deployment
	var created, updated string
	err := scanDeployment(db.QueryRow(`SELECT id, app_id, app_name, owner, repo_url, name,
		image, container, status, ports, volumes, env, cpu_cores, memory_gb, parent_id, parent_app, service, branch, created_at, updated_at FROM deployments WHERE id = ?`, id), &d, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoDeployment
	}
	if err != nil {
		return nil, err
	}
	parseDeploymentTime(&d, created, updated)
	return &d, nil
}

func ListDeployments(db *sql.DB) ([]Deployment, error) {
	rows, err := db.Query(`SELECT id, app_id, app_name, owner, repo_url, name,
		image, container, status, ports, volumes, env, cpu_cores, memory_gb, parent_id, parent_app, service, branch, created_at, updated_at FROM deployments ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []Deployment{}
	for rows.Next() {
		var d Deployment
		var created, updated string
		if err := scanDeployment(rows, &d, &created, &updated); err != nil {
			return nil, err
		}
		parseDeploymentTime(&d, created, updated)
		out = append(out, d)
	}
	return out, rows.Err()
}

func UpdateDeploymentStatus(db *sql.DB, id, status, container string) error {
	_, err := db.Exec(`UPDATE deployments SET status = ?, container = ?, updated_at = ? WHERE id = ?`,
		status, container, now(), id)
	return err
}

func DeleteDeployment(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM deployments WHERE id = ?`, id)
	return err
}
