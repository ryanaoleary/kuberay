package e2e

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rayv1 "github.com/ray-project/kuberay/ray-operator/apis/ray/v1"
	rayv1ac "github.com/ray-project/kuberay/ray-operator/pkg/client/applyconfiguration/ray/v1"
	. "github.com/ray-project/kuberay/ray-operator/test/support"
)

func TestRayJobRetry(t *testing.T) {
	test := With(t)

	// Create a namespace
	namespace := test.NewTestNamespace()
	test.StreamKubeRayOperatorLogs()

	// Job scripts
	jobsAC := newConfigMap(namespace.Name, "jobs", files(test, "fail.py"))
	jobs, err := test.Client().Core().CoreV1().ConfigMaps(namespace.Name).Apply(test.Ctx(), jobsAC, TestApplyOptions)
	test.Expect(err).NotTo(HaveOccurred())
	test.T().Logf("Created ConfigMap %s/%s successfully", jobs.Namespace, jobs.Name)

	test.T().Run("Failing RayJob without cluster shutdown after finished", func(_ *testing.T) {
		// RayJob: Set RayJob.BackoffLimit to 2
		rayJobAC := rayv1ac.RayJob("fail", namespace.Name).
			WithSpec(rayv1ac.RayJobSpec().
				WithBackoffLimit(2).
				WithSubmitterConfig(rayv1ac.SubmitterConfig().
					WithBackoffLimit(0)).
				WithRayClusterSpec(newRayClusterSpec(mountConfigMap[rayv1ac.RayClusterSpecApplyConfiguration](jobs, "/home/ray/jobs"))).
				WithEntrypoint("python /home/ray/jobs/fail.py").
				WithShutdownAfterJobFinishes(false).
				WithSubmitterPodTemplate(jobSubmitterPodTemplateApplyConfiguration()))

		rayJob, err := test.Client().Ray().RayV1().RayJobs(namespace.Name).Apply(test.Ctx(), rayJobAC, TestApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())
		test.T().Logf("Created RayJob %s/%s successfully", rayJob.Namespace, rayJob.Name)

		test.T().Logf("Waiting for RayJob %s/%s to complete", rayJob.Namespace, rayJob.Name)

		// Assert that the RayJob deployment status and RayJob reason have been updated accordingly.
		test.Eventually(RayJob(test, rayJob.Namespace, rayJob.Name), TestTimeoutLong).
			Should(WithTransform(RayJobDeploymentStatus, Equal(rayv1.JobDeploymentStatusFailed)))
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			To(WithTransform(RayJobStatus, Equal(rayv1.JobStatusFailed)))
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			To(WithTransform(RayJobReason, Equal(rayv1.AppFailed)))
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			To(WithTransform(RayJobStatus, Equal(rayv1.JobStatusFailed)))

		// Check whether the controller respects the backoffLimit.
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			Should(WithTransform(RayJobFailed, Equal(int32(3))))
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			Should(WithTransform(RayJobSucceeded, Equal(int32(0))))

		// Refresh the RayJob status
		rayJob = GetRayJob(test, rayJob.Namespace, rayJob.Name)

		// Delete the RayJob
		err = test.Client().Ray().RayV1().RayJobs(namespace.Name).Delete(test.Ctx(), rayJob.Name, metav1.DeleteOptions{})
		test.Expect(err).NotTo(HaveOccurred())
		test.T().Logf("Deleted RayJob %s/%s successfully", rayJob.Namespace, rayJob.Name)

		// Assert the RayCluster has been cascade deleted
		test.Eventually(NotFound(RayClusterOrError(test, namespace.Name, rayJob.Status.RayClusterName))).
			Should(BeTrue())

		// Assert the submitter Job has been cascade deleted
		test.Eventually(Jobs(test, namespace.Name)).Should(BeEmpty())
	})

	test.T().Run("Failing submitter K8s Job", func(_ *testing.T) {
		// RayJob: Set RayJob.BackoffLimit to 2 & SubmitterConfig.BackoffLimit to 0 to test RayJob level backoffLimit
		rayJobAC := rayv1ac.RayJob("fail-submitter-k8s-job", namespace.Name).
			WithSpec(rayv1ac.RayJobSpec().
				WithBackoffLimit(2).
				WithSubmitterConfig(rayv1ac.SubmitterConfig().
					WithBackoffLimit(0)).
				WithRayClusterSpec(newRayClusterSpec(mountConfigMap[rayv1ac.RayClusterSpecApplyConfiguration](jobs, "/home/ray/jobs"))).
				WithEntrypoint("The command will be overridden by the submitter Job").
				WithShutdownAfterJobFinishes(true).
				WithSubmitterPodTemplate(jobSubmitterPodTemplateApplyConfiguration()))

		// In this test, we try to simulate the case where the submitter Job can't connect to the RayCluster successfully.
		// Hence, KubeRay can't get the Ray job information from the RayCluster. When the RayJob reaches the backoff
		// limit, it will be marked as failed. Then, the RayJob should transition to `Failed`.
		rayJobAC.Spec.SubmitterPodTemplate.Spec.Containers[0].WithCommand("ray", "job", "submit", "--address", "http://do-not-exist:8265", "--", "echo 123")

		rayJob, err := test.Client().Ray().RayV1().RayJobs(namespace.Name).Apply(test.Ctx(), rayJobAC, TestApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())
		test.T().Logf("Created RayJob %s/%s successfully", rayJob.Namespace, rayJob.Name)
		test.T().Logf("Waiting for RayJob %s/%s to complete", rayJob.Namespace, rayJob.Name)

		// Ensure JobDeploymentStatus transit to Failed
		test.Eventually(RayJob(test, rayJob.Namespace, rayJob.Name), TestTimeoutMedium).
			Should(WithTransform(RayJobDeploymentStatus, Equal(rayv1.JobDeploymentStatusFailed)))
		// Ensure JobStatus is empty
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			To(WithTransform(RayJobStatus, Equal(rayv1.JobStatusNew)))
		// Ensure Reason is SubmissionFailed
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			To(WithTransform(RayJobReason, Equal(rayv1.SubmissionFailed)))

		// Check whether the controller respects the backoffLimit.
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			Should(WithTransform(RayJobFailed, Equal(int32(3))))
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			Should(WithTransform(RayJobSucceeded, Equal(int32(0))))

		// Refresh the RayJob status
		rayJob = GetRayJob(test, rayJob.Namespace, rayJob.Name)

		// Assert the RayCluster has been deleted because ShutdownAfterJobFinishes is true.
		test.Eventually(NotFound(RayClusterOrError(test, namespace.Name, rayJob.Status.RayClusterName)), TestTimeoutMedium).
			Should(BeTrue())
		// Asset submitter Job is not deleted yet
		test.Eventually(Jobs(test, namespace.Name)).ShouldNot(BeEmpty())

		// Delete the RayJob
		err = test.Client().Ray().RayV1().RayJobs(namespace.Name).Delete(test.Ctx(), rayJob.Name, metav1.DeleteOptions{})
		test.Expect(err).NotTo(HaveOccurred())
		test.T().Logf("Deleted RayJob %s/%s successfully", rayJob.Namespace, rayJob.Name)
	})

	test.T().Run("RayJob has passed ActiveDeadlineSeconds", func(_ *testing.T) {
		// RayJob will transition to JobDeploymentStatusFailed
		// regardless of the value of backoffLimit.
		rayJobAC := rayv1ac.RayJob("long-running", namespace.Name).
			WithSpec(rayv1ac.RayJobSpec().
				WithBackoffLimit(2).
				WithSubmitterConfig(rayv1ac.SubmitterConfig().
					WithBackoffLimit(0)).
				WithRayClusterSpec(newRayClusterSpec(mountConfigMap[rayv1ac.RayClusterSpecApplyConfiguration](jobs, "/home/ray/jobs"))).
				WithEntrypoint("python /home/ray/jobs/long_running.py").
				WithShutdownAfterJobFinishes(true).
				WithTTLSecondsAfterFinished(600).
				WithActiveDeadlineSeconds(5).
				WithSubmitterPodTemplate(jobSubmitterPodTemplateApplyConfiguration()))

		rayJob, err := test.Client().Ray().RayV1().RayJobs(namespace.Name).Apply(test.Ctx(), rayJobAC, TestApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())
		test.T().Logf("Created RayJob %s/%s successfully", rayJob.Namespace, rayJob.Name)

		// The RayJob will transition to `Failed` because it has passed `ActiveDeadlineSeconds`.
		test.T().Logf("Waiting for RayJob %s/%s to be 'Failed'", rayJob.Namespace, rayJob.Name)

		test.Eventually(RayJob(test, rayJob.Namespace, rayJob.Name), TestTimeoutShort).
			Should(WithTransform(RayJobDeploymentStatus, Equal(rayv1.JobDeploymentStatusFailed)))
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			To(WithTransform(RayJobReason, Equal(rayv1.DeadlineExceeded)))

		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			Should(WithTransform(RayJobFailed, Equal(int32(1))))
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			Should(WithTransform(RayJobSucceeded, Equal(int32(0))))
	})

	test.T().Run("Failing RayJob with HttpMode submission mode", func(_ *testing.T) {
		// Set up the RayJob with HTTP mode and a BackoffLimit
		rayJobAC := rayv1ac.RayJob("failing-rayjob-in-httpmode", namespace.Name).
			WithSpec(rayv1ac.RayJobSpec().
				WithSubmissionMode(rayv1.HTTPMode).
				WithBackoffLimit(2).
				WithEntrypoint("python /home/ray/jobs/fail.py").
				WithShutdownAfterJobFinishes(false).
				WithRayClusterSpec(newRayClusterSpec(mountConfigMap[rayv1ac.RayClusterSpecApplyConfiguration](jobs, "/home/ray/jobs"))))

		rayJob, err := test.Client().Ray().RayV1().RayJobs(namespace.Name).Apply(test.Ctx(), rayJobAC, TestApplyOptions)
		test.Expect(err).NotTo(HaveOccurred())
		test.T().Logf("Created RayJob %s/%s successfully", rayJob.Namespace, rayJob.Name)

		test.T().Logf("Waiting for RayJob %s/%s to complete", rayJob.Namespace, rayJob.Name)

		// Assert that the RayJob deployment status has been updated.
		test.Eventually(RayJob(test, rayJob.Namespace, rayJob.Name), TestTimeoutMedium).
			Should(WithTransform(RayJobDeploymentStatus, Equal(rayv1.JobDeploymentStatusFailed)))

		// Assert the Ray job has failed.
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			To(WithTransform(RayJobStatus, Equal(rayv1.JobStatusFailed)))

		// Check the RayJob reason has been updated.
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			To(WithTransform(RayJobReason, Equal(rayv1.AppFailed)))

		// Check whether the controller respects the backoffLimit.
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			Should(WithTransform(RayJobFailed, Equal(int32(3)))) // 2 retries + 1 initial attempt = 3 failures
		test.Expect(GetRayJob(test, rayJob.Namespace, rayJob.Name)).
			Should(WithTransform(RayJobSucceeded, Equal(int32(0))))

		// Clean up
		err = test.Client().Ray().RayV1().RayJobs(namespace.Name).Delete(test.Ctx(), rayJob.Name, metav1.DeleteOptions{})
		test.Expect(err).NotTo(HaveOccurred())
		test.T().Logf("Deleted RayJob %s/%s successfully", rayJob.Namespace, rayJob.Name)
	})
}
