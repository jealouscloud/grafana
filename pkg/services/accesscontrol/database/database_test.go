package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/accesscontrol/resourcepermissions/types"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/services/user"
)

type getUserPermissionsTestCase struct {
	desc               string
	anonymousUser      bool
	orgID              int64
	role               string
	userPermissions    []string
	teamPermissions    []string
	builtinPermissions []string
	actions            []string
	expected           int
}

func TestAccessControlStore_GetUserPermissions(t *testing.T) {
	tests := []getUserPermissionsTestCase{
		{
			desc:               "should successfully get user, team and builtin permissions",
			orgID:              1,
			role:               "Admin",
			userPermissions:    []string{"1", "2", "10"},
			teamPermissions:    []string{"100", "2"},
			builtinPermissions: []string{"5", "6"},
			expected:           7,
		},
		{
			desc:               "Should not get admin roles",
			orgID:              1,
			role:               "Viewer",
			userPermissions:    []string{"1", "2", "10"},
			teamPermissions:    []string{"100", "2"},
			builtinPermissions: []string{"5", "6"},
			expected:           5,
		},
		{
			desc:               "Should work without org role",
			orgID:              1,
			role:               "",
			userPermissions:    []string{"1", "2", "10"},
			teamPermissions:    []string{"100", "2"},
			builtinPermissions: []string{"5", "6"},
			expected:           5,
		},
		{
			desc:               "Should filter on actions",
			orgID:              1,
			role:               "",
			userPermissions:    []string{"1", "2", "10"},
			teamPermissions:    []string{"100", "2"},
			builtinPermissions: []string{"5", "6"},
			expected:           3,
			actions:            []string{"dashboards:write"},
		},
		{
			desc:               "Should return no permission when passing empty slice of actions",
			orgID:              1,
			role:               "Viewer",
			userPermissions:    []string{"1", "2", "10"},
			teamPermissions:    []string{"100", "2"},
			builtinPermissions: []string{"5", "6"},
			expected:           0,
			actions:            []string{},
		},
		{
			desc:               "should only get br permissions for anonymous user",
			anonymousUser:      true,
			orgID:              1,
			role:               "Admin",
			userPermissions:    []string{"1", "2", "10"},
			teamPermissions:    []string{"100", "2"},
			builtinPermissions: []string{"5", "6"},
			expected:           2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			store, sql := setupTestEnv(t)

			user, team := createUserAndTeam(t, sql, tt.orgID)

			for _, id := range tt.userPermissions {
				_, err := store.SetUserResourcePermission(context.Background(), tt.orgID, accesscontrol.User{ID: user.ID}, types.SetResourcePermissionCommand{
					Actions:    []string{"dashboards:write"},
					Resource:   "dashboards",
					ResourceID: id,
				}, nil)
				require.NoError(t, err)
			}

			for _, id := range tt.teamPermissions {
				_, err := store.SetTeamResourcePermission(context.Background(), tt.orgID, team.Id, types.SetResourcePermissionCommand{
					Actions:    []string{"dashboards:read"},
					Resource:   "dashboards",
					ResourceID: id,
				}, nil)
				require.NoError(t, err)
			}

			for _, id := range tt.builtinPermissions {
				_, err := store.SetBuiltInResourcePermission(context.Background(), tt.orgID, "Admin", types.SetResourcePermissionCommand{
					Actions:    []string{"dashboards:read"},
					Resource:   "dashboards",
					ResourceID: id,
				}, nil)
				require.NoError(t, err)
			}

			var roles []string
			role := org.RoleType(tt.role)

			if role.IsValid() {
				roles = append(roles, string(role))
				for _, c := range role.Children() {
					roles = append(roles, string(c))
				}
			}

			userID := user.ID
			if tt.anonymousUser {
				userID = 0
			}
			permissions, err := store.GetUserPermissions(context.Background(), accesscontrol.GetUserPermissionsQuery{
				OrgID:   tt.orgID,
				UserID:  userID,
				Roles:   roles,
				Actions: tt.actions,
			})

			require.NoError(t, err)
			assert.Len(t, permissions, tt.expected)
		})
	}
}

func TestAccessControlStore_DeleteUserPermissions(t *testing.T) {
	store, sql := setupTestEnv(t)

	user, _ := createUserAndTeam(t, sql, 1)

	_, err := store.SetUserResourcePermission(context.Background(), 1, accesscontrol.User{ID: user.ID}, types.SetResourcePermissionCommand{
		Actions:    []string{"dashboards:write"},
		Resource:   "dashboards",
		ResourceID: "1",
	}, nil)
	require.NoError(t, err)

	err = store.DeleteUserPermissions(context.Background(), user.ID)
	require.NoError(t, err)

	permissions, err := store.GetUserPermissions(context.Background(), accesscontrol.GetUserPermissionsQuery{
		OrgID:   1,
		UserID:  user.ID,
		Roles:   []string{"Admin"},
		Actions: []string{"dashboards:write"},
	})
	require.NoError(t, err)
	assert.Len(t, permissions, 0)
}

func createUserAndTeam(t *testing.T, sql *sqlstore.SQLStore, orgID int64) (*user.User, models.Team) {
	t.Helper()

	user, err := sql.CreateUser(context.Background(), user.CreateUserCommand{
		Login: "user",
		OrgID: orgID,
	})
	require.NoError(t, err)

	team, err := sql.CreateTeam("team", "", orgID)
	require.NoError(t, err)

	err = sql.AddTeamMember(user.ID, orgID, team.Id, false, models.PERMISSION_VIEW)
	require.NoError(t, err)

	return user, team
}

func setupTestEnv(t testing.TB) (*AccessControlStore, *sqlstore.SQLStore) {
	store := sqlstore.InitTestDB(t)
	return ProvideService(store), store
}
