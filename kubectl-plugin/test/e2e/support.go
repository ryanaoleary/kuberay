package e2e

import (
	"math/rand"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

func randStringBytes(n int) string {
	// Reference: https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go/22892986
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))] //nolint:gosec // Don't need cryptographically secure random number
	}
	return string(b)
}

func createTestNamespace() string {
	GinkgoHelper()
	suffix := randStringBytes(5)
	ns := "test-ns-" + suffix
	cmd := exec.Command("kubectl", "create", "namespace", ns)
	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())
	nsWithPrefix := "namespace/" + ns
	cmd = exec.Command("kubectl", "wait", "--timeout=20s", "--for", "jsonpath={.status.phase}=Active", nsWithPrefix)
	err = cmd.Run()
	Expect(err).NotTo(HaveOccurred())
	return ns
}

func deleteTestNamespace(ns string) {
	GinkgoHelper()
	cmd := exec.Command("kubectl", "delete", "namespace", ns)
	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())
}

func deployTestRayCluster(ns string) {
	GinkgoHelper()
	// Print current working directory
	cmd := exec.Command("kubectl", "apply", "-f", "../../../ray-operator/config/samples/ray-cluster.sample.yaml", "-n", ns)
	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())
	cmd = exec.Command("kubectl", "wait", "--timeout=300s", "--for", "jsonpath={.status.state}=ready", "raycluster/raycluster-kuberay", "-n", ns)
	err = cmd.Run()
	Expect(err).NotTo(HaveOccurred())
}