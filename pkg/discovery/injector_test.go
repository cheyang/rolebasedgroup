package discovery

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	workloadsv1alpha "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
)

func TestInjectSidecar(t *testing.T) {
	// Initialize test scheme with required types
	testScheme := runtime.NewScheme()
	_ = workloadsv1alpha.AddToScheme(testScheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithRuntimeObjects(&workloadsv1alpha.ClusterEngineRuntimeProfile{
			ObjectMeta: metav1.ObjectMeta{Name: "patio-runtime"},
			Spec: workloadsv1alpha.ClusterEngineRuntimeProfileSpec{
				InitContainers: []corev1.Container{
					{
						Name:  "init-patio-runtime",
						Image: "init-container-image",
					},
				},
				Containers: []corev1.Container{
					{
						Name:  "patio-runtime",
						Image: "sidecar-image",
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "patio-runtime-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		}).Build()

	rbg := &workloadsv1alpha.RoleBasedGroup{
		Spec: workloadsv1alpha.RoleBasedGroupSpec{
			Roles: []workloadsv1alpha.RoleSpec{
				{
					Name: "test",
					EngineRuntimes: []workloadsv1alpha.EngineRuntime{
						{
							ProfileName: "patio-runtime",
							Containers: []corev1.Container{
								{
									Name: "patio-runtime",
									Args: []string{"--foo=bar"},
									Env: []corev1.EnvVar{
										{
											Name:  "INFERENCE_ENGINE",
											Value: "vLLM",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		podSpec *corev1.PodTemplateSpec
		want    *corev1.PodTemplateSpec
	}{
		{
			name: "Add init & sidecar & volume to pod",
			podSpec: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "test-image",
						},
					},
				},
			},
			want: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "init-patio-runtime",
							Image: "init-container-image",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "test-image",
						},
						{
							Name:  "patio-runtime",
							Image: "sidecar-image",
							Args:  []string{"--foo=bar"},
							Env: []corev1.EnvVar{
								{
									Name:  "INFERENCE_ENGINE",
									Value: "vLLM",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "patio-runtime-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
		{
			name: "Add duplicated init & sidecar & volume to pod",
			podSpec: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "init-patio-runtime",
							Image: "init-container-image",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "test-image",
						},
						{
							Name:  "patio-runtime",
							Image: "sidecar-image",
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "patio-runtime-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			want: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "init-patio-runtime",
							Image: "init-container-image",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "test-image",
						},
						{
							Name:  "patio-runtime",
							Image: "sidecar-image",
							Args:  []string{"--foo=bar"},
							Env: []corev1.EnvVar{
								{
									Name:  "INFERENCE_ENGINE",
									Value: "vLLM",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "patio-runtime-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, _ := rbg.GetRole("test")
			b := NewSidecarBuilder(fakeClient, rbg, role)
			err := b.Build(context.TODO(), tt.podSpec)
			if err != nil {
				t.Errorf("build error: %s", err.Error())
			}
			if !reflect.DeepEqual(tt.podSpec, tt.want) {
				t.Errorf("Build expect err, want %v, got %v", tt.want, tt.podSpec)
			}

		})
	}
}

func TestDefaultInjector_hasClusterConfigChanged(t *testing.T) {

}
