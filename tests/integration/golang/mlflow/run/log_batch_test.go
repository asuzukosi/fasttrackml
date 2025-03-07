package run

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api/request"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
	"github.com/G-Research/fasttrackml/tests/integration/golang/helpers"
)

type LogBatchTestSuite struct {
	helpers.BaseTestSuite
}

func TestLogBatchTestSuite(t *testing.T) {
	suite.Run(t, new(LogBatchTestSuite))
}

func (s *LogBatchTestSuite) TestTags_Ok() {
	run, err := s.RunFixtures.CreateRun(context.Background(), &models.Run{
		ID:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		ExperimentID:   *s.DefaultExperiment.ID,
		SourceType:     "JOB",
		LifecycleStage: models.LifecycleStageActive,
		Status:         models.StatusRunning,
	})
	s.Require().Nil(err)

	tests := []struct {
		name    string
		request *request.LogBatchRequest
	}{
		{
			name: "LogOne",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Tags: []request.TagPartialRequest{
					{
						Key:   "key1",
						Value: "value1",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			resp := map[string]any{}
			s.Require().Nil(
				s.MlflowClient().WithMethod(
					http.MethodPost,
				).WithRequest(
					tt.request,
				).WithResponse(
					&resp,
				).DoRequest(
					"%s%s", mlflow.RunsRoutePrefix, mlflow.RunsLogBatchRoute,
				),
			)
			s.Empty(resp)
		})
	}
}

func (s *LogBatchTestSuite) TestParams_Ok() {
	run, err := s.RunFixtures.CreateRun(context.Background(), &models.Run{
		ID:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		ExperimentID:   *s.DefaultExperiment.ID,
		SourceType:     "JOB",
		LifecycleStage: models.LifecycleStageActive,
		Status:         models.StatusRunning,
	})
	s.Require().Nil(err)

	// create preexisting param (other batch) for conflict testing
	_, err = s.ParamFixtures.CreateParam(context.Background(), &models.Param{
		RunID: run.ID,
		Key:   "key1",
		Value: "value1",
	})
	s.Require().Nil(err)

	tests := []struct {
		name    string
		request *request.LogBatchRequest
	}{
		{
			name: "LogOne",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Params: []request.ParamPartialRequest{
					{
						Key:   "key1",
						Value: "value1",
					},
				},
			},
		},
		{
			name: "LogDuplicateSeparateBatch",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Params: []request.ParamPartialRequest{
					{
						Key:   "key1",
						Value: "value1",
					},
				},
			},
		},
		{
			name: "LogDuplicateSameBatch",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Params: []request.ParamPartialRequest{
					{
						Key:   "key2",
						Value: "value2",
					},
					{
						Key:   "key2",
						Value: "value2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			resp := map[string]any{}
			s.Require().Nil(
				s.MlflowClient().WithMethod(
					http.MethodPost,
				).WithRequest(
					tt.request,
				).WithResponse(
					&resp,
				).DoRequest(
					"%s%s", mlflow.RunsRoutePrefix, mlflow.RunsLogBatchRoute,
				),
			)
			s.Empty(resp)

			// verify params are inserted
			params, err := s.ParamFixtures.GetParamsByRunID(context.Background(), run.ID)
			s.Require().Nil(err)
			for _, param := range tt.request.Params {
				s.Contains(params, models.Param{Key: param.Key, Value: param.Value, RunID: run.ID})
			}
		})
	}
}

func (s *LogBatchTestSuite) TestMetrics_Ok() {
	run, err := s.RunFixtures.CreateRun(context.Background(), &models.Run{
		ID:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		ExperimentID:   *s.DefaultExperiment.ID,
		SourceType:     "JOB",
		LifecycleStage: models.LifecycleStageActive,
		Status:         models.StatusRunning,
	})
	s.Require().Nil(err)

	tests := []struct {
		name                  string
		request               *request.LogBatchRequest
		latestMetricIteration map[string]int64
		latestMetricKeyCount  map[string]int
	}{
		{
			name: "LogOne",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Metrics: []request.MetricPartialRequest{
					{
						Key:       "key0",
						Value:     1.0,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key1": "value1",
							"key2": 2,
						},
					},
				},
			},
			latestMetricIteration: map[string]int64{
				"key0": 1,
			},
		},
		{
			name: "LogSeveral",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Metrics: []request.MetricPartialRequest{
					{
						Key:       "key1",
						Value:     1.1,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key1": "value1",
							"key2": 2,
						},
					},
					{
						Key:       "key2",
						Value:     1.1,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key3": "value3",
						},
					},
					{
						Key:       "key1",
						Value:     1.1,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key1": "value1",
							"key2": 2,
						},
					},
					{
						Key:       "key2",
						Value:     1.2,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key3": "value3",
						},
					},
					{
						Key:       "key1",
						Value:     1.3,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key1": "value1",
							"key2": 2,
						},
					},
				},
			},
			latestMetricIteration: map[string]int64{
				"key1": 3,
				"key2": 2,
			},
		},
		{
			name: "LogDuplicateSameContext",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Metrics: []request.MetricPartialRequest{
					{
						Key:       "key3",
						Value:     1.0,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key3": "value3",
						},
					},
					{
						Key:       "key3",
						Value:     1.0,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key3": "value3",
						},
					},
				},
			},
			latestMetricIteration: map[string]int64{
				"key3": 2,
			},
			latestMetricKeyCount: map[string]int{
				"key3": 1,
			},
		},
		{
			name: "LogDuplicateDifferentContext",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Metrics: []request.MetricPartialRequest{
					{
						Key:       "key4",
						Value:     1.0,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key3": "value3",
						},
					},
					{
						Key:       "key4",
						Value:     1.0,
						Timestamp: 1687325991,
						Step:      1,
						Context: map[string]any{
							"key4": "value4",
						},
					},
				},
			},
			latestMetricIteration: map[string]int64{
				"key4": 1,
			},
			latestMetricKeyCount: map[string]int{
				"key4": 2,
			},
		},
		{
			name: "LogMany",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Metrics: func() []request.MetricPartialRequest {
					metrics := make([]request.MetricPartialRequest, 100*1000)
					for k := 0; k < 100; k++ {
						key := fmt.Sprintf("many%d", k)
						for i := 0; i < 1000; i++ {
							metrics[k*1000+i] = request.MetricPartialRequest{
								Key:       key,
								Value:     float64(i) + 0.1,
								Timestamp: 1687325991,
								Step:      1,
								Context: map[string]any{
									"key1": "value1",
								},
							}
						}
					}
					return metrics
				}(),
			},
			latestMetricIteration: func() map[string]int64 {
				metrics := make(map[string]int64, 100)
				for k := 0; k < 100; k++ {
					key := fmt.Sprintf("many%d", k)
					metrics[key] = 1000
				}
				return metrics
			}(),
		},
		{
			name: "LogNaNValue",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Metrics: []request.MetricPartialRequest{
					{
						Key:       "key4",
						Value:     "NaN",
						Timestamp: 1687325991,
						Step:      1,
					},
				},
			},
			latestMetricIteration: map[string]int64{
				"key4": 1,
			},
		},
		{
			name: "LogPositiveInfinityValue",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Metrics: []request.MetricPartialRequest{
					{
						Key:       "key5",
						Value:     "Infinity",
						Timestamp: 1687325991,
						Step:      1,
					},
				},
			},
			latestMetricIteration: map[string]int64{
				"key5": 1,
			},
		},
		{
			name: "LogNegativeInfinityValue",
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Metrics: []request.MetricPartialRequest{
					{
						Key:       "key6",
						Value:     "-Infinity",
						Timestamp: 1687325991,
						Step:      1,
					},
				},
			},
			latestMetricIteration: map[string]int64{
				"key6": 1,
			},
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			// do actual call to API.
			resp := map[string]any{}
			s.Require().Nil(
				s.MlflowClient().WithMethod(
					http.MethodPost,
				).WithRequest(
					tt.request,
				).WithResponse(
					&resp,
				).DoRequest(
					"%s%s", mlflow.RunsRoutePrefix, mlflow.RunsLogBatchRoute,
				),
			)
			s.Empty(resp)

			// make sure that `iter` and `last_iter` for each metric has been updated correctly.
			for key, iteration := range tt.latestMetricIteration {
				lastMetric, err := s.MetricFixtures.GetLatestMetricByKey(context.Background(), key)
				s.Require().Nil(err)
				s.Equal(iteration, lastMetric.LastIter)
			}
			for key, count := range tt.latestMetricKeyCount {
				latestMetrics, err := s.MetricFixtures.GetLatestMetricsByKey(context.Background(), key)
				s.Require().Nil(err)
				s.Equal(count, len(latestMetrics))
			}
			for _, metric := range tt.request.Metrics {
				if metric.Context != nil {
					metricContextJson, err := json.Marshal(metric.Context)
					s.Require().Nil(err)
					context, err := s.ContextFixtures.GetContextByJSON(context.Background(), string(metricContextJson))
					s.Require().Nil(err)
					s.Require().NotNil(context)
				}
			}
		})
	}
}

func (s *LogBatchTestSuite) Test_Error() {
	run, err := s.RunFixtures.CreateRun(context.Background(), &models.Run{
		ID:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		ExperimentID:   *s.DefaultExperiment.ID,
		SourceType:     "JOB",
		LifecycleStage: models.LifecycleStageActive,
		Status:         models.StatusRunning,
	})
	s.Require().Nil(err)

	testData := []struct {
		name    string
		error   *api.ErrorResponse
		request *request.LogBatchRequest
	}{
		{
			name:    "MissingRunIDFails",
			error:   api.NewInvalidParameterValueError("Missing value for required parameter 'run_id'"),
			request: &request.LogBatchRequest{},
		},
		{
			name:  "DuplicateKeyDifferentValueFails",
			error: api.NewInvalidParameterValueError("unable to insert params for run"),
			request: &request.LogBatchRequest{
				RunID: run.ID,
				Params: []request.ParamPartialRequest{
					{
						Key:   "key1",
						Value: "value1",
					},
					{
						Key:   "key1",
						Value: "value2",
					},
					{
						Key:   "key2",
						Value: "value2",
					},
				},
			},
		},
	}

	for _, tt := range testData {
		s.Run(tt.name, func() {
			resp := api.ErrorResponse{}
			s.Require().Nil(
				s.MlflowClient().WithMethod(
					http.MethodPost,
				).WithRequest(
					tt.request,
				).WithResponse(
					&resp,
				).DoRequest(
					"%s%s", mlflow.RunsRoutePrefix, mlflow.RunsLogBatchRoute,
				),
			)
			s.Equal(tt.error.ErrorCode, resp.ErrorCode)
			s.Contains(resp.Error(), tt.error.Message)

			// there should be no params inserted when error occurs.
			params, err := s.ParamFixtures.GetParamsByRunID(context.Background(), run.ID)
			s.Require().Nil(err)
			s.Empty(params)
		})
	}
}
