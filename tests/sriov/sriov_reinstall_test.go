package sriov

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/namespace"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/nodes"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/pod"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/params"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	reinstallTestNS *namespace.Builder
)

// getReinstallTestNS returns the reinstall test namespace, initializing it if necessary
func getReinstallTestNS() *namespace.Builder {
	if reinstallTestNS == nil {
		reinstallTestNS = namespace.NewBuilder(getAPIClient(), "sriov-reinstall-test")
	}
	return reinstallTestNS
}

var _ = Describe("[sig-networking] SR-IOV Operator Reinstallation", Label("reinstall"), func() {
	defer GinkgoRecover()
	var (
		sriovOpNs   = NetConfig.SriovOperatorNamespace
		vfNum       = getVFNum()
		workerNodes []*nodes.Builder
	)

	testData := getDeviceConfig()

	BeforeEach(func() {
		By("Verifying SR-IOV operator status before test")
		chkSriovOperatorStatus(sriovOpNs)

		By("Discovering worker nodes")
		var err error
		workerNodes, err = nodes.List(getAPIClient(),
			metav1.ListOptions{LabelSelector: labels.Set(NetConfig.WorkerLabelMap).String()})
		Expect(err).ToNot(HaveOccurred(), "Failed to discover worker nodes")
	})

	It("test_sriov_operator_control_plane_before_removal - Validate control plane operational before removal [Disruptive] [Serial]", func() {
		By("Step 1: Validating operator pods are running")
		err := validateOperatorControlPlane(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Control plane validation failed")

		By("Step 2: Checking CSV status")
		csv, err := getOperatorCSV(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to get operator CSV")
		Expect(csv.Definition.Status.Phase).To(Equal("Succeeded"), "CSV should be in Succeeded phase")
		GinkgoLogr.Info("CSV validation passed", "csvName", csv.Definition.Name, "phase", csv.Definition.Status.Phase)

		By("Step 3: Checking Subscription status")
		sub, err := getOperatorSubscription(getAPIClient(), sriovOpNs)
		if err != nil {
			GinkgoLogr.Info("Subscription not found, may be managed differently", "error", err)
		} else {
			Expect(sub.Object.Status.State).ToNot(Equal("Failed"), "Subscription should not be in failed state")
			GinkgoLogr.Info("Subscription validation passed", "name", sub.Definition.Name, "state", sub.Object.Status.State)
		}

		By("Step 4: Validating SriovNetworkNodeState resources")
		err = validateNodeStatesReconciled(getAPIClient(), sriovOpNs, 5*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Node states should be synced")

		By("Step 5: Capturing baseline configuration state")
		state, err := captureSriovState(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to capture SR-IOV state")
		GinkgoLogr.Info("Baseline state captured",
			"policies", len(state.Policies),
			"networks", len(state.Networks),
			"nodeStates", len(state.NodeStates))

		By("Control plane validation completed successfully")
	})

	It("test_sriov_operator_data_plane_before_removal - Validate data plane operational before removal [Disruptive] [Serial]", func() {
		executed := false
		var testDeviceConfig deviceConfig
		var testNamespace string
		var testNetworkName string

		// Find a suitable device for testing
		for _, data := range testData {
			result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
			if result {
				testDeviceConfig = data
				executed = true
				break
			}
		}

		if !executed {
			Skip("No SR-IOV devices available for data plane testing")
		}

	// Use timestamp suffix to avoid namespace collision from previous test runs (fixes race condition in namespace termination)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	testNamespace = "e2e-reinstall-dataplane-" + testDeviceConfig.Name + "-" + timestamp
	testNetworkName = "reinstall-test-net-" + testDeviceConfig.Name

		// Create namespace for data plane test
		nsBuilder := namespace.NewBuilder(getAPIClient(), testNamespace)
		for key, value := range params.PrivilegedNSLabels {
			nsBuilder.WithLabel(key, value)
		}
		_, err := nsBuilder.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

		defer func() {
			By("Cleaning up data plane test resources")
			// Clean up network
			rmSriovNetwork(testNetworkName, sriovOpNs)
			// Clean up policy
			rmSriovPolicy(testDeviceConfig.Name, sriovOpNs)
			// Clean up namespace
			err := nsBuilder.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete test namespace", "namespace", testNamespace, "error", err)
			}
		}()

		By("Step 1: Creating SR-IOV network for data plane test")
		sriovNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-whereabouts-template.yaml")
		sriovnetwork := sriovNetwork{
			name:             testNetworkName,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespace,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
			spoolchk:         "off",
			trust:            "on",
		}
		sriovnetwork.createSriovNetwork()

		By("Step 2: Creating test pods with SR-IOV interfaces")
		clientPod := createTestPod("client-dp", testNamespace, testNetworkName, "192.168.10.10/24", "20:04:0f:f1:99:01")
		serverPod := createTestPod("server-dp", testNamespace, testNetworkName, "192.168.10.11/24", "20:04:0f:f1:99:02")

		By("Step 3: Waiting for pods to be ready")
	err = clientPod.WaitUntilReady(10 * time.Minute)
	Expect(err).ToNot(HaveOccurred(), "Client pod should be ready")

	err = serverPod.WaitUntilReady(10 * time.Minute)
	Expect(err).ToNot(HaveOccurred(), "Server pod should be ready")

		By("Step 4: Validating SR-IOV interfaces on pods")
		err = verifyPodSriovInterface(clientPod, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "Client pod should have SR-IOV interface")

		err = verifyPodSriovInterface(serverPod, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "Server pod should have SR-IOV interface")

		By("Step 5: Validating connectivity between pods")
		err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.10.11")
		Expect(err).ToNot(HaveOccurred(), "Pods should be able to communicate")

		By("Data plane validation completed successfully")
	})

	It("test_sriov_operator_reinstallation_functionality - Validate functionality after reinstallation [Disruptive] [Serial]", func() {
		var beforeState *SriovClusterState
		var testDeviceConfig deviceConfig
		var testNamespace string
		var testNetworkName string
		var clientPod, serverPod *pod.Builder
		executed := false

	// ==================== SETUP PHASE ====================
	By("SETUP: Creating test configuration with SR-IOV workloads")

	// IMPORTANT: Capture the current Subscription BEFORE any operator removal
	// This ensures we can restore with the exact same configuration
	By("Capturing operator Subscription configuration for later restoration")
	capturedSubscription, err := getOperatorSubscription(getAPIClient(), sriovOpNs)
	if err != nil {
		GinkgoLogr.Info("Warning: Could not capture subscription, will use default restoration", "error", err)
	} else {
		GinkgoLogr.Info("Operator Subscription captured successfully", 
			"name", capturedSubscription.Definition.Name,
			"channel", capturedSubscription.Definition.Spec.Channel,
			"source", capturedSubscription.Definition.Spec.Source)
	}

	// Find a suitable device for testing
	for _, data := range testData {
		result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
		if result {
			testDeviceConfig = data
			executed = true
			break
		}
	}

	if !executed {
		Skip("No SR-IOV devices available for reinstallation testing")
	}

	// Use timestamp suffix to avoid namespace collision from previous test runs (fixes race condition in namespace termination)
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	testNamespace = "e2e-reinstall-full-" + testDeviceConfig.Name + "-" + timestamp
	testNetworkName = "reinstall-full-net-" + testDeviceConfig.Name

	// Create namespace
	nsBuilder := namespace.NewBuilder(getAPIClient(), testNamespace)
	for key, value := range params.PrivilegedNSLabels {
		nsBuilder.WithLabel(key, value)
	}
	_, err = nsBuilder.Create()
	Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

		defer func() {
			By("CLEANUP: Removing all test resources")
			// Delete pods if they exist
			if clientPod != nil {
				clientPod.DeleteAndWait(60 * time.Second)
			}
			if serverPod != nil {
				serverPod.DeleteAndWait(60 * time.Second)
			}
			// Clean up network
			rmSriovNetwork(testNetworkName, sriovOpNs)
			// Clean up policy
			rmSriovPolicy(testDeviceConfig.Name, sriovOpNs)
			// Clean up namespace
			err := nsBuilder.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete test namespace", "namespace", testNamespace, "error", err)
			}
		}()

		// Create SR-IOV network
		sriovNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-whereabouts-template.yaml")
		sriovnetwork := sriovNetwork{
			name:             testNetworkName,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespace,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
			spoolchk:         "off",
			trust:            "on",
		}
		sriovnetwork.createSriovNetwork()

		// Create test pods
		clientPod = createTestPod("client-full", testNamespace, testNetworkName, "192.168.20.10/24", "20:04:0f:f1:88:01")
		serverPod = createTestPod("server-full", testNamespace, testNetworkName, "192.168.20.11/24", "20:04:0f:f1:88:02")

	err = clientPod.WaitUntilReady(10 * time.Minute)
	Expect(err).ToNot(HaveOccurred(), "Client pod should be ready before operator removal")

	err = serverPod.WaitUntilReady(10 * time.Minute)
	Expect(err).ToNot(HaveOccurred(), "Server pod should be ready before operator removal")

		// Verify initial connectivity
		err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.20.11")
		Expect(err).ToNot(HaveOccurred(), "Initial connectivity should work")

		// Capture state before removal
		beforeState, err = captureSriovState(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to capture state before removal")

		// ==================== PHASE 1: OPERATOR REMOVAL ====================
		By("PHASE 1: Removing SR-IOV operator via OLM")

		By("Phase 1.1: Deleting CSV")
		err = deleteOperatorCSV(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to delete operator CSV")

		By("Phase 1.2: Verifying operator pods are terminated")
		Eventually(func() bool {
			podList := &corev1.PodList{}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := getAPIClient().Client.List(ctx, podList, &client.ListOptions{
				Namespace: sriovOpNs,
			})
			if err != nil {
				return false
			}

			runningPods := 0
			for _, pod := range podList.Items {
				if pod.Status.Phase == corev1.PodRunning {
					runningPods++
				}
			}

			GinkgoLogr.Info("Checking operator pod termination", "runningPods", runningPods)
			return runningPods == 0
		}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "Operator pods should be terminated")

		By("Phase 1.3: Verifying CRDs still exist")
		_, err = captureSriovState(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "CRDs should still be accessible after operator removal")

		By("Phase 1.4: Verifying workload pods remain operational")
		// Re-pull pods to get latest status
		clientPod, err = pod.Pull(getAPIClient(), clientPod.Definition.Name, clientPod.Definition.Namespace)
		Expect(err).ToNot(HaveOccurred(), "Client pod should still exist")
		Expect(clientPod.Definition.Status.Phase).To(Equal(corev1.PodRunning), "Client pod should still be running")

		serverPod, err = pod.Pull(getAPIClient(), serverPod.Definition.Name, serverPod.Definition.Namespace)
		Expect(err).ToNot(HaveOccurred(), "Server pod should still exist")
		Expect(serverPod.Definition.Status.Phase).To(Equal(corev1.PodRunning), "Server pod should still be running")

		// Verify connectivity still works
		err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.20.11")
		Expect(err).ToNot(HaveOccurred(), "Workload connectivity should still work after operator removal")

		GinkgoLogr.Info("Phase 1 completed: Operator removed, workloads still operational")

	// ==================== PHASE 2: OPERATOR REINSTALLATION ====================
	By("PHASE 2: Reinstalling SR-IOV operator via OLM")

	By("Phase 2.1: Triggering operator reinstallation using captured Subscription configuration")
	// Use the subscription we captured BEFORE deletion to ensure exact restoration
	if capturedSubscription != nil {
		GinkgoLogr.Info("Restoring operator with captured Subscription configuration", 
			"name", capturedSubscription.Definition.Name,
			"channel", capturedSubscription.Definition.Spec.Channel,
			"source", capturedSubscription.Definition.Spec.Source)
		// Update subscription to trigger reinstallation (no-op update)
		_, err = capturedSubscription.Update()
		Expect(err).ToNot(HaveOccurred(), "Failed to update captured subscription for reinstallation")
	} else {
		GinkgoLogr.Info("Captured Subscription was nil, attempting manual operator restoration")
		// Try manual restoration if subscription was not captured
		err = manuallyRestoreOperatorWithCapturedConfig(getAPIClient(), sriovOpNs, nil)
		if err != nil {
			GinkgoLogr.Info("Manual restoration attempt failed", "error", err)
			// Don't skip - fail explicitly so subsequent tests aren't silently affected
			Fail("CRITICAL: Failed to restore SR-IOV operator - subsequent tests will fail. Manual intervention required.")
		}
	}

		By("Phase 2.2: Waiting for new CSV and operator pods")
		err = waitForOperatorReinstall(getAPIClient(), sriovOpNs, 10*time.Minute)
		if err != nil {
			GinkgoLogr.Info("Operator reinstall failed, retrying with extended timeout", "error", err)
			// Extended retry with longer timeout
			err = waitForOperatorReinstall(getAPIClient(), sriovOpNs, 10*time.Minute)
		}
		Expect(err).ToNot(HaveOccurred(), "CRITICAL: Operator must reinstall for subsequent tests")

	By("Phase 2.3: Explicitly verifying operator pods are running")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	podList := &corev1.PodList{}
	err = getAPIClient().Client.List(ctx, podList, &client.ListOptions{Namespace: sriovOpNs})
	Expect(err).ToNot(HaveOccurred(), "Failed to list operator pods")
	Expect(len(podList.Items)).To(BeNumerically(">", 0), "CRITICAL: Operator pods must be running after restoration")
	GinkgoLogr.Info("Operator pods verified running", "count", len(podList.Items))

		By("Phase 2.4: Verifying CSV reaches Succeeded phase")
		csv, err := getOperatorCSV(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "CSV should be available after reinstall")
		Expect(csv.Definition.Status.Phase).To(Equal("Succeeded"), "CSV should be in Succeeded phase")

	By("Phase 2.5: Final verification that operator is fully operational")
	chkSriovOperatorStatus(sriovOpNs)

		GinkgoLogr.Info("Phase 2 completed: Operator successfully reinstalled and verified operational")

		// ==================== PHASE 3: CONTROL PLANE VALIDATION ====================
		By("PHASE 3: Validating control plane recovery")

		By("Phase 3.1: Validating node states reconcile")
		err = validateNodeStatesReconciled(getAPIClient(), sriovOpNs, 20*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Node states should reconcile after reinstall")

		By("Phase 3.2: Validating existing policies are recognized")
		afterState, err := captureSriovState(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to capture state after reinstall")

		differences := compareSriovState(beforeState, afterState)
		if len(differences) > 0 {
			GinkgoLogr.Info("State differences detected", "differences", differences)
		}
		Expect(len(afterState.Policies)).To(BeNumerically(">=", len(beforeState.Policies)),
			"Policies should be preserved after reinstall")
		Expect(len(afterState.Networks)).To(BeNumerically(">=", len(beforeState.Networks)),
			"Networks should be preserved after reinstall")

		By("Phase 3.3: Validating operator control plane health")
		err = validateOperatorControlPlane(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Control plane should be healthy after reinstall")

		GinkgoLogr.Info("Phase 3 completed: Control plane validated successfully")

		// ==================== PHASE 4: DATA PLANE VALIDATION ====================
		By("PHASE 4: Validating data plane functionality")

		By("Phase 4.1: Verifying existing workload pods still function")
		// Re-pull pods to get latest status
		clientPod, err = pod.Pull(getAPIClient(), clientPod.Definition.Name, clientPod.Definition.Namespace)
		Expect(err).ToNot(HaveOccurred(), "Client pod should still exist after reinstall")
		Expect(clientPod.Definition.Status.Phase).To(Equal(corev1.PodRunning), "Client pod should still be running after reinstall")

		serverPod, err = pod.Pull(getAPIClient(), serverPod.Definition.Name, serverPod.Definition.Namespace)
		Expect(err).ToNot(HaveOccurred(), "Server pod should still exist after reinstall")
		Expect(serverPod.Definition.Status.Phase).To(Equal(corev1.PodRunning), "Server pod should still be running after reinstall")

		By("Phase 4.2: Testing traffic between existing pods")
		err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.20.11")
		Expect(err).ToNot(HaveOccurred(), "Existing pods should communicate after reinstall")

		By("Phase 4.3: Creating new test pods and validating connectivity")
		newClientPod := createTestPod("client-new", testNamespace, testNetworkName, "192.168.20.20/24", "20:04:0f:f1:88:11")
		newServerPod := createTestPod("server-new", testNamespace, testNetworkName, "192.168.20.21/24", "20:04:0f:f1:88:12")

		defer func() {
			newClientPod.DeleteAndWait(60 * time.Second)
			newServerPod.DeleteAndWait(60 * time.Second)
		}()

		err = newClientPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "New client pod should be ready")

		err = newServerPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "New server pod should be ready")

		By("Phase 4.4: Confirming new workloads can use SR-IOV networks")
		err = validateWorkloadConnectivity(newClientPod, newServerPod, "192.168.20.21")
		Expect(err).ToNot(HaveOccurred(), "New pods should communicate successfully")

		GinkgoLogr.Info("Phase 4 completed: Data plane validated successfully")

		By("✅ REINSTALLATION TEST COMPLETED SUCCESSFULLY")
		By("All phases passed: operator removed, reinstalled, and functionality verified")
	})
})
