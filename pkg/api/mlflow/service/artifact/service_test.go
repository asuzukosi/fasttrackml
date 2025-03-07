package artifact

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api/request"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/models"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/dao/repositories"
	"github.com/G-Research/fasttrackml/pkg/api/mlflow/service/artifact/storage"
)

func TestService_ListArtifacts_Ok(t *testing.T) {
	artifactStorage := storage.MockArtifactStorageProvider{}
	artifactStorage.On(
		"List", context.TODO(), "/artifact/uri", "",
	).Return(
		[]storage.ArtifactObject{
			{
				Path:  "path1",
				Size:  1234567890,
				IsDir: false,
			},
			{
				Path:  "path2",
				Size:  123456788,
				IsDir: true,
			},
		}, nil,
	)

	artifactStorageFactory := storage.MockArtifactStorageFactoryProvider{}
	artifactStorageFactory.On(
		"GetStorage", context.TODO(), "/artifact/uri",
	).Return(&artifactStorage, nil)

	// init repository mocks.
	runRepository := repositories.MockRunRepositoryProvider{}
	runRepository.On(
		"GetByNamespaceIDAndRunID",
		context.TODO(),
		uint(1),
		"id",
	).Return(&models.Run{
		ID:          "id",
		ArtifactURI: "/artifact/uri",
	}, nil)

	// call service under testing.
	service := NewService(&runRepository, &artifactStorageFactory)
	rootURI, artifacts, err := service.ListArtifacts(
		context.TODO(),
		&models.Namespace{
			ID: 1,
		},
		&request.ListArtifactsRequest{
			RunID: "id",
		},
	)

	require.Nil(t, err)
	assert.Equal(t, "/artifact/uri", rootURI)
	assert.Equal(t, []storage.ArtifactObject{
		{
			Path:  "path1",
			Size:  1234567890,
			IsDir: false,
		},
		{
			Path:  "path2",
			Size:  123456788,
			IsDir: true,
		},
	}, artifacts)
}

func TestService_ListArtifacts_Error(t *testing.T) {
	testData := []struct {
		name    string
		error   *api.ErrorResponse
		request *request.ListArtifactsRequest
		service func() *Service
	}{
		{
			name:    "EmptyOrIncorrectRunID",
			error:   api.NewInvalidParameterValueError("Missing value for required parameter 'run_id'"),
			request: &request.ListArtifactsRequest{},
			service: func() *Service {
				return NewService(
					&repositories.MockRunRepositoryProvider{},
					&storage.MockArtifactStorageFactoryProvider{},
				)
			},
		},
		{
			name:  "PathIsRelativeAndContains2Dots",
			error: api.NewInvalidParameterValueError("Invalid path"),
			request: &request.ListArtifactsRequest{
				RunID: "id",
				Path:  "../",
			},
			service: func() *Service {
				return NewService(
					&repositories.MockRunRepositoryProvider{},
					&storage.MockArtifactStorageFactoryProvider{},
				)
			},
		},
		{
			name:  "RunNotFoundDatabaseError",
			error: api.NewInternalError("unable to find run 'id': database error"),
			request: &request.ListArtifactsRequest{
				RunID: "id",
			},
			service: func() *Service {
				runRepository := repositories.MockRunRepositoryProvider{}
				runRepository.On(
					"GetByNamespaceIDAndRunID",
					context.TODO(),
					uint(1),
					"id",
				).Return(nil, errors.New("database error"))
				return NewService(
					&runRepository,
					&storage.MockArtifactStorageFactoryProvider{},
				)
			},
		},
		{
			name:  "StorageError",
			error: api.NewInternalError("error getting artifact list from storage"),
			request: &request.ListArtifactsRequest{
				RunID: "id",
			},
			service: func() *Service {
				artifactStorage := storage.MockArtifactStorageProvider{}
				artifactStorage.On(
					"List", context.TODO(), "/artifact/uri", "",
				).Return(
					nil, errors.New("storage error"),
				)

				artifactStorageFactory := storage.MockArtifactStorageFactoryProvider{}
				artifactStorageFactory.On(
					"GetStorage", context.TODO(), "/artifact/uri",
				).Return(&artifactStorage, nil)

				runRepository := repositories.MockRunRepositoryProvider{}
				runRepository.On(
					"GetByNamespaceIDAndRunID",
					context.TODO(),
					uint(1),
					"id",
				).Return(&models.Run{
					ID:          "id",
					ArtifactURI: "/artifact/uri",
				}, nil)
				return NewService(
					&runRepository,
					&artifactStorageFactory,
				)
			},
		},
	}

	for _, tt := range testData {
		t.Run(tt.name, func(t *testing.T) {
			// call service under testing.
			_, _, err := tt.service().ListArtifacts(context.TODO(), &models.Namespace{
				ID: 1,
			}, tt.request)
			assert.Equal(t, tt.error, err)
		})
	}
}

func TestService_GetArtifact_Ok(t *testing.T) {
	artifactStorage := storage.MockArtifactStorageProvider{}
	artifactStorage.On(
		"Get", context.TODO(), "/artifact/uri", "",
	).Return(
		io.NopCloser(strings.NewReader("content")), nil,
	)

	artifactStorageFactory := storage.MockArtifactStorageFactoryProvider{}
	artifactStorageFactory.On(
		"GetStorage", context.TODO(), "/artifact/uri",
	).Return(&artifactStorage, nil)

	// init repository mocks.
	runRepository := repositories.MockRunRepositoryProvider{}
	runRepository.On(
		"GetByNamespaceIDAndRunID",
		context.TODO(),
		uint(1),
		"id",
	).Return(&models.Run{
		ID:          "id",
		ArtifactURI: "/artifact/uri",
	}, nil)

	// call service under testing.
	service := NewService(&runRepository, &artifactStorageFactory)
	data, err := service.GetArtifact(
		context.TODO(),
		&models.Namespace{
			ID: 1,
		},
		&request.GetArtifactRequest{
			RunID: "id",
		},
	)

	require.Nil(t, err)
	result := new(bytes.Buffer)
	_, err = result.ReadFrom(data)
	require.Nil(t, err)
	assert.Equal(t, "content", result.String())
}

func TestService_GetArtifact_Error(t *testing.T) {
	testData := []struct {
		name    string
		error   *api.ErrorResponse
		request *request.GetArtifactRequest
		service func() *Service
	}{
		{
			name:    "EmptyOrIncorrectRunID",
			error:   api.NewInvalidParameterValueError("Missing value for required parameter 'run_id'"),
			request: &request.GetArtifactRequest{},
			service: func() *Service {
				return NewService(
					&repositories.MockRunRepositoryProvider{},
					&storage.MockArtifactStorageFactoryProvider{},
				)
			},
		},
		{
			name:  "PathIsRelativeAndContains2Dots",
			error: api.NewInvalidParameterValueError("Invalid path"),
			request: &request.GetArtifactRequest{
				RunID: "id",
				Path:  "../",
			},
			service: func() *Service {
				return NewService(
					&repositories.MockRunRepositoryProvider{},
					&storage.MockArtifactStorageFactoryProvider{},
				)
			},
		},
		{
			name:  "RunNotFoundDatabaseError",
			error: api.NewInternalError("unable to find run 'id': database error"),
			request: &request.GetArtifactRequest{
				RunID: "id",
			},
			service: func() *Service {
				runRepository := repositories.MockRunRepositoryProvider{}
				runRepository.On(
					"GetByNamespaceIDAndRunID",
					context.TODO(),
					uint(1),
					"id",
				).Return(nil, errors.New("database error"))
				return NewService(
					&runRepository,
					&storage.MockArtifactStorageFactoryProvider{},
				)
			},
		},
		{
			name:  "StorageError",
			error: api.NewInternalError("error getting artifact object for URI: /artifact/uri"),
			request: &request.GetArtifactRequest{
				RunID: "id",
			},
			service: func() *Service {
				artifactStorage := storage.MockArtifactStorageProvider{}
				artifactStorage.On(
					"Get", context.TODO(), "/artifact/uri", "",
				).Return(
					nil, errors.New("storage error"),
				)

				artifactStorageFactory := storage.MockArtifactStorageFactoryProvider{}
				artifactStorageFactory.On(
					"GetStorage", context.TODO(), "/artifact/uri",
				).Return(&artifactStorage, nil)

				runRepository := repositories.MockRunRepositoryProvider{}
				runRepository.On(
					"GetByNamespaceIDAndRunID",
					context.TODO(),
					uint(1),
					"id",
				).Return(&models.Run{
					ID:          "id",
					ArtifactURI: "/artifact/uri",
				}, nil)
				return NewService(
					&runRepository,
					&artifactStorageFactory,
				)
			},
		},
		{
			name:  "UnsupportedStorage",
			error: api.NewInternalError("run with id 'id' has unsupported artifact storage"),
			request: &request.GetArtifactRequest{
				RunID: "id",
			},
			service: func() *Service {
				artifactStorage := storage.MockArtifactStorageProvider{}
				artifactStorage.On(
					"Get", context.TODO(), "/artifact/uri", "",
				).Return(
					nil, errors.New("storage error"),
				)

				artifactStorageFactory := storage.MockArtifactStorageFactoryProvider{}
				artifactStorageFactory.On(
					"GetStorage", context.TODO(), "/artifact/uri",
				).Return(nil, errors.New("unsupported error"))

				runRepository := repositories.MockRunRepositoryProvider{}
				runRepository.On(
					"GetByNamespaceIDAndRunID",
					context.TODO(),
					uint(1),
					"id",
				).Return(&models.Run{
					ID:          "id",
					ArtifactURI: "/artifact/uri",
				}, nil)
				return NewService(
					&runRepository,
					&artifactStorageFactory,
				)
			},
		},
	}

	for _, tt := range testData {
		t.Run(tt.name, func(t *testing.T) {
			// call service under testing.
			_, err := tt.service().GetArtifact(context.TODO(), &models.Namespace{
				ID: 1,
			}, tt.request)
			assert.Equal(t, tt.error, err)
		})
	}
}
