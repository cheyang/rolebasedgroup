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
