package create

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"

	"github.com/ray-project/kuberay/kubectl-plugin/pkg/util/client"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

type CreateWorkerGroupOptions struct {
	configFlags       *genericclioptions.ConfigFlags
	ioStreams         *genericclioptions.IOStreams
	clusterName       string
	groupName         string
	rayVersion        string
	image             string
	workerCPU         string
	workerGPU         string
	workerMemory      string
	workerReplicas    int32
	workerMinReplicas int32
	workerMaxReplicas int32
}

var (
	createWorkerGroupLong = templates.LongDesc(`
		Adds a worker group to an existing RayCluster.
	`)

	createWorkerGroupExample = templates.Examples(`
		# Create a worker group in an existing RayCluster
		kubectl ray create worker-group example-group --cluster sample-cluster --image rayproject/ray:2.39.0 --worker-cpu=2 --worker-memory=5Gi
	`)
)

func NewCreateWorkerGroupOptions(streams genericclioptions.IOStreams) *CreateWorkerGroupOptions {
	return &CreateWorkerGroupOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		ioStreams:   &streams,
	}
}

func NewCreateWorkerGroupCommand(streams genericclioptions.IOStreams) *cobra.Command {
	options := NewCreateWorkerGroupOptions(streams)
	cmdFactory := cmdutil.NewFactory(options.configFlags)
	// Silence warnings to avoid messages like 'unknown field "spec.headGroupSpec.template.metadata.creationTimestamp"'
	// See https://github.com/kubernetes/kubernetes/issues/67610 for more details.
	rest.SetDefaultWarningHandler(rest.NoWarnings{})

	cmd := &cobra.Command{
		Use:          "workergroup [WORKERGROUP]",
		Short:        "Create worker group in an existing RayCluster",
		Long:         createWorkerGroupLong,
		Example:      createWorkerGroupExample,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Complete(cmd, args); err != nil {
				return err
			}
			if err := options.Validate(); err != nil {
				return err
			}
			return options.Run(cmd.Context(), cmdFactory)
		},
	}

	cmd.Flags().StringVar(&options.clusterName, "ray-cluster", "", "The name of the RayCluster to add a worker group.")
	cmd.Flags().StringVar(&options.rayVersion, "ray-version", "2.39.0", "Ray Version to use in the Ray Cluster yaml. Default to 2.39.0")
	cmd.Flags().StringVar(&options.image, "image", options.image, "Ray image to use in the Ray Cluster yaml")
	cmd.Flags().Int32Var(&options.workerReplicas, "worker-replicas", 1, "Number of the worker group replicas. Default of 1")
	cmd.Flags().Int32Var(&options.workerMinReplicas, "worker-min-replicas", 1, "Number of the worker group replicas. Default of 10")
	cmd.Flags().Int32Var(&options.workerMaxReplicas, "worker-max-replicas", 10, "Number of the worker group replicas. Default of 10")
	cmd.Flags().StringVar(&options.workerCPU, "worker-cpu", "2", "Number of CPU for the ray worker. Default to 2")
	cmd.Flags().StringVar(&options.workerGPU, "worker-gpu", "0", "Number of GPU for the ray worker. Default to 0")
	cmd.Flags().StringVar(&options.workerMemory, "worker-memory", "4Gi", "Amount of memory to use for the ray worker. Default to 4Gi")

	options.configFlags.AddFlags(cmd.Flags())
	return cmd
}

func (options *CreateWorkerGroupOptions) Complete(cmd *cobra.Command, args []string) error {
	if *options.configFlags.Namespace == "" {
		*options.configFlags.Namespace = "default"
	}

	if len(args) != 1 {
		return cmdutil.UsageErrorf(cmd, "%s", cmd.Use)
	}
	options.groupName = args[0]

	if options.image == "" {
		options.image = fmt.Sprintf("rayproject/ray:%s", options.rayVersion)
	}

	return nil
}

func (options *CreateWorkerGroupOptions) Validate() error {
	config, err := options.configFlags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return fmt.Errorf("Error retrieving raw config: %w", err)
	}
	if len(config.CurrentContext) == 0 {
		return fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
	}

	return nil
}

func (options *CreateWorkerGroupOptions) Run(ctx context.Context, factory cmdutil.Factory) error {
	k8sClient, err := client.NewClient(factory)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	rayCluster, err := k8sClient.RayClient().RayV1().RayClusters(*options.configFlags.Namespace).Get(ctx, options.clusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting RayCluster: %w", err)
	}

	newRayCluster := rayCluster.DeepCopy()
	podTemplate := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "ray-worker",
					Image: options.image,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(options.workerCPU),
							corev1.ResourceMemory: resource.MustParse(options.workerMemory),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse(options.workerMemory),
						},
					},
				},
			},
		},
	}

	gpuResource := resource.MustParse(options.workerGPU)
	if !gpuResource.IsZero() {
		podTemplate.Spec.Containers[0].Resources.Requests[corev1.ResourceName("nvidia.com/gpu")] = gpuResource
		podTemplate.Spec.Containers[0].Resources.Limits[corev1.ResourceName("nvidia.com/gpu")] = gpuResource
	}

	workerGroup := rayv1.WorkerGroupSpec{
		GroupName:      options.groupName,
		Replicas:       &options.workerReplicas,
		MinReplicas:    &options.workerMinReplicas,
		MaxReplicas:    &options.workerMaxReplicas,
		RayStartParams: map[string]string{},
		Template:       podTemplate,
	}
	newRayCluster.Spec.WorkerGroupSpecs = append(newRayCluster.Spec.WorkerGroupSpecs, workerGroup)

	newRayCluster, err = k8sClient.RayClient().RayV1().RayClusters(*options.configFlags.Namespace).Update(ctx, newRayCluster, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating RayCluster with new worker group: %w", err)
	}

	fmt.Printf("Updated RayCluster %s/%s with new worker group\n", newRayCluster.Namespace, newRayCluster.Name)
	return nil
}
