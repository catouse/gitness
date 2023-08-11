// Copyright 2022 Harness Inc. All rights reserved.
// Use of this source code is governed by the Polyform Free Trial License
// that can be found in the LICENSE.md file for this repository.

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/gitness/internal/store"
	gitness_store "github.com/harness/gitness/store"
	"github.com/harness/gitness/store/database"
	"github.com/harness/gitness/store/database/dbtx"
	"github.com/harness/gitness/types"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

var _ store.ExecutionStore = (*executionStore)(nil)

// NewExecutionStore returns a new ExecutionStore.
func NewExecutionStore(db *sqlx.DB) *executionStore {
	return &executionStore{
		db: db,
	}
}

type executionStore struct {
	db *sqlx.DB
}

const (
	executionColumns = `
		execution_id
		,execution_pipeline_id
		,execution_repo_id
		,execution_trigger
		,execution_number
		,execution_parent
		,execution_status
		,execution_error
		,execution_event
		,execution_action
		,execution_link
		,execution_timestamp
		,execution_title
		,execution_message
		,execution_before
		,execution_after
		,execution_ref
		,execution_source_repo
		,execution_source
		,execution_target
		,execution_author
		,execution_author_name
		,execution_author_email
		,execution_author_avatar
		,execution_sender
		,execution_params
		,execution_cron
		,execution_deploy
		,execution_deploy_id
		,execution_debug
		,execution_started
		,execution_finished
		,execution_created
		,execution_updated
		,execution_version
	`
)

// Find returns an execution given a pipeline ID and an execution number.
func (s *executionStore) Find(ctx context.Context, pipelineID int64, executionNum int64) (*types.Execution, error) {
	const findQueryStmt = `
	SELECT` + executionColumns + `
	FROM executions
	WHERE execution_pipeline_id = $1 AND execution_number = $2`
	db := dbtx.GetAccessor(ctx, s.db)

	dst := new(types.Execution)
	if err := db.GetContext(ctx, dst, findQueryStmt, pipelineID, executionNum); err != nil {
		return nil, database.ProcessSQLErrorf(err, "Failed to find execution")
	}
	return dst, nil
}

// Create creates a new execution in the datastore.
func (s *executionStore) Create(ctx context.Context, execution *types.Execution) error {
	const executionInsertStmt = `
	INSERT INTO executions (
		execution_pipeline_id
		,execution_repo_id
		,execution_trigger
		,execution_number
		,execution_parent
		,execution_status
		,execution_error
		,execution_event
		,execution_action
		,execution_link
		,execution_timestamp
		,execution_title
		,execution_message
		,execution_before
		,execution_after
		,execution_ref
		,execution_source_repo
		,execution_source
		,execution_target
		,execution_author
		,execution_author_name
		,execution_author_email
		,execution_author_avatar
		,execution_sender
		,execution_params
		,execution_cron
		,execution_deploy
		,execution_deploy_id
		,execution_debug
		,execution_started
		,execution_finished
		,execution_created
		,execution_updated
		,execution_version
	) VALUES (
		:execution_pipeline_id
		,:execution_repo_id
		,:execution_trigger
		,:execution_number
		,:execution_parent
		,:execution_status
		,:execution_error
		,:execution_event
		,:execution_action
		,:execution_link
		,:execution_timestamp
		,:execution_title
		,:execution_message
		,:execution_before
		,:execution_after
		,:execution_ref
		,:execution_source_repo
		,:execution_source
		,:execution_target
		,:execution_author
		,:execution_author_name
		,:execution_author_email
		,:execution_author_avatar
		,:execution_sender
		,:execution_params
		,:execution_cron
		,:execution_deploy
		,:execution_deploy_id
		,:execution_debug
		,:execution_started
		,:execution_finished
		,:execution_created
		,:execution_updated
		,:execution_version
	) RETURNING execution_id`
	db := dbtx.GetAccessor(ctx, s.db)

	query, arg, err := db.BindNamed(executionInsertStmt, execution)
	if err != nil {
		return database.ProcessSQLErrorf(err, "Failed to bind execution object")
	}

	if err = db.QueryRowContext(ctx, query, arg...).Scan(&execution.ID); err != nil {
		return database.ProcessSQLErrorf(err, "Execution query failed")
	}

	return nil
}

// Update tries to update an execution in the datastore with optimistic locking.
func (s *executionStore) Update(ctx context.Context, e *types.Execution) error {
	const executionUpdateStmt = `
	UPDATE executions
	SET
		,execution_status = :execution_status
		,execution_error = :execution_error
		,execution_event = :execution_event
		,execution_started = :execution_started
		,execution_finished = :execution_finished
		,execution_updated = :execution_updated
		,execution_version = :execution_version
	WHERE execution_id = :execution_id AND execution_version = :execution_version - 1`
	updatedAt := time.Now()

	execution := *e

	execution.Version++
	execution.Updated = updatedAt.UnixMilli()

	db := dbtx.GetAccessor(ctx, s.db)

	query, arg, err := db.BindNamed(executionUpdateStmt, execution)
	if err != nil {
		return database.ProcessSQLErrorf(err, "Failed to bind execution object")
	}

	result, err := db.ExecContext(ctx, query, arg...)
	if err != nil {
		return database.ProcessSQLErrorf(err, "Failed to update execution")
	}

	count, err := result.RowsAffected()
	if err != nil {
		return database.ProcessSQLErrorf(err, "Failed to get number of updated rows")
	}

	if count == 0 {
		return gitness_store.ErrVersionConflict
	}

	e.Version = execution.Version
	e.Updated = execution.Updated
	return nil
}

// UpdateOptLock updates the pipeline using the optimistic locking mechanism.
func (s *executionStore) UpdateOptLock(ctx context.Context,
	execution *types.Execution,
	mutateFn func(execution *types.Execution) error) (*types.Execution, error) {
	for {
		dup := *execution

		err := mutateFn(&dup)
		if err != nil {
			return nil, err
		}

		err = s.Update(ctx, &dup)
		if err == nil {
			return &dup, nil
		}
		if !errors.Is(err, gitness_store.ErrVersionConflict) {
			return nil, err
		}

		execution, err = s.Find(ctx, execution.PipelineID, execution.Number)
		if err != nil {
			return nil, err
		}
	}
}

// List lists the executions for a given pipeline ID.
func (s *executionStore) List(
	ctx context.Context,
	pipelineID int64,
	pagination types.Pagination,
) ([]*types.Execution, error) {
	stmt := database.Builder.
		Select(executionColumns).
		From("executions").
		Where("execution_pipeline_id = ?", fmt.Sprint(pipelineID))

	stmt = stmt.Limit(database.Limit(pagination.Size))
	stmt = stmt.Offset(database.Offset(pagination.Page, pagination.Size))

	sql, args, err := stmt.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert query to sql")
	}

	db := dbtx.GetAccessor(ctx, s.db)

	dst := []*types.Execution{}
	if err = db.SelectContext(ctx, &dst, sql, args...); err != nil {
		return nil, database.ProcessSQLErrorf(err, "Failed executing custom list query")
	}

	return dst, nil
}

// Count of executions in a space.
func (s *executionStore) Count(ctx context.Context, pipelineID int64) (int64, error) {
	stmt := database.Builder.
		Select("count(*)").
		From("executions").
		Where("execution_pipeline_id = ?", pipelineID)

	sql, args, err := stmt.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "Failed to convert query to sql")
	}

	db := dbtx.GetAccessor(ctx, s.db)

	var count int64
	err = db.QueryRowContext(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, database.ProcessSQLErrorf(err, "Failed executing count query")
	}
	return count, nil
}

// Delete deletes an execution given a pipeline ID and an execution number.
func (s *executionStore) Delete(ctx context.Context, pipelineID int64, executionNum int64) error {
	const executionDeleteStmt = `
		DELETE FROM executions
		WHERE execution_pipeline_id = $1 AND execution_number = $2`

	db := dbtx.GetAccessor(ctx, s.db)

	if _, err := db.ExecContext(ctx, executionDeleteStmt, pipelineID, executionNum); err != nil {
		return database.ProcessSQLErrorf(err, "Could not delete execution")
	}

	return nil
}