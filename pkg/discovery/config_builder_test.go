package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
)

func TestConfigBuilder_Build(t *testing.T) {
	tests := []struct {
		name     string
		rbg      *workloadsv1alpha1.RoleBasedGroup
		expected string
	}{
		{
			name: "basic statefulset",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-group",
					Namespace: "test-ns",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "web",
							Replicas: ptr.To[int32](3),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "StatefulSet",
							},
						},
					},
				},
			},
			expected: `group:
  name: test-group
  namespace: test-ns
  roleNames:
  - web
roles:
- name: web
  replicas: 3
  service: test-group-web
  startIndex: 0
  type: StatefulSet
`,
		},
		{
			name: "leader-worker-set",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lws-group",
					Namespace: "lws-ns",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "leader",
							Replicas: ptr.To[int32](2),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "LeaderWorkerSet",
							},
							LeaderWorkerSet: workloadsv1alpha1.LeaderWorkerTemplate{
								Size: ptr.To[int32](3),
							},
						},
					},
				},
			},
			expected: `group:
  name: lws-group
  namespace: lws-ns
  roleNames:
  - leader
roles:
- lwsWorkers: 3
  name: leader
  replicas: 2
  service: lws-group-leader
  startIndex: 0
  type: LeaderWorkerSet
`,
		},
		{
			name: "mixed roles",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-group",
					Namespace: "mixed-ns",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "leader",
							Replicas: ptr.To[int32](1),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "StatefulSet",
							},
						},
						{
							Name:     "worker",
							Replicas: ptr.To[int32](3),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "LeaderWorkerSet",
							},
							LeaderWorkerSet: workloadsv1alpha1.LeaderWorkerTemplate{
								Size: ptr.To[int32](2),
							},
						},
					},
				},
			},
			expected: `group:
  name: mixed-group
  namespace: mixed-ns
  roleNames:
  - leader
  - worker
roles:
- name: leader
  replicas: 1
  service: mixed-group-leader
  startIndex: 0
  type: StatefulSet
- lwsWorkers: 2
  name: worker
  replicas: 3
  service: mixed-group-worker
  startIndex: 0
  type: LeaderWorkerSet
`,
		},
		{
			name: "default namespace",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default-ns-group",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "default",
							Replicas: ptr.To[int32](1),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "StatefulSet",
							},
						},
					},
				},
			},
			expected: `group:
  name: default-ns-group
  namespace: default
  roleNames:
  - default
roles:
- name: default
  replicas: 1
  service: default-ns-group-default
  startIndex: 0
  type: StatefulSet
`,
		},
		{
			name: "zero replicas",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "zero-replicas",
					Namespace: "test-ns",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "inactive",
							Replicas: ptr.To[int32](0),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "StatefulSet",
							},
						},
					},
				},
			},
			expected: `group:
  name: zero-replicas
  namespace: test-ns
  roleNames:
  - inactive
roles:
- name: inactive
  replicas: 0
  service: zero-replicas-inactive
  startIndex: 0
  type: StatefulSet
`,
		},
		{
			name: "no leader worker set config",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-lws-config",
					Namespace: "test-ns",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "worker",
							Replicas: ptr.To[int32](2),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "LeaderWorkerSet",
							},
						},
					},
				},
			},
			expected: `group:
  name: no-lws-config
  namespace: test-ns
  roleNames:
  - worker
roles:
- name: worker
  replicas: 2
  service: no-lws-config-worker
  startIndex: 0
  type: LeaderWorkerSet
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &ConfigBuilder{
				rbg: tt.rbg,
			}

			config, err := builder.Build()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(config))
		})
	}
}

func TestConfigBuilder_BuildRolesInfo(t *testing.T) {
	tests := []struct {
		name     string
		rbg      *workloadsv1alpha1.RoleBasedGroup
		expected []RoleInfo
	}{
		{
			name: "multiple roles",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multi-role-group",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "api",
							Replicas: ptr.To[int32](3),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "StatefulSet",
							},
						},
						{
							Name:     "worker",
							Replicas: ptr.To[int32](5),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "LeaderWorkerSet",
							},
							LeaderWorkerSet: workloadsv1alpha1.LeaderWorkerTemplate{
								Size: ptr.To[int32](4),
							},
						},
					},
				},
			},
			expected: []RoleInfo{
				{
					Name:       "api",
					Type:       "StatefulSet",
					Service:    "multi-role-group-api",
					Replicas:   ptr.To[int32](3),
					StartIndex: ptr.To[int32](0),
				},
				{
					Name:       "worker",
					Type:       "LeaderWorkerSet",
					Service:    "multi-role-group-worker",
					Replicas:   ptr.To[int32](5),
					LwsWorkers: ptr.To[int32](4),
					StartIndex: ptr.To[int32](0),
				},
			},
		},
		{
			name: "empty kind defaults to StatefulSet",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-kind-group",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "default",
							Replicas: ptr.To[int32](1),
							Workload: workloadsv1alpha1.WorkloadSpec{
								Kind: "", // empty kind
							},
						},
					},
				},
			},
			expected: []RoleInfo{
				{
					Name:       "default",
					Type:       "StatefulSet", // should default to StatefulSet
					Service:    "empty-kind-group-default",
					Replicas:   ptr.To[int32](1),
					StartIndex: ptr.To[int32](0),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &ConfigBuilder{
				rbg: tt.rbg,
			}

			roles := builder.buildRolesInfo()
			assert.Equal(t, tt.expected, roles)
		})
	}
}

func TestConfigBuilder_GetRoleNames(t *testing.T) {
	tests := []struct {
		name     string
		rbg      *workloadsv1alpha1.RoleBasedGroup
		expected []string
	}{
		{
			name: "single role",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{Name: "web"},
					},
				},
			},
			expected: []string{"web"},
		},
		{
			name: "multiple roles",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{Name: "leader"},
						{Name: "worker"},
						{Name: "monitor"},
					},
				},
			},
			expected: []string{"leader", "worker", "monitor"},
		},
		{
			name: "no roles",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{},
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &ConfigBuilder{
				rbg: tt.rbg,
			}

			names := builder.getRoleNames()
			assert.Equal(t, tt.expected, names)
		})
	}
}

func Test_clusterConfigSemanticallyEqual(t *testing.T) {
	defaultReplicas := int32(3)
	defaultStartIndex := int32(0)
	defaultLwsWorkers := int32(2)

	tests := []struct {
		name      string
		old       *ClusterConfig
		new       *ClusterConfig
		wantEqual bool
		// wantDiff is checked only if wantEqual is false
		wantDiff string
	}{
		{
			name:      "both nil",
			old:       nil,
			new:       nil,
			wantEqual: true,
			wantDiff:  "",
		},
		{
			name:      "old nil",
			old:       nil,
			new:       &ClusterConfig{},
			wantEqual: false,
			wantDiff:  "old is nil",
		},
		{
			name:      "new nil",
			old:       &ClusterConfig{},
			new:       nil,
			wantEqual: false,
			wantDiff:  "new is nil",
		},
		{
			name: "identical configs",
			old: &ClusterConfig{
				Group: GroupInfo{
					Name:      "test-group",
					Namespace: "test-ns",
					RoleNames: []string{"role1", "role2"},
				},
				Roles: []RoleInfo{
					{
						Name:       "role1",
						Type:       "StatefulSet",
						Service:    "svc1",
						Replicas:   &defaultReplicas,
						StartIndex: &defaultStartIndex,
					},
					{
						Name:       "role2",
						Type:       "LeaderWorkerSet",
						Service:    "svc2",
						Replicas:   &defaultReplicas,
						LwsWorkers: &defaultLwsWorkers,
						StartIndex: &defaultStartIndex,
					},
				},
			},
			new: &ClusterConfig{
				Group: GroupInfo{
					Name:      "test-group",
					Namespace: "test-ns",
					RoleNames: []string{"role1", "role2"},
				},
				Roles: []RoleInfo{
					{
						Name:       "role1",
						Type:       "StatefulSet",
						Service:    "svc1",
						Replicas:   &defaultReplicas,
						StartIndex: &defaultStartIndex,
					},
					{
						Name:       "role2",
						Type:       "LeaderWorkerSet",
						Service:    "svc2",
						Replicas:   &defaultReplicas,
						LwsWorkers: &defaultLwsWorkers,
						StartIndex: &defaultStartIndex,
					},
				},
			},
			wantEqual: true,
			wantDiff:  "",
		},
		{
			name: "different group name",
			old: &ClusterConfig{
				Group: GroupInfo{Name: "group-a", Namespace: "ns", RoleNames: []string{}},
				Roles: []RoleInfo{},
			},
			new: &ClusterConfig{
				Group: GroupInfo{Name: "group-b", Namespace: "ns", RoleNames: []string{}},
				Roles: []RoleInfo{},
			},
			wantEqual: false,
			// wantDiff content depends on cmp output, test checks it's non-empty
		},
		{
			name: "different role count",
			old: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{"role1"}},
				Roles: []RoleInfo{{Name: "role1", Type: "StatefulSet", Replicas: &defaultReplicas, StartIndex: &defaultStartIndex}},
			},
			new: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{"role1"}}, // RoleNames might still be the same
				Roles: []RoleInfo{},                                                            // But actual Roles list is different
			},
			wantEqual: false,
			// wantDiff content depends on cmp output, test checks it's non-empty
		},
		{
			name: "different role content",
			old: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{"role1"}},
				Roles: []RoleInfo{{Name: "role1", Type: "StatefulSet", Service: "svc1", Replicas: &defaultReplicas, StartIndex: &defaultStartIndex}},
			},
			new: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{"role1"}},
				Roles: []RoleInfo{{Name: "role1", Type: "StatefulSet", Service: "svc2", Replicas: &defaultReplicas, StartIndex: &defaultStartIndex}}, // Service changed
			},
			wantEqual: false,
			// wantDiff content depends on cmp output, test checks it's non-empty
		},
		{
			name: "different role order (should matter now that it's a slice)",
			old: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{"role1", "role2"}},
				Roles: []RoleInfo{
					{Name: "role1", Type: "StatefulSet", Replicas: &defaultReplicas, StartIndex: &defaultStartIndex},
					{Name: "role2", Type: "Deployment", Replicas: &defaultReplicas, StartIndex: &defaultStartIndex},
				},
			},
			new: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{"role1", "role2"}}, // RoleNames order might be independent
				Roles: []RoleInfo{
					{Name: "role2", Type: "Deployment", Replicas: &defaultReplicas, StartIndex: &defaultStartIndex}, // Order swapped
					{Name: "role1", Type: "StatefulSet", Replicas: &defaultReplicas, StartIndex: &defaultStartIndex},
				},
			},
			wantEqual: false, // Order in slice matters
			// wantDiff content depends on cmp output, test checks it's non-empty
		},
		{
			name: "nil pointer vs zero value replica",
			old: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{"role1"}},
				Roles: []RoleInfo{{Name: "role1", Type: "StatefulSet", Replicas: nil, StartIndex: &defaultStartIndex}}, // Replicas is nil
			},
			new: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{"role1"}},
				Roles: []RoleInfo{{Name: "role1", Type: "StatefulSet", Replicas: &defaultReplicas, StartIndex: &defaultStartIndex}}, // Replicas is pointer to 3
			},
			wantEqual: false,
			// wantDiff content depends on cmp output, test checks it's non-empty
		},
		{
			name: "empty slice vs nil slice",
			old: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: []string{}}, // Empty slice
				Roles: []RoleInfo{},                                                     // Empty slice
			},
			new: &ClusterConfig{
				Group: GroupInfo{Name: "group", Namespace: "ns", RoleNames: nil}, // Nil slice
				Roles: nil,                                                       // Nil slice
			},
			// Note: cmpopts.EquateEmpty() should make empty and nil slices equal
			wantEqual: true,
			wantDiff:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEqual, gotDiff := clusterConfigSemanticallyEqual(tt.old, tt.new)

			if gotEqual != tt.wantEqual {
				t.Errorf("clusterConfigSemanticallyEqual() equal = %v, want %v", gotEqual, tt.wantEqual)
			}

			if !tt.wantEqual {
				// If we expect them to be different, ensure a diff message was provided
				if gotDiff == "" {
					t.Errorf("clusterConfigSemanticallyEqual() diff = %q, want non-empty diff message", gotDiff)
				}
				// If a specific diff message was expected, check it (though cmp diff strings can be fragile)
				// For tests where diff content varies, just checking non-empty is often sufficient
				// If you add specific wantDiff strings for stable cases, uncomment below:
				// if tt.wantDiff != "" && gotDiff != tt.wantDiff {
				//     t.Errorf("clusterConfigSemanticallyEqual() diff = %q, want %q", gotDiff, tt.wantDiff)
				// }
			} else {
				// If we expect them to be equal, diff should be empty
				if gotDiff != "" {
					t.Errorf("clusterConfigSemanticallyEqual() diff = %q, want empty diff message", gotDiff)
				}
			}
		})
	}
}

func TestConfigBuilder_ToClusterConfig(t *testing.T) {
	defaultReplicas := int32(2)
	defaultStartIndex := int32(0)
	defaultLwsWorkers := int32(3)

	tests := []struct {
		name       string
		rbg        *workloadsv1alpha1.RoleBasedGroup
		wantConfig *ClusterConfig // Expected *ClusterConfig
	}{
		{
			name: "basic conversion with StatefulSet and LeaderWorkerSet",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-group",
					Namespace: "test-namespace",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "web",
							Replicas: &defaultReplicas,
							Workload: workloadsv1alpha1.WorkloadSpec{Kind: "StatefulSet"},
						},
						{
							Name:            "worker",
							Replicas:        &defaultReplicas,
							Workload:        workloadsv1alpha1.WorkloadSpec{Kind: "LeaderWorkerSet"},
							LeaderWorkerSet: workloadsv1alpha1.LeaderWorkerTemplate{Size: &defaultLwsWorkers},
						},
					},
				},
			},
			wantConfig: &ClusterConfig{
				Group: GroupInfo{
					Name:      "test-group",
					Namespace: "test-namespace",
					RoleNames: []string{"web", "worker"},
				},
				Roles: []RoleInfo{
					{
						Name:       "web",
						Type:       "StatefulSet",
						Service:    "test-group-web", // Assuming GetWorkloadName generates this
						Replicas:   &defaultReplicas,
						StartIndex: &defaultStartIndex,
					},
					{
						Name:       "worker",
						Type:       "LeaderWorkerSet",
						Service:    "test-group-worker", // Assuming GetWorkloadName generates this
						Replicas:   &defaultReplicas,
						LwsWorkers: &defaultLwsWorkers,
						StartIndex: &defaultStartIndex,
					},
				},
			},
		},
		{
			name: "default namespace",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "group-default-ns", // No Namespace set
					// Namespace: "", // Implicitly empty
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "default-role",
							Replicas: &defaultReplicas,
							Workload: workloadsv1alpha1.WorkloadSpec{Kind: "StatefulSet"},
						},
					},
				},
			},
			wantConfig: &ClusterConfig{
				Group: GroupInfo{
					Name:      "group-default-ns",
					Namespace: "default", // Should default to "default"
					RoleNames: []string{"default-role"},
				},
				Roles: []RoleInfo{
					{
						Name:       "default-role",
						Type:       "StatefulSet",
						Service:    "group-default-ns-default-role",
						Replicas:   &defaultReplicas,
						StartIndex: &defaultStartIndex,
					},
				},
			},
		},
		{
			name: "empty kind defaults to StatefulSet",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-kind-group",
					Namespace: "test-ns",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{
						{
							Name:     "defaulted-role",
							Replicas: &defaultReplicas,
							Workload: workloadsv1alpha1.WorkloadSpec{Kind: ""}, // Empty Kind
						},
					},
				},
			},
			wantConfig: &ClusterConfig{
				Group: GroupInfo{
					Name:      "default-kind-group",
					Namespace: "test-ns",
					RoleNames: []string{"defaulted-role"},
				},
				Roles: []RoleInfo{
					{
						Name:       "defaulted-role",
						Type:       "StatefulSet", // Should default to StatefulSet
						Service:    "default-kind-group-defaulted-role",
						Replicas:   &defaultReplicas,
						StartIndex: &defaultStartIndex,
					},
				},
			},
		},
		{
			name: "no roles",
			rbg: &workloadsv1alpha1.RoleBasedGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-group",
					Namespace: "test-ns",
				},
				Spec: workloadsv1alpha1.RoleBasedGroupSpec{
					Roles: []workloadsv1alpha1.RoleSpec{}, // Empty Roles
				},
			},
			wantConfig: &ClusterConfig{
				Group: GroupInfo{
					Name:      "empty-group",
					Namespace: "test-ns",
					RoleNames: []string{}, // Should be empty
				},
				Roles: []RoleInfo{}, // Should be empty slice
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := &ConfigBuilder{
				rbg: tt.rbg,
			}

			gotConfig := builder.ToClusterConfig()

			// Use the new clusterConfigSemanticallyEqual for comparison
			equal, diff := clusterConfigSemanticallyEqual(tt.wantConfig, gotConfig)
			if !equal {
				// Provide a detailed diff using go-cmp if available, or just the message from clusterConfigSemanticallyEqual
				t.Errorf("ConfigBuilder.ToClusterConfig() = mismatch (-want +got):\n%s", diff)
				// Alternative using cmp.Diff directly for potentially more detail (uncomment if needed):
				// detailedDiff := cmp.Diff(tt.wantConfig, gotConfig, cmpopts.EquateEmpty())
				// t.Errorf("ConfigBuilder.ToClusterConfig() = mismatch (-want +got):\n%s\nOr semantically: %s", detailedDiff, diff)
			}
		})
	}
}
