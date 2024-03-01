package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nlewo/comin/internal/deployment"
	"github.com/nlewo/comin/internal/generation"
	"github.com/sirupsen/logrus"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*
var fs embed.FS

var generationColumns = "uuid, selected_remote_name, selected_branch_name, selected_commit_id, selected_commit_msg, selected_branch_is_testing, eval_started_at, eval_ended_at, build_started_at, build_ended_at, hostname, out_path, drv_path, evaluated_machine_id, status"

type Storable interface {
	Close()
	DeploymentInsert(d deployment.Deployment) error
	DeploymentGet(uuid string) (d deployment.Deployment, ok bool)
}

type Storage struct {
	db *sql.DB
}

func New(filepath string) (s Storage, err error) {
	s.db, err = sql.Open("sqlite3", filepath)

	if err != nil {
		logrus.Fatal(err)
	}

	err = runMigrateScripts(s.db)
	if err != nil {
		logrus.Fatal(err)
	}

	return
}

func (s Storage) Close() {
	s.db.Close()
}

func runMigrateScripts(db *sql.DB) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("Creating sqlite3 db driver failed %s", err)
	}

	d, err := iofs.New(fs, "migrations")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", d, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("Initializing db migration failed %s", err)
	}
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("Getting database schema version failed %s", err)
	}
	if dirty {
		logrus.Infof("The database schema version is %d and will be upgraded", version)
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("Migrating database failed %s", err)
	}
	version, _, err = m.Version()
	if err != nil {
		return fmt.Errorf("Getting database schema version failed %s", err)
	}
	logrus.Infof("The database schema version is %d", version)

	return nil
}

func (s Storage) DeploymentInsert(d deployment.Deployment) error {
	_, ok := s.GenerationGetByUuid(d.Generation.UUID)
	if !ok {
		err := s.GenerationInsert(d.Generation)
		if err != nil {
			return err
		}
	}
	var restartComin int
	if d.RestartComin {
		restartComin = 1
	}
	// FIXME: we should use ? instead of %s
	stmt := fmt.Sprintf(
		"INSERT INTO deployment(%s, %s, %s, %s, %s, %s, %s, %s) VALUES('%s', '%s', '%d', '%d', '%s', '%d', '%s', '%s');",
		"uuid",
		"generation_uuid",
		"start_at",
		"end_at",
		"error_msg",
		"restart_comin",
		"status",
		"operation",
		d.UUID,
		d.Generation.UUID,
		d.StartAt.Unix(),
		d.EndAt.Unix(),
		d.ErrorMsg,
		restartComin,
		deployment.StatusToString(d.Status),
		d.Operation,
	)
	_, err := s.db.Exec(stmt)
	return err
}

func (s Storage) DeploymentList(ctx context.Context, limit int) (d []deployment.Deployment, err error) {
	// FIXME: write a more efficient version!
	stmt := "SELECT uuid from deployment order by start_at desc limit ?;"
	rows, err := s.db.QueryContext(ctx, stmt, limit)
	if err != nil {
		return
	}
	defer rows.Close()
	d = make([]deployment.Deployment, 0)
	for rows.Next() {
		var uuid string
		if err = rows.Scan(&uuid); err != nil {
			return
		}
		t, ok := s.DeploymentGet(uuid)
		logrus.Debug(t)
		if ok {
			d = append(d, t)
		}
	}
	if err = rows.Err(); err != nil {
		return
	}
	return
}

func (s Storage) DeploymentGet(uuid string) (d deployment.Deployment, ok bool) {
	var restartComin, startAt, endAt int64
	var gUuid, status string
	stmt := "SELECT uuid, generation_uuid, start_at, end_at, error_msg, restart_comin, status, operation from deployment where uuid=?;"
	err := s.db.QueryRow(stmt, uuid).Scan(&d.UUID, &gUuid, &startAt, &endAt, &d.ErrorMsg, &restartComin, &status, &d.Operation)
	d.Status = deployment.StatusFromString(status)
	if restartComin == 1 {
		d.RestartComin = true
	}
	d.StartAt = time.Unix(startAt, 0).UTC()
	d.EndAt = time.Unix(endAt, 0).UTC()

	g, ok := s.GenerationGetByUuid(gUuid)
	if !ok {
		return
	}
	d.Generation = g

	if err == sql.ErrNoRows {
		return d, ok
	}
	if err != nil {
		logrus.Error(err)
	}
	return d, true
}

func (s Storage) GenerationGetByUuid(uuid string) (g generation.Generation, ok bool) {
	var evalStartedAt, evalEndedAt, buildStartedAt, buildEndedAt, isTesting int64
	var status string
	stmt := "SELECT " + generationColumns + " from generation where uuid=?;"
	err := s.db.QueryRow(stmt, uuid).Scan(&g.UUID, &g.SelectedRemoteName, &g.SelectedBranchName, &g.SelectedCommitId, &g.SelectedCommitMsg, &isTesting, &evalStartedAt, &evalEndedAt, &buildStartedAt, &buildEndedAt, &g.Hostname, &g.OutPath, &g.DrvPath, &g.EvalMachineId, &status)
	g.EvalStartedAt = time.Unix(evalStartedAt, 0).UTC()
	g.EvalEndedAt = time.Unix(evalEndedAt, 0).UTC()
	g.BuildStartedAt = time.Unix(buildStartedAt, 0).UTC()
	g.BuildEndedAt = time.Unix(buildEndedAt, 0).UTC()
	g.Status = generation.StatusFromString(status)

	if err == sql.ErrNoRows {
		return g, ok
	}
	return g, true
}

func (s Storage) GenerationList(ctx context.Context) (generations []generation.Generation, err error) {
	var evalStartedAt, evalEndedAt, buildStartedAt, buildEndedAt, isTesting int64
	var status string
	stmt := "SELECT " + generationColumns + " from generation;"
	rows, err := s.db.QueryContext(ctx, stmt)
	if err != nil {
		return
	}
	defer rows.Close()
	generations = make([]generation.Generation, 0)
	for rows.Next() {
		var g generation.Generation
		if err = rows.Scan(&g.UUID, &g.SelectedRemoteName, &g.SelectedBranchName, &g.SelectedCommitId, &g.SelectedCommitMsg, &isTesting, &evalStartedAt, &evalEndedAt, &buildStartedAt, &buildEndedAt, &g.Hostname, &g.OutPath, &g.DrvPath, &g.EvalMachineId, &status); err != nil {
			return
		}
		g.EvalStartedAt = time.Unix(evalStartedAt, 0).UTC()
		g.EvalEndedAt = time.Unix(evalEndedAt, 0).UTC()
		g.BuildStartedAt = time.Unix(buildStartedAt, 0).UTC()
		g.BuildEndedAt = time.Unix(buildEndedAt, 0).UTC()
		g.Status = generation.StatusFromString(status)
		generations = append(generations, g)
	}
	if err = rows.Err(); err != nil {
		return
	}
	return
}

func (s Storage) GenerationInsert(g generation.Generation) error {
	var isTesting bool
	stmt := fmt.Sprintf(
		"INSERT INTO generation(%s) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);",
		generationColumns,
	)
	if g.SelectedBranchIsTesting {
		isTesting = true
	}
	_, err := s.db.Exec(stmt,
		g.UUID,
		g.SelectedRemoteName,
		g.SelectedBranchName,
		g.SelectedCommitId,
		g.SelectedCommitMsg,
		isTesting,
		g.EvalStartedAt.Unix(),
		g.EvalEndedAt.Unix(),
		g.BuildStartedAt.Unix(),
		g.BuildEndedAt.Unix(),
		g.Hostname,
		g.OutPath,
		g.DrvPath,
		g.EvalMachineId,
		generation.StatusToString(g.Status),
	)
	return err
}
