package validation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nginxv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas"
	"github.com/tsuru/rpaas-operator/internal/pkg/rpaas/fake"
)

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	networkingv1.AddToScheme(scheme)
	v1alpha1.SchemeBuilder.AddToScheme(scheme)
	metricsv1beta1.SchemeBuilder.AddToScheme(scheme)
	nginxv1alpha1.SchemeBuilder.AddToScheme(scheme)
	return scheme
}

func TestUpdateBlock(t *testing.T) {
	block := rpaas.ConfigurationBlock{
		Name:    "http",
		Content: "blah;",
	}

	cli := clientFake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects().Build()

	baseManager := &fake.RpaasManager{
		FakeUpdateBlock: func(instance string, updateBlock rpaas.ConfigurationBlock) error {
			assert.Equal(t, instance, "blah")
			assert.Equal(t, block, updateBlock)
			return nil
		},
		FakeGetInstance: func(instanceName string) (*v1alpha1.RpaasInstance, error) {
			return &v1alpha1.RpaasInstance{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      instanceName,
				},
				Spec: v1alpha1.RpaasInstanceSpec{},
			}, nil
		},
	}

	stop := fakeValidationController(cli, true, "", nil)
	defer stop()

	validationMngr := New(baseManager, cli)

	err := validationMngr.UpdateBlock(context.TODO(), "blah", block)

	require.NoError(t, err)
}

func TestUpdateBlockControllerError(t *testing.T) {
	block := rpaas.ConfigurationBlock{
		Name:    "http",
		Content: "blah;",
	}

	cli := clientFake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects().Build()

	baseManager := &fake.RpaasManager{
		FakeUpdateBlock: func(instance string, updateBlock rpaas.ConfigurationBlock) error {
			assert.Equal(t, instance, "blah")
			assert.Equal(t, block, updateBlock)
			return nil
		},
		FakeGetInstance: func(instanceName string) (*v1alpha1.RpaasInstance, error) {
			return &v1alpha1.RpaasInstance{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      instanceName,
				},
				Spec: v1alpha1.RpaasInstanceSpec{},
			}, nil
		},
	}

	stop := fakeValidationController(cli, false, "rpaas-operator error", nil)
	defer stop()

	validationMngr := New(baseManager, cli)

	err := validationMngr.UpdateBlock(context.TODO(), "blah", block)

	require.Equal(t, &rpaas.ValidationError{Msg: "rpaas-operator error"}, err)
}

func TestDeleteBlock(t *testing.T) {
	cli := clientFake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects().Build()

	baseManager := &fake.RpaasManager{
		FakeDeleteBlock: func(instance string, blockName string) error {
			assert.Equal(t, instance, "blah")
			assert.Equal(t, "http", blockName)
			return nil
		},
		FakeGetInstance: func(instanceName string) (*v1alpha1.RpaasInstance, error) {
			return &v1alpha1.RpaasInstance{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      instanceName,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Blocks: map[v1alpha1.BlockType]v1alpha1.Value{
						"http": {
							Value: "blah;",
						},
					},
				},
			}, nil
		},
	}

	stop := fakeValidationController(cli, true, "", nil)
	defer stop()

	validationMngr := New(baseManager, cli)

	err := validationMngr.DeleteBlock(context.TODO(), "blah", "http")

	require.NoError(t, err)
}

func TestDeleteBlockError(t *testing.T) {
	cli := clientFake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects().Build()

	baseManager := &fake.RpaasManager{
		FakeDeleteBlock: func(instance string, blockName string) error {
			assert.Equal(t, instance, "blah")
			assert.Equal(t, "http", blockName)
			return nil
		},
		FakeGetInstance: func(instanceName string) (*v1alpha1.RpaasInstance, error) {
			return &v1alpha1.RpaasInstance{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      instanceName,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Blocks: map[v1alpha1.BlockType]v1alpha1.Value{
						"http": {
							Value: "blah;",
						},
					},
				},
			}, nil
		},
	}

	stop := fakeValidationController(cli, false, "validation error from rpaas-operator", nil)
	defer stop()

	validationMngr := New(baseManager, cli)

	err := validationMngr.DeleteBlock(context.TODO(), "blah", "http")

	require.Equal(t, &rpaas.ValidationError{Msg: "validation error from rpaas-operator"}, err)
}

func TestCreateExtraFiles(t *testing.T) {

	cli := clientFake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects().Build()

	baseManager := &fake.RpaasManager{
		FakeCreateExtraFiles: func(instance string, files ...rpaas.File) error {
			assert.Equal(t, instance, "blah")
			assert.Equal(t, rpaas.File{
				Name:    "myfile",
				Content: []byte("mycontent"),
			}, files[0])

			return nil
		},
		FakeGetInstance: func(instanceName string) (*v1alpha1.RpaasInstance, error) {
			return &v1alpha1.RpaasInstance{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      instanceName,
				},
				Spec: v1alpha1.RpaasInstanceSpec{},
			}, nil
		},
	}

	preUpdate := func() {
		defer recover()
		configMaps := corev1.ConfigMapList{}

		err := cli.List(context.Background(), &configMaps)

		if err != nil {
			fmt.Println("stop controller", err)
		}

		require.Len(t, configMaps.Items, 1)
		assert.True(t, strings.HasPrefix(configMaps.Items[0].Name, "blah-validation-file-"))
		assert.Equal(t, map[string][]byte{
			"myfile": []byte("mycontent"),
		}, configMaps.Items[0].BinaryData)
	}

	stop := fakeValidationController(cli, true, "", preUpdate)
	defer stop()

	validationMngr := New(baseManager, cli)

	err := validationMngr.CreateExtraFiles(context.TODO(), "blah", rpaas.File{
		Name:    "myfile",
		Content: []byte("mycontent"),
	})

	require.NoError(t, err)

	rpaas := v1alpha1.RpaasValidation{}
	err = cli.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "blah"}, &rpaas)
	require.NoError(t, err)

	require.Len(t, rpaas.Spec.Files, 1)
	assert.Equal(t, "myfile", rpaas.Spec.Files[0].Name)
	assert.Equal(t, "myfile", rpaas.Spec.Files[0].ConfigMap.Key)
	assert.True(t, strings.HasPrefix(rpaas.Spec.Files[0].ConfigMap.LocalObjectReference.Name, "blah-validation-file-"))
}

func TestUpdateExtraFiles(t *testing.T) {

	cli := clientFake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects().Build()

	baseManager := &fake.RpaasManager{
		FakeCreateExtraFiles: func(instance string, files ...rpaas.File) error {
			assert.Equal(t, instance, "blah")
			assert.Equal(t, rpaas.File{
				Name:    "myfile",
				Content: []byte("mycontent"),
			}, files[0])

			return nil
		},
		FakeGetInstance: func(instanceName string) (*v1alpha1.RpaasInstance, error) {
			return &v1alpha1.RpaasInstance{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      instanceName,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Files: []v1alpha1.File{
						{
							Name: "myfile",
							ConfigMap: &corev1.ConfigMapKeySelector{
								Key: "myfile",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "myconfigmap",
								},
							},
						},
					},
				},
			}, nil
		},
	}

	preUpdate := func() {
		defer recover()
		configMaps := corev1.ConfigMapList{}

		err := cli.List(context.Background(), &configMaps)

		if err != nil {
			fmt.Println("stop controller", err)
		}

		require.Len(t, configMaps.Items, 1)
		assert.True(t, strings.HasPrefix(configMaps.Items[0].Name, "blah-validation-file-"))
		assert.Equal(t, map[string][]byte{
			"myfile": []byte("mycontent"),
		}, configMaps.Items[0].BinaryData)
	}

	stop := fakeValidationController(cli, true, "", preUpdate)
	defer stop()

	validationMngr := New(baseManager, cli)

	err := validationMngr.UpdateExtraFiles(context.TODO(), "blah", rpaas.File{
		Name:    "myfile",
		Content: []byte("mycontent"),
	})

	require.NoError(t, err)

	rpaas := v1alpha1.RpaasValidation{}
	err = cli.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "blah"}, &rpaas)
	require.NoError(t, err)

	require.Len(t, rpaas.Spec.Files, 1)
	assert.Equal(t, "myfile", rpaas.Spec.Files[0].Name)
	assert.Equal(t, "myfile", rpaas.Spec.Files[0].ConfigMap.Key)
	assert.True(t, strings.HasPrefix(rpaas.Spec.Files[0].ConfigMap.LocalObjectReference.Name, "blah-validation-file-"))
}

func TestDeleteExtraFiles(t *testing.T) {

	cli := clientFake.NewClientBuilder().WithScheme(newScheme()).WithRuntimeObjects().Build()

	baseManager := &fake.RpaasManager{
		FakeCreateExtraFiles: func(instance string, files ...rpaas.File) error {
			assert.Equal(t, instance, "blah")
			assert.Equal(t, rpaas.File{
				Name:    "myfile",
				Content: []byte("mycontent"),
			}, files[0])

			return nil
		},
		FakeGetInstance: func(instanceName string) (*v1alpha1.RpaasInstance, error) {
			return &v1alpha1.RpaasInstance{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      instanceName,
				},
				Spec: v1alpha1.RpaasInstanceSpec{
					Files: []v1alpha1.File{
						{
							Name: "myfile",
							ConfigMap: &corev1.ConfigMapKeySelector{
								Key: "myfile",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "myconfigmap",
								},
							},
						},
					},
				},
			}, nil
		},
	}

	stop := fakeValidationController(cli, true, "", nil)
	defer stop()

	validationMngr := New(baseManager, cli)

	err := validationMngr.DeleteExtraFiles(context.TODO(), "blah", "myfile")

	require.NoError(t, err)

	rpaas := v1alpha1.RpaasValidation{}
	err = cli.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "blah"}, &rpaas)
	require.NoError(t, err)

	require.Len(t, rpaas.Spec.Files, 0)
}

func fakeValidationController(cli client.Client, valid bool, errorMesssage string, preUpdate func()) (stop func()) {
	running := true
	stop = func() {
		running = false
	}

	go func() {
		for {
			list := v1alpha1.RpaasValidationList{}
			err := cli.List(context.Background(), &list, &client.ListOptions{})
			if err != nil {
				fmt.Println("stop controller", err)
			}

		itemsLoop:
			for _, item := range list.Items {
				if item.Status.Valid != nil {
					continue itemsLoop
				}

				if preUpdate != nil {
					preUpdate()
				}

				item.Status.Valid = &valid
				item.Status.Error = errorMesssage

				cli.Update(context.Background(), &item)
				if err != nil {
					fmt.Println("stop controller", err)
				}
			}

			if !running {
				break
			}

			time.Sleep(time.Millisecond * 100)
		}
	}()

	return stop
}
