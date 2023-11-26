//go:build integration

package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/G-Research/fasttrackml/pkg/api/mlflow/api"
	"github.com/G-Research/fasttrackml/tests/integration/golang/helpers"
)

type NamespaceTestSuite struct {
	helpers.BaseTestSuite
}

func TestNamespaceTestSuite(t *testing.T) {
	suite.Run(t, new(NamespaceTestSuite))
}

func (s *NamespaceTestSuite) TearDownTest() {
	require.Nil(s.T(), s.NamespaceFixtures.UnloadFixtures())
}

func (s *NamespaceTestSuite) Test_Error() {
	tests := []struct {
		name      string
		error     *api.ErrorResponse
		namespace string
	}{
		{
			name:      "RequestNotExistingNamespace",
			error:     api.NewResourceDoesNotExistError("unable to find namespace with code: not-existing-namespace"),
			namespace: "not-existing-namespace",
		},
		{
			name:      "RequestNotExistingDefaultNamespaceExplicitly",
			error:     api.NewResourceDoesNotExistError("unable to find namespace with code: default"),
			namespace: "default",
		},
		{
			name:  "RequestNotExistingDefaultNamespaceImplicitly",
			error: api.NewResourceDoesNotExistError("unable to find namespace with code: default"),
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			resp := api.ErrorResponse{}
			require.Nil(
				s.T(),
				s.AIMClient().WithNamespace(
					tt.namespace,
				).WithResponse(
					&resp,
				).DoRequest(
					"/experiments",
				),
			)
			assert.Equal(s.T(), tt.error.Error(), resp.Error())
			assert.Equal(s.T(), api.ErrorCodeResourceDoesNotExist, string(resp.ErrorCode))
		})
	}
}