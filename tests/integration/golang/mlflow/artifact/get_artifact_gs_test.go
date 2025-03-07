package artifact

import (
	"bytes"
	"context"
	"fmt"
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

type GetArtifactGSTestSuite struct {
	helpers.GSTestSuite
}

func TestGetArtifactGSTestSuite(t *testing.T) {
	suite.Run(t, &GetArtifactGSTestSuite{
		helpers.NewGSTestSuite("bucket1", "bucket2"),
	})
}

func (s *GetArtifactGSTestSuite) Test_Ok() {
	tests := []struct {
		name   string
		bucket string
	}{
		{
			name:   "TestWithBucket1",
			bucket: "bucket1",
		},
		{
			name:   "TestWithBucket2",
			bucket: "bucket2",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// create test experiment
			experiment, err := s.ExperimentFixtures.CreateExperiment(context.Background(), &models.Experiment{
				Name:             fmt.Sprintf("Test Experiment In Bucket %s", tt.bucket),
				NamespaceID:      s.DefaultNamespace.ID,
				LifecycleStage:   models.LifecycleStageActive,
				ArtifactLocation: fmt.Sprintf("gs://%s/1", tt.bucket),
			})
			s.Require().Nil(err)

			// create test run
			runID := strings.ReplaceAll(uuid.New().String(), "-", "")
			run, err := s.RunFixtures.CreateRun(context.Background(), &models.Run{
				ID:             runID,
				Status:         models.StatusRunning,
				SourceType:     "JOB",
				ExperimentID:   *experiment.ID,
				ArtifactURI:    fmt.Sprintf("%s/%s/artifacts", experiment.ArtifactLocation, runID),
				LifecycleStage: models.LifecycleStageActive,
			})
			s.Require().Nil(err)

			// upload artifact root object to GS
			writer := s.Client.Bucket(tt.bucket).Object(
				fmt.Sprintf("/1/%s/artifacts/artifact.txt", runID),
			).NewWriter(context.Background())
			_, err = writer.Write([]byte("content"))
			s.Require().Nil(err)
			s.Require().Nil(writer.Close())

			// upload artifact subdir object to GS
			writer = s.Client.Bucket(tt.bucket).Object(
				fmt.Sprintf("/1/%s/artifacts/artifact/artifact.txt", runID),
			).NewWriter(context.Background())
			_, err = writer.Write([]byte("subdir-object-content"))
			s.Require().Nil(err)
			s.Require().Nil(writer.Close())

			// make API call for root object
			query := request.GetArtifactRequest{
				RunID: run.ID,
				Path:  "artifact.txt",
			}

			resp := new(bytes.Buffer)
			s.Require().Nil(s.MlflowClient().WithQuery(
				query,
			).WithResponseType(
				helpers.ResponseTypeBuffer,
			).WithResponse(
				resp,
			).DoRequest(
				fmt.Sprintf("%s%s", mlflow.ArtifactsRoutePrefix, mlflow.ArtifactsGetRoute),
			))
			s.Equal("content", resp.String())

			// make API call for subdir object
			query = request.GetArtifactRequest{
				RunID: run.ID,
				Path:  "artifact/artifact.txt",
			}

			resp = new(bytes.Buffer)
			s.Require().Nil(s.MlflowClient().WithQuery(
				query,
			).WithResponseType(
				helpers.ResponseTypeBuffer,
			).WithResponse(
				resp,
			).DoRequest(
				"%s%s", mlflow.ArtifactsRoutePrefix, mlflow.ArtifactsGetRoute,
			))
			s.Equal("subdir-object-content", resp.String())
		})
	}
}

func (s *GetArtifactGSTestSuite) Test_Error() {
	// create test experiment
	experiment, err := s.ExperimentFixtures.CreateExperiment(context.Background(), &models.Experiment{
		Name:             "Test Experiment In Bucket bucket1",
		NamespaceID:      s.DefaultNamespace.ID,
		LifecycleStage:   models.LifecycleStageActive,
		ArtifactLocation: "gs://bucket1/1",
	})
	s.Require().Nil(err)

	// create test run
	runID := strings.ReplaceAll(uuid.New().String(), "-", "")
	_, err = s.RunFixtures.CreateRun(context.Background(), &models.Run{
		ID:             runID,
		Status:         models.StatusRunning,
		SourceType:     "JOB",
		ExperimentID:   *experiment.ID,
		ArtifactURI:    fmt.Sprintf("%s/%s/artifacts", experiment.ArtifactLocation, runID),
		LifecycleStage: models.LifecycleStageActive,
	})
	s.Require().Nil(err)

	// upload artifact subdir object to GS
	s.Require().Nil(err)
	writer := s.Client.Bucket("bucket1").Object(
		fmt.Sprintf("1/%s/artifacts/artifact/artifact.file", runID),
	).NewWriter(context.Background())
	_, err = writer.Write([]byte("content"))
	s.Require().Nil(err)
	s.Require().Nil(writer.Close())

	tests := []struct {
		name    string
		error   *api.ErrorResponse
		request request.GetArtifactRequest
	}{
		{
			name:    "EmptyOrIncorrectRunIDOrRunUUID",
			error:   api.NewInvalidParameterValueError("Missing value for required parameter 'run_id'"),
			request: request.GetArtifactRequest{},
		},
		{
			name:  "IncorrectPathProvidedCase1",
			error: api.NewInvalidParameterValueError("Invalid path"),
			request: request.GetArtifactRequest{
				RunID: "run_id",
				Path:  "..",
			},
		},
		{
			name:  "IncorrectPathProvidedCase2",
			error: api.NewInvalidParameterValueError("Invalid path"),
			request: request.GetArtifactRequest{
				RunID: "run_id",
				Path:  "./..",
			},
		},
		{
			name:  "IncorrectPathProvidedCase3",
			error: api.NewInvalidParameterValueError("Invalid path"),
			request: request.GetArtifactRequest{
				RunID: "run_id",
				Path:  "./../",
			},
		},
		{
			name:  "IncorrectPathProvidedCase4",
			error: api.NewInvalidParameterValueError("Invalid path"),
			request: request.GetArtifactRequest{
				RunID: "run_id",
				Path:  "foo/../bar",
			},
		},
		{
			name:  "IncorrectPathProvidedCase5",
			error: api.NewInvalidParameterValueError("Invalid path"),
			request: request.GetArtifactRequest{
				RunID: "run_id",
				Path:  "/foo/../bar",
			},
		},
		{
			name: "GSIncompletePath",
			error: api.NewResourceDoesNotExistError(
				fmt.Sprintf("error getting artifact object for URI: gs:/bucket1/1/%s/artifacts/artifact", runID),
			),
			request: request.GetArtifactRequest{
				RunID: runID,
				Path:  "artifact",
			},
		},
		{
			name: "NonExistentFile",
			error: api.NewResourceDoesNotExistError(
				fmt.Sprintf("error getting artifact object for URI: gs:/bucket1/1/%s/artifacts/non-existent-file", runID),
			),
			request: request.GetArtifactRequest{
				RunID: runID,
				Path:  "non-existent-file",
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			resp := api.ErrorResponse{}
			s.Require().Nil(s.MlflowClient().WithQuery(
				tt.request,
			).WithResponse(
				&resp,
			).DoRequest(
				"%s%s", mlflow.ArtifactsRoutePrefix, mlflow.ArtifactsGetRoute,
			))
			s.Equal(tt.error.Error(), resp.Error())
		})
	}
}
