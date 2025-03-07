package experiment

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/G-Research/fasttrackml/pkg/api/aim/response"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
	"github.com/G-Research/fasttrackml/tests/integration/golang/helpers"
)

type DeleteExperimentTestSuite struct {
	helpers.BaseTestSuite
}

func TestDeleteExperimentTestSuite(t *testing.T) {
	suite.Run(t, &DeleteExperimentTestSuite{})
}

func (s *DeleteExperimentTestSuite) Test_Ok() {
	experiment, err := s.ExperimentFixtures.CreateExperiment(context.Background(), &models.Experiment{
		Name:           "Test Experiment",
		NamespaceID:    s.DefaultNamespace.ID,
		LifecycleStage: models.LifecycleStageActive,
	})
	s.Require().Nil(err)

	experiments, err := s.ExperimentFixtures.GetTestExperiments(context.Background())
	s.Require().Nil(err)
	length := len(experiments)

	var resp response.DeleteExperiment
	s.Require().Nil(
		s.AIMClient().WithMethod(
			http.MethodDelete,
		).WithResponse(
			&resp,
		).DoRequest(
			"/experiments/%d", *experiment.ID,
		),
	)

	remainingExperiments, err := s.ExperimentFixtures.GetTestExperiments(context.Background())
	s.Require().Nil(err)
	s.Equal(length-1, len(remainingExperiments))
}

func (s *DeleteExperimentTestSuite) Test_Error() {
	tests := []struct {
		name  string
		ID    string
		error string
	}{
		{
			ID:    "123",
			name:  "DeleteWithUnknownIDFails",
			error: "Not Found",
		},
		{
			ID:   "incorrect_experiment_id",
			name: "DeleteIncorrectExperimentID",
			error: `: unable to parse experiment id "incorrect_experiment_id": strconv.ParseInt:` +
				` parsing "incorrect_experiment_id": invalid syntax`,
		},
		{
			ID:    "0",
			name:  "DeleteDefaultExperiment",
			error: `unable to delete default experiment`,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			var resp api.ErrorResponse
			s.Require().Nil(
				s.AIMClient().WithMethod(
					http.MethodDelete,
				).WithResponse(
					&resp,
				).DoRequest(
					"/experiments/%s", tt.ID,
				),
			)
			s.Contains(resp.Error(), tt.error)
		})
	}
}
