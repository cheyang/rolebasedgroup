package discovery

import (
	"fmt"
	"sort"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/pkg/utils"
)

type ConfigBuilder struct {
	rbg  *workloadsv1alpha1.RoleBasedGroup
	role *workloadsv1alpha1.RoleSpec
}

const currentConfigVersion = "v1"

type ClusterConfig struct {
	Group         GroupInfo  `json:"group"`
	ConfigVersion string     `json:"configVersion,omitempty"`
	Roles         []RoleInfo `json:"roles"`
}

type GroupInfo struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	RoleNames []string `json:"roleNames"`
}

type RoleInfo struct {
	Name            string `json:"name"`                      // 1. name
	Type            string `json:"type"`                      // 2. type
	Service         string `json:"service,omitempty"`         // 3. service (for StatefulSet and LWS)
	ServiceTemplate string `json:"serviceTemplate,omitempty"` // 4. Service template name for LWS (if enabled)
	Replicas        *int32 `json:"replicas"`                  // 5. Service name for StatefulSet/LWS
	LwsWorkers      *int32 `json:"lwsWorkers,omitempty"`      // 6. Number of workers per leader in LWS
	StartIndex      *int32 `json:"startIndex"`                // 7. Starting index for instances
}

func (b *ConfigBuilder) ToClusterConfig() *ClusterConfig {
	namespace := b.rbg.Namespace
	if len(namespace) == 0 {
		namespace = corev1.NamespaceDefault
	}
	return &ClusterConfig{
		Group: GroupInfo{
			Name:      b.rbg.Name,
			Namespace: namespace,
			RoleNames: b.getRoleNames(),
		},
		ConfigVersion: currentConfigVersion,
		Roles:         b.buildRolesInfo(),
	}
}

func (b *ConfigBuilder) Build() ([]byte, error) {
	return yaml.Marshal(b.ToClusterConfig())
}

func (b *ConfigBuilder) getRoleNames() []string {
	names := make([]string, 0, len(b.rbg.Spec.Roles))
	for _, r := range b.rbg.Spec.Roles {
		names = append(names, r.Name)
	}
	return names
}

func (b *ConfigBuilder) buildRolesInfo() []RoleInfo {
	roles := make([]RoleInfo, 0, len(b.rbg.Spec.Roles))
	for _, role := range b.rbg.Spec.Roles {
		serviceName := b.rbg.GetWorkloadName(&role)
		kind := role.Workload.Kind
		if len(kind) == 0 {
			kind = "StatefulSet"
		}

		rg := RoleInfo{
			Name:       role.Name,
			Type:       kind,
			Replicas:   role.Replicas,
			StartIndex: ptr.To[int32](0),
		}

		if rg.Replicas == nil {
			rg.Replicas = ptr.To[int32](1)
		}

		if rg.Replicas == ptr.To[int32](0) {
			rg.StartIndex = ptr.To[int32](-1)
		}

		switch kind {
		case "StatefulSet":
			rg.Service = serviceName
		case "LeaderWorkerSet":
			// rg.ServiceTemplate = serviceName
			rg.Service = serviceName
			rg.LwsWorkers = role.LeaderWorkerSet.Size
		}

		roles = append(roles, rg)
	}
	sort.SliceStable(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	return roles
}

func semanticallyClusterConfig(old, new *ClusterConfig) (bool, string) {
	if old == nil && new == nil {
		return true, ""
	}
	if old == nil {
		return false, "old is nil"
	}
	if new == nil {
		return false, "new is nil"
	}

	opts := cmp.Options{
		cmpopts.EquateEmpty(),
	}

	diff := cmp.Diff(old, new, opts)
	return diff == "", diff
}

func semanticallyEqualConfigmap(old, new *corev1.ConfigMap) (bool, string) {
	if old == nil && new == nil {
		return true, ""
	}
	if old == nil || new == nil {
		return false, fmt.Sprintf("nil mismatch: old=%v, new=%v", old, new)
	}
	// Defensive copy to prevent side effects
	oldCopy := old.DeepCopy()
	newCopy := new.DeepCopy()

	oldCopy.Annotations = utils.FilterSystemAnnotations(oldCopy.Annotations)
	newCopy.Annotations = utils.FilterSystemAnnotations(newCopy.Annotations)

	objectMetaIgnoreOpts := cmpopts.IgnoreFields(
		metav1.ObjectMeta{},
		"ResourceVersion",
		"UID",
		"CreationTimestamp",
		"Generation",
		"ManagedFields",
		"SelfLink",
	)

	opts := cmp.Options{
		objectMetaIgnoreOpts,
		cmpopts.SortSlices(func(a, b metav1.OwnerReference) bool {
			return a.UID < b.UID // Make OwnerReferences comparison order-insensitive
		}),
		cmpopts.EquateEmpty(),
	}

	diff := cmp.Diff(oldCopy, newCopy, opts)
	return diff == "", diff
}
