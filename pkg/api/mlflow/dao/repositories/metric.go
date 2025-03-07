package repositories

import (
	"context"
	"database/sql"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api/request"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
	"github.com/G-Research/fasttrackml/pkg/database"
)

const (
	MetricHistoriesDefaultLimit   = 10000000
	MetricHistoryBulkDefaultLimit = 25000
)

// MetricRepositoryProvider provides an interface to work with models.Metric entity.
type MetricRepositoryProvider interface {
	BaseRepositoryProvider
	// CreateBatch creates []models.Metric entities in batch.
	CreateBatch(ctx context.Context, run *models.Run, batchSize int, params []models.Metric) error
	// GetMetricHistories returns metric histories by request parameters.
	GetMetricHistories(
		ctx context.Context,
		namespaceID uint,
		experimentIDs []string, runIDs []string, metricKeys []string,
		viewType request.ViewType,
		limit int32,
		jsonPathValueMap map[string]string,
	) (*sql.Rows, func(*sql.Rows, interface{}) error, error)
	// GetMetricHistoryBulk returns metrics history bulk.
	GetMetricHistoryBulk(
		ctx context.Context, namespaceID uint, runIDs []string, key string, limit int,
	) ([]models.Metric, error)
	// GetMetricHistoryByRunIDAndKey returns metrics history by RunID and Key.
	GetMetricHistoryByRunIDAndKey(ctx context.Context, runID, key string) ([]models.Metric, error)
}

// MetricRepository repository to work with models.Metric entity.
type MetricRepository struct {
	BaseRepository
}

// NewMetricRepository creates repository to work with models.Metric entity.
func NewMetricRepository(db *gorm.DB) *MetricRepository {
	return &MetricRepository{
		BaseRepository{
			db: db,
		},
	}
}

// CreateBatch creates []models.Metric entities in batch.
// TODO:get back and fix `gocyclo` problem.
//
//nolint:gocyclo
func (r MetricRepository) CreateBatch(
	ctx context.Context, run *models.Run, batchSize int, metrics []models.Metric,
) error {
	if len(metrics) == 0 {
		return nil
	}

	metricKeysMap := make(map[string]any)
	for _, m := range metrics {
		metricKeysMap[m.Key] = nil
	}
	metricKeys := make([]string, 0, len(metricKeysMap))
	for k := range metricKeysMap {
		metricKeys = append(metricKeys, k)
	}

	// get the latest metrics by requested Run ID and metric keys.
	lastMetrics, err := r.getLatestMetricsByRunIDAndKeys(ctx, run.ID, metricKeys)
	if err != nil {
		return eris.Wrap(err, "error getting latest metrics")
	}

	lastIters := make(map[string]int64)
	for _, lastMetric := range lastMetrics {
		lastIters[lastMetric.UniqueKey()] = lastMetric.LastIter
	}
	allContexts := make([]*models.Context, len(metrics))
	uniqueContexts := make([]*models.Context, 0, len(metrics))
	contextProcessed := make(map[string]*models.Context)
	latestMetrics := make(map[string]models.LatestMetric)
	for n := range metrics {
		ctxHash := metrics[n].Context.GetJsonHash()
		ctxRef, ok := contextProcessed[ctxHash]
		if ok {
			allContexts[n] = ctxRef
		} else {
			uniqueContexts = append(uniqueContexts, &metrics[n].Context)
			allContexts[n] = &metrics[n].Context
			contextProcessed[ctxHash] = &metrics[n].Context
		}
	}

	if err := r.db.WithContext(ctx).Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "json"}},
			UpdateAll: true,
		},
	).CreateInBatches(&uniqueContexts, batchSize).Error; err != nil {
		return eris.Wrapf(err, "error creating contexts")
	}

	for n := range metrics {
		metrics[n].ContextID = allContexts[n].ID
		metrics[n].Context = *allContexts[n]
		metrics[n].Iter = lastIters[metrics[n].UniqueKey()] + 1
		lastIters[metrics[n].UniqueKey()] = metrics[n].Iter
		lm, ok := latestMetrics[metrics[n].UniqueKey()]
		if !ok ||
			metrics[n].Step > lm.Step ||
			(metrics[n].Step == lm.Step && metrics[n].Timestamp > lm.Timestamp) ||
			(metrics[n].Step == lm.Step && metrics[n].Timestamp == lm.Timestamp && metrics[n].Value > lm.Value) {
			latestMetrics[metrics[n].UniqueKey()] = models.LatestMetric{
				RunID:     metrics[n].RunID,
				Key:       metrics[n].Key,
				Value:     metrics[n].Value,
				Timestamp: metrics[n].Timestamp,
				Step:      metrics[n].Step,
				IsNan:     metrics[n].IsNan,
				LastIter:  metrics[n].Iter,
				ContextID: allContexts[n].ID,
				Context:   *allContexts[n],
			}
		}
	}

	if err := r.db.WithContext(ctx).Clauses(
		clause.OnConflict{DoNothing: true},
	).CreateInBatches(&metrics, batchSize).Error; err != nil {
		return eris.Wrapf(err, "error creating metrics for run: %s", run.ID)
	}

	// TODO update latest metrics in the background?
	currentLatestMetricsMap := make(map[string]models.LatestMetric, len(latestMetrics))
	for k, m := range latestMetrics {
		currentLatestMetricsMap[k] = m
	}

	updatedLatestMetrics := make([]models.LatestMetric, 0, len(latestMetrics))
	for k, m := range latestMetrics {
		lm, ok := currentLatestMetricsMap[k]
		if !ok ||
			m.Step > lm.Step ||
			(m.Step == lm.Step && m.Timestamp > lm.Timestamp) ||
			(m.Step == lm.Step && m.Timestamp == lm.Timestamp && m.Value > lm.Value) {
			updatedLatestMetrics = append(updatedLatestMetrics, m)
		} else {
			lm.LastIter = lastIters[k]
			updatedLatestMetrics = append(updatedLatestMetrics, lm)
		}
	}

	if len(updatedLatestMetrics) > 0 {
		if err := r.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "run_uuid"}, {Name: "key"}, {Name: "context_id"}},
			UpdateAll: true,
		}).Create(&updatedLatestMetrics).Error; err != nil {
			return eris.Wrapf(err, "error updating latest metrics for run: %s", run.ID)
		}
	}
	return nil
}

// GetMetricHistories returns metric histories by request parameters.
// TODO think about to use interface instead of underlying type for -> func(*sql.Rows, interface{})
func (r MetricRepository) GetMetricHistories(
	ctx context.Context,
	namespaceID uint,
	experimentIDs []string, runIDs []string, metricKeys []string,
	viewType request.ViewType,
	limit int32,
	jsonPathValueMap map[string]string,
) (*sql.Rows, func(*sql.Rows, interface{}) error, error) {
	// if experimentIDs has been provided then firstly get the runs by provided experimentIDs.
	if len(experimentIDs) > 0 {
		query := r.db.WithContext(ctx).Model(
			&database.Run{},
		).Joins(
			"INNER JOIN experiments ON experiments.experiment_id = runs.experiment_id AND experiments.namespace_id = ?",
			namespaceID,
		).Where(
			"runs.experiment_id IN ?", experimentIDs,
		)

		switch viewType {
		case request.ViewTypeActiveOnly, "":
			query.Where("runs.lifecycle_stage IN ?", []models.LifecycleStage{
				models.LifecycleStageActive,
			})
		case request.ViewTypeDeletedOnly:
			query.Where("runs.lifecycle_stage IN ?", []models.LifecycleStage{
				models.LifecycleStageDeleted,
			})
		case request.ViewTypeAll:
			query.Where("runs.lifecycle_stage IN ?", []models.LifecycleStage{
				models.LifecycleStageActive,
				models.LifecycleStageDeleted,
			})
		}
		if err := query.Pluck("run_uuid", &runIDs).Error; err != nil {
			return nil, nil, eris.Wrapf(
				err, "error getting runs by experimentIDs: %v, viewType: %s", experimentIDs, viewType,
			)
		}
	}

	// if experimentIDs has been provided then runIDs contains values from previous step,
	// otherwise runIDs may or may not contain values.
	query := r.db.WithContext(ctx).Model(
		&database.Metric{},
	).Where(
		"metrics.run_uuid IN ?", runIDs,
	).Joins(
		"JOIN runs on runs.run_uuid = metrics.run_uuid",
	).Joins(
		"INNER JOIN experiments ON experiments.experiment_id = runs.experiment_id AND experiments.namespace_id = ?",
		namespaceID,
	).Joins(
		"Context",
	).Order(
		"runs.start_time DESC",
	).Order(
		"metrics.run_uuid",
	).Order(
		"metrics.key",
	).Order(
		"metrics.step",
	).Order(
		"metrics.timestamp",
	).Order(
		"metrics.value",
	)

	if limit == 0 {
		limit = MetricHistoriesDefaultLimit
	}
	query.Limit(int(limit))

	if len(metricKeys) > 0 {
		query.Where("metrics.key IN ?", metricKeys)
	}

	if len(jsonPathValueMap) > 0 {
		query.Joins("LEFT JOIN contexts on metrics.context_id = contexts.id")
		sql, args := BuildJsonCondition(query.Dialector.Name(), "contexts.json", jsonPathValueMap)
		query.Where(sql, args...)
	}

	rows, err := query.Rows()
	if err != nil {
		return nil, nil, eris.Wrapf(
			err, "error getting metrics by experimentIDs: %v, runIDs: %v, metricKeys: %v, viewType: %s",
			experimentIDs,
			runIDs,
			metricKeys,
			viewType,
		)
	}
	return rows, r.db.ScanRows, nil
}

// getLatestMetricsByRunIDAndKeys returns the latest metrics by requested Run ID and keys.
func (r MetricRepository) getLatestMetricsByRunIDAndKeys(
	ctx context.Context, runID string, keys []string,
) ([]models.LatestMetric, error) {
	var metrics []models.LatestMetric
	if err := r.db.WithContext(ctx).Where(
		"run_uuid = ?", runID,
	).Where(
		"key IN ?", keys,
	).Find(&metrics).Error; err != nil {
		return nil, eris.Wrapf(err, "error getting latest metrics by run id: %s and keys: %v", runID, keys)
	}
	return metrics, nil
}

// GetMetricHistoryByRunIDAndKey returns metrics history by RunID and Key.
func (r MetricRepository) GetMetricHistoryByRunIDAndKey(
	ctx context.Context, runID, key string,
) ([]models.Metric, error) {
	var metrics []models.Metric
	if err := r.db.WithContext(ctx).Preload("Context").Where(
		"run_uuid = ?", runID,
	).Where(
		"key = ?", key,
	).Find(&metrics).Error; err != nil {
		return nil, eris.Wrapf(err, "error getting metric history by run id: %s and key: %s", runID, key)
	}
	return metrics, nil
}

// GetMetricHistoryBulk returns metrics history bulk.
func (r MetricRepository) GetMetricHistoryBulk(
	ctx context.Context, namespaceID uint, runIDs []string, key string, limit int,
) ([]models.Metric, error) {
	var metrics []models.Metric
	query := r.db.WithContext(ctx).Where(
		"runs.run_uuid IN ?", runIDs,
	).Joins(
		"LEFT JOIN runs ON runs.run_uuid = metrics.run_uuid",
	).Joins(
		"INNER JOIN experiments ON experiments.experiment_id = runs.experiment_id AND experiments.namespace_id = ?",
		namespaceID,
	).Where(
		"key = ?", key,
	).Order(
		"metrics.run_uuid",
	).Order(
		"metrics.timestamp",
	).Order(
		"metrics.step",
	).Order(
		"metrics.value",
	)

	if limit == 0 {
		limit = MetricHistoryBulkDefaultLimit
	}
	query.Limit(limit)

	if err := query.Find(
		&metrics,
	).Error; err != nil {
		return nil, eris.Wrapf(err, "error getting metric history by run ids: %v and key: %s", runIDs, key)
	}
	return metrics, nil
}
