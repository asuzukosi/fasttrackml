package run

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api/request"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
	"github.com/G-Research/fasttrackml/tests/integration/golang/helpers"
)

type DeleteRunTagTestSuite struct {
	helpers.BaseTestSuite
}

func TestDeleteRunTagTestSuite(t *testing.T) {
	suite.Run(t, new(DeleteRunTagTestSuite))
}

func (s *DeleteRunTagTestSuite) Test_Ok() {
	// create test run.
	run, err := s.RunFixtures.CreateRun(context.Background(), &models.Run{
		ID:     strings.ReplaceAll(uuid.New().String(), "-", ""),
		Name:   "TestRun",
		Status: models.StatusRunning,
		StartTime: sql.NullInt64{
			Int64: 1234567890,
			Valid: true,
		},
		EndTime: sql.NullInt64{
			Int64: 1234567899,
			Valid: true,
		},
		SourceType:     "JOB",
		ArtifactURI:    "artifact_uri",
		ExperimentID:   *s.DefaultExperiment.ID,
		LifecycleStage: models.LifecycleStageActive,
	})
	s.Require().Nil(err)

	// create few tags.
	_, err = s.TagFixtures.CreateTag(context.Background(), &models.Tag{
		Key:   "tag1",
		Value: "value1",
		RunID: run.ID,
	})
	s.Require().Nil(err)
	_, err = s.TagFixtures.CreateTag(context.Background(), &models.Tag{
		Key:   "tag2",
		Value: "value2",
		RunID: run.ID,
	})
	s.Require().Nil(err)

	// make actual call to API.
	query := request.GetRunRequest{
		RunID: run.ID,
	}
	req := request.DeleteRunTagRequest{
		RunID: run.ID,
		Key:   "tag1",
	}
	resp := fiber.Map{}
	s.Require().Nil(
		s.MlflowClient().WithMethod(
			http.MethodPost,
		).WithQuery(
			query,
		).WithRequest(
			req,
		).WithResponse(
			&resp,
		).DoRequest(
			"%s%s", mlflow.RunsRoutePrefix, mlflow.RunsDeleteTagRoute,
		),
	)
	s.Equal(fiber.Map{}, resp)

	// make sure that we still have one tag connected to Run.
	tags, err := s.TagFixtures.GetByRunID(context.Background(), run.ID)
	s.Require().Nil(err)
	s.Equal(1, len(tags))
	s.Equal([]models.Tag{
		{
			Key:   "tag2",
			RunID: run.ID,
			Value: "value2",
		},
	}, tags)
}

func (s *DeleteRunTagTestSuite) Test_Error() {
	// create test run.
	run, err := s.RunFixtures.CreateRun(context.Background(), &models.Run{
		ID:     strings.ReplaceAll(uuid.New().String(), "-", ""),
		Name:   "TestRun",
		Status: models.StatusRunning,
		StartTime: sql.NullInt64{
			Int64: 1234567890,
			Valid: true,
		},
		EndTime: sql.NullInt64{
			Int64: 1234567899,
			Valid: true,
		},
		SourceType:     "JOB",
		ArtifactURI:    "artifact_uri",
		ExperimentID:   *s.DefaultExperiment.ID,
		LifecycleStage: models.LifecycleStageActive,
	})
	s.Require().Nil(err)

	tests := []struct {
		name    string
		error   *api.ErrorResponse
		request request.DeleteRunTagRequest
	}{
		{
			name:    "EmptyOrIncorrectRunID",
			request: request.DeleteRunTagRequest{},
			error:   api.NewInvalidParameterValueError("Missing value for required parameter 'run_id'"),
		},
		{
			name: "NotFoundRun",
			request: request.DeleteRunTagRequest{
				RunID: "id",
			},
			error: api.NewResourceDoesNotExistError("Run 'id' not found"),
		},
		{
			name: "NotFoundTag",
			request: request.DeleteRunTagRequest{
				Key:   "not_found_tag",
				RunID: run.ID,
			},
			error: api.NewResourceDoesNotExistError("No tag with name: not_found_tag"),
		},
	}
	for _, tt := range tests {
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
					"%s%s", mlflow.RunsRoutePrefix, mlflow.RunsDeleteTagRoute,
				),
			)
			s.Equal(tt.error.Error(), resp.Error())
		})
	}
}
