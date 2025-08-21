package discovery

import (
	"context"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	coreapplyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	metaapplyv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/pkg/utils"
)

const (
	volumeName = "rbg-cluster-config"
	mountPath  = "/etc/rbg"
	configKey  = "config.yaml"
)

type GroupInfoInjector interface {
	InjectConfig(context context.Context, podSpec *corev1.PodTemplateSpec, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error
	InjectEnv(context context.Context, podSpec *corev1.PodTemplateSpec, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error
	InjectSidecar(context context.Context, podSpec *corev1.PodTemplateSpec, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error
}

type DefaultInjector struct {
	scheme *runtime.Scheme
	client client.Client
}

var _ GroupInfoInjector = &DefaultInjector{}

func NewDefaultInjector(scheme *runtime.Scheme, client client.Client) *DefaultInjector {
	return &DefaultInjector{
		client: client,
		scheme: scheme,
	}
}

func (i *DefaultInjector) shouldUpdateConfigMap(
	ctx context.Context,
	rbg *workloadsv1alpha1.RoleBasedGroup,
	role *workloadsv1alpha1.RoleSpec,
	clusterConfig *ClusterConfig,
) (needUpdate bool, err error) {
	logger := log.FromContext(ctx)

	cmName := rbg.GetWorkloadName(role)
	currentCM := &corev1.ConfigMap{}
	if err = i.client.Get(ctx, types.NamespacedName{Name: cmName, Namespace: rbg.Namespace}, currentCM); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	var oldClusterConfig *ClusterConfig
	if data := currentCM.Data[configKey]; data != "" {
		oldClusterConfig = &ClusterConfig{}
		if err = yaml.Unmarshal([]byte(data), oldClusterConfig); err != nil {
			oldClusterConfig = nil
			logger.Info("Failed to unmarshal old cluster config", "error", err)
		}
	}

	equal, err := i.hasClusterConfigChanged(ctx, oldClusterConfig, clusterConfig)
	if err != nil {
		return false, err
	}
	if equal {
		logger.V(1).Info("configmap equal, skip reconcile")
		return false, nil
	}
	return true, nil
}

func (i *DefaultInjector) applyConfigMap(
	ctx context.Context,
	rbg *workloadsv1alpha1.RoleBasedGroup,
	role *workloadsv1alpha1.RoleSpec,
	clusterConfig *ClusterConfig,
) error {
	configData, err := yaml.Marshal(clusterConfig)
	if err != nil {
		return err
	}

	cmApplyConfig := coreapplyv1.ConfigMap(rbg.GetWorkloadName(role), rbg.Namespace).
		WithData(map[string]string{
			configKey: string(configData),
		}).
		WithOwnerReferences(metaapplyv1.OwnerReference().
			WithAPIVersion(rbg.APIVersion).
			WithKind(rbg.Kind).
			WithName(rbg.Name).
			WithUID(rbg.GetUID()).
			WithBlockOwnerDeletion(true).
			WithController(true),
		)
	return utils.PatchObjectApplyConfiguration(ctx, i.client, cmApplyConfig, utils.PatchSpec)
}

func (i *DefaultInjector) InjectConfig(ctx context.Context, podSpec *corev1.PodTemplateSpec, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error {
	builder := &ConfigBuilder{rbg: rbg, role: role}
	clusterConfig := builder.ToClusterConfig()

	needUpdate, err := i.shouldUpdateConfigMap(ctx, rbg, role, clusterConfig)
	if err != nil {
		return err
	}
	if needUpdate {
		if err := i.applyConfigMap(ctx, rbg, role, clusterConfig); err != nil {
			return err
		}
	}

	volumeExists := false
	for _, vol := range podSpec.Spec.Volumes {
		if vol.Name == volumeName {
			volumeExists = true
			break
		}
	}
	if !volumeExists {
		podSpec.Spec.Volumes = append(podSpec.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: rbg.GetWorkloadName(role),
					},
					Items: []corev1.KeyToPath{
						{Key: configKey, Path: configKey},
					},
				},
			},
		})
	}

	for i := range podSpec.Spec.Containers {
		container := &podSpec.Spec.Containers[i]
		mountExists := false
		for _, vm := range container.VolumeMounts {
			if vm.Name == volumeName && vm.MountPath == mountPath {
				mountExists = true
				break
			}
		}
		if !mountExists {
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: mountPath,
				ReadOnly:  true,
			})
		}
	}
	return nil
}

func (i *DefaultInjector) InjectEnv(ctx context.Context, podSpec *corev1.PodTemplateSpec, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error {
	builder := &EnvBuilder{
		rbg:  rbg,
		role: role,
	}

	envVars := builder.Build()

	for idx := range podSpec.Spec.Containers {
		container := &podSpec.Spec.Containers[idx]
		// 1. Convert env to Map to remove duplicates
		existingEnv := make(map[string]corev1.EnvVar)
		for _, e := range container.Env {
			existingEnv[e.Name] = e
		}
		for _, newEnv := range envVars {
			existingEnv[newEnv.Name] = newEnv // Overwrite env.Value if the name exists
		}
		// 2. Convert back to slice
		mergedEnv := make([]corev1.EnvVar, 0, len(existingEnv))
		for _, env := range existingEnv {
			mergedEnv = append(mergedEnv, env)
		}
		// Avoid sts updates caused by env order changes
		sort.Slice(mergedEnv, func(i, j int) bool {
			return mergedEnv[i].Name < mergedEnv[j].Name
		})
		container.Env = mergedEnv
	}
	return nil
}

func (i *DefaultInjector) InjectSidecar(ctx context.Context, podSpec *corev1.PodTemplateSpec, rbg *workloadsv1alpha1.RoleBasedGroup, role *workloadsv1alpha1.RoleSpec) error {
	builder := NewSidecarBuilder(i.client, rbg, role)
	return builder.Build(ctx, podSpec)
}

func (i *DefaultInjector) hasClusterConfigChanged(
	ctx context.Context,
	oldClusterConfig *ClusterConfig,
	clusterConfig *ClusterConfig,
) (bool, error) {
	logger := log.FromContext(ctx)
	equal, diff := clusterConfigSemanticallyEqual(clusterConfig, oldClusterConfig)
	if !equal {
		logger.Info("ClusterConfig changed", "diff", diff)
	}
	return equal, nil
}
