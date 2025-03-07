package experiment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api/request"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api/response"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
	"github.com/G-Research/fasttrackml/tests/integration/golang/helpers"
)

type SearchExperimentsTestSuite struct {
	helpers.BaseTestSuite
}

func TestSearchExperimentsTestSuite(t *testing.T) {
	suite.Run(t, &SearchExperimentsTestSuite{
		helpers.BaseTestSuite{
			SkipCreateDefaultExperiment: true,
		},
	})
}

func (s *SearchExperimentsTestSuite) Test_Ok() {
	// 1. prepare database with test data.
	experiments := []models.Experiment{
		{
			Name:           "Test Experiment 1",
			LifecycleStage: models.LifecycleStageActive,
		},
		{
			Name:           "Test Experiment 2",
			LifecycleStage: models.LifecycleStageActive,
		},
		{
			Name:           "Test Experiment 3",
			LifecycleStage: models.LifecycleStageActive,
		},
		{
			Name:           "Test Experiment 4",
			LifecycleStage: models.LifecycleStageActive,
		},
		{
			Name:           "Test Experiment 5",
			LifecycleStage: models.LifecycleStageActive,
		},
		{
			Name:           "Test Experiment 6",
			LifecycleStage: models.LifecycleStageDeleted,
		},
	}
	for _, ex := range experiments {
		_, err := s.ExperimentFixtures.CreateExperiment(context.Background(), &models.Experiment{
			Name:           ex.Name,
			NamespaceID:    s.DefaultNamespace.ID,
			LifecycleStage: ex.LifecycleStage,
		})
		s.Require().Nil(err)
	}

	tests := []struct {
		name     string
		request  request.SearchExperimentsRequest
		expected []string
	}{
		{
			name: "TestFilter",
			request: request.SearchExperimentsRequest{
				Filter: "attribute.name != 'Test Experiment 5'",
			},
			expected: []string{
				"Test Experiment 1",
				"Test Experiment 2",
				"Test Experiment 3",
				"Test Experiment 4",
			},
		},
		{
			name: "TestViewType",
			request: request.SearchExperimentsRequest{
				ViewType: request.ViewTypeDeletedOnly,
			},
			expected: []string{"Test Experiment 6"},
		},
		{
			name: "TestOrderBy",
			request: request.SearchExperimentsRequest{
				OrderBy: []string{"name ASC"},
			},
			expected: []string{
				"Test Experiment 1",
				"Test Experiment 2",
				"Test Experiment 3",
				"Test Experiment 4",
				"Test Experiment 5",
			},
		},
		{
			name: "TestMaxResults",
			request: request.SearchExperimentsRequest{
				OrderBy:    []string{"name ASC"},
				MaxResults: 3,
			},
			expected: []string{"Test Experiment 1", "Test Experiment 2", "Test Experiment 3"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			resp := response.SearchExperimentsResponse{}
			s.Require().Nil(
				s.MlflowClient().WithQuery(
					tt.request,
				).WithResponse(
					&resp,
				).DoRequest(
					"%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsSearchRoute,
				),
			)

			names := make([]string, len(resp.Experiments))
			for i, exp := range resp.Experiments {
				names[i] = exp.Name
			}

			s.ElementsMatch(tt.expected, names)
		})
	}
}

func (s *SearchExperimentsTestSuite) Test_Error() {
	testData := []struct {
		name    string
		error   *api.ErrorResponse
		request request.SearchExperimentsRequest
	}{
		{
			name:  "InvalidViewType",
			error: api.NewInvalidParameterValueError("Invalid view_type 'invalid_ViewType'"),
			request: request.SearchExperimentsRequest{
				ViewType: "invalid_ViewType",
			},
		},
		{
			name:  "InvalidMaxResult",
			error: api.NewInvalidParameterValueError("Invalid value for parameter 'max_results' supplied."),
			request: request.SearchExperimentsRequest{
				MaxResults: 10000000,
			},
		},
		{
			name:  "InvalidFilterValue",
			error: api.NewInvalidParameterValueError("invalid numeric value 'cc'"),
			request: request.SearchExperimentsRequest{
				Filter: "attribute.creation_time > cc",
			},
		},
		{
			name:  "MalformedFilter",
			error: api.NewInvalidParameterValueError("malformed filter 'invalid_filter'"),
			request: request.SearchExperimentsRequest{
				Filter: "invalid_filter",
			},
		},
		{
			name:  "InvalidNumericValue",
			error: api.NewInvalidParameterValueError("invalid numeric value 'invalid_value'"),
			request: request.SearchExperimentsRequest{
				Filter: "creation_time > invalid_value",
			},
		},
		{
			name:  "InvalidStringOperator",
			error: api.NewInvalidParameterValueError("invalid string attribute comparison operator '<'"),
			request: request.SearchExperimentsRequest{
				Filter: "attribute.name < 'value'",
			},
		},
		{
			name:  "InvalidTagOperator",
			error: api.NewInvalidParameterValueError("invalid tag comparison operator '<'"),
			request: request.SearchExperimentsRequest{
				Filter: "tag.value < 'value'",
			},
		},
		{
			name: "InvalidEntity",
			error: api.NewInvalidParameterValueError(
				"invalid entity type 'invalid_entity'. Valid values are ['tag', 'attribute']",
			),
			request: request.SearchExperimentsRequest{
				Filter: "invalid_entity.name = value",
			},
		},
		{
			name: "InvalidOrderByAttribute",
			error: api.NewInvalidParameterValueError(
				`invalid attribute 'invalid_attribute'. ` +
					`Valid values are ['name', 'experiment_id', 'creation_time', 'last_update_time']`,
			),
			request: request.SearchExperimentsRequest{
				OrderBy: []string{"invalid_attribute"},
			},
		},
	}

	for _, tt := range testData {
		s.Run(tt.name, func() {
			resp := api.ErrorResponse{}
			s.Require().Nil(
				s.MlflowClient().WithQuery(
					tt.request,
				).WithResponse(
					&resp,
				).DoRequest(
					"%s%s", mlflow.ExperimentsRoutePrefix, mlflow.ExperimentsSearchRoute,
				),
			)
			s.Equal(tt.error.Error(), resp.Error())
		})
	}
}
