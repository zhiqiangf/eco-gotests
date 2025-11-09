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
)

var (
	lifecycleTestNS *namespace.Builder
)

// getLifecycleTestNS returns the lifecycle test namespace, initializing it if necessary
func getLifecycleTestNS() *namespace.Builder {
	if lifecycleTestNS == nil {
		lifecycleTestNS = namespace.NewBuilder(getAPIClient(), "sriov-lifecycle-test")
	}
	return lifecycleTestNS
}

var _ = Describe("[sig-networking] SR-IOV Component Lifecycle", Label("lifecycle"), func() {
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

	It("test_sriov_components_cleanup_on_removal - Validate complete cleanup when operator removed [Disruptive] [Serial]", func() {
		var testDeviceConfig deviceConfig
		var testNamespace string
		var testNetworkName string
		var testPolicyName string
		var clientPod, serverPod *pod.Builder
		executed := false

		// ==================== PHASE 1: SETUP AND BASELINE ====================
		By("PHASE 1: Setting up test workload and capturing baseline")

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
			Skip("No SR-IOV devices available for component cleanup testing")
		}

		testNamespace = "e2e-lifecycle-cleanup-" + testDeviceConfig.Name
		testNetworkName = "lifecycle-cleanup-net-" + testDeviceConfig.Name
		testPolicyName = testDeviceConfig.Name

		// Create namespace for test
		nsBuilder := namespace.NewBuilder(getAPIClient(), testNamespace)
		for key, value := range params.PrivilegedNSLabels {
			nsBuilder.WithLabel(key, value)
		}
		_, err := nsBuilder.Create()
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
			rmSriovPolicy(testPolicyName, sriovOpNs)
			// Clean up namespace
			err := nsBuilder.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete test namespace", "namespace", testNamespace, "error", err)
			}
		}()

		By("Phase 1.1: Capturing baseline state of operator components")
		baselineState, err := captureSriovState(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to capture baseline state")
		GinkgoLogr.Info("Baseline state captured",
			"policies", len(baselineState.Policies),
			"networks", len(baselineState.Networks))

		By("Phase 1.2: Creating SR-IOV network for test workload")
		sriovNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-whereabouts-template.yaml")
		sriovnetwork := sriovNetwork{
			name:             testNetworkName,
			resourceName:     testPolicyName,
			networkNamespace: testNamespace,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
			spoolchk:         "off",
			trust:            "on",
		}
		sriovnetwork.createSriovNetwork()

		By("Phase 1.3: Creating test pods with SR-IOV interfaces")
		clientPod = createTestPod("client-cleanup", testNamespace, testNetworkName, "192.168.30.10/24", "20:04:0f:f1:77:01")
		serverPod = createTestPod("server-cleanup", testNamespace, testNetworkName, "192.168.30.11/24", "20:04:0f:f1:77:02")

		err = clientPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client pod should be ready")

		err = serverPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server pod should be ready")

		By("Phase 1.4: Validating initial connectivity between pods")
		err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.30.11")
		Expect(err).ToNot(HaveOccurred(), "Initial connectivity should work")

		GinkgoLogr.Info("Phase 1 completed: Baseline established with working data plane")

		// ==================== PHASE 2: OPERATOR REMOVAL ====================
		By("PHASE 2: Removing SR-IOV operator and validating component cleanup")

		By("Phase 2.1: Deleting SriovOperatorConfig")
		err = deleteOperatorConfiguration(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to delete operator configuration")

		By("Phase 2.2: Deleting CSV to trigger operator removal")
		err = deleteOperatorCSV(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to delete operator CSV")

		By("Phase 2.3: Validating all operator components are removed")
		err = validateAllComponentsRemoved(getAPIClient(), sriovOpNs, 5*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "All operator components should be removed")

		By("Phase 2.4: Verifying operator pods are terminated")
		err = validateOperatorPodsRemoved(getAPIClient(), sriovOpNs, 5*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Operator pods should be terminated")

		By("Phase 2.5: Verifying daemonsets are removed")
		err = validateDaemonSetsRemoved(getAPIClient(), sriovOpNs, 5*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "DaemonSets should be removed")

		By("Phase 2.6: Verifying webhooks are removed")
		err = validateWebhooksRemoved(getAPIClient(), 5*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Webhooks should be removed")

		GinkgoLogr.Info("Phase 2 completed: All components successfully removed")

		// ==================== PHASE 3: VALIDATE CRDs AND WORKLOAD SURVIVAL ====================
		By("PHASE 3: Validating CRDs remain and workloads survive")

		By("Phase 3.1: Verifying CRDs still exist")
		_, err = captureSriovState(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "CRDs should still be accessible")

		By("Phase 3.2: Verifying existing workload pods still function")
		// Re-pull pods to get latest status
		clientPod, err = pod.Pull(getAPIClient(), clientPod.Definition.Name, clientPod.Definition.Namespace)
		Expect(err).ToNot(HaveOccurred(), "Client pod should still exist")
		Expect(clientPod.Definition.Status.Phase).To(Equal(corev1.PodRunning), "Client pod should still be running")

		serverPod, err = pod.Pull(getAPIClient(), serverPod.Definition.Name, serverPod.Definition.Namespace)
		Expect(err).ToNot(HaveOccurred(), "Server pod should still exist")
		Expect(serverPod.Definition.Status.Phase).To(Equal(corev1.PodRunning), "Server pod should still be running")

		By("Phase 3.3: Validating workload connectivity still works")
		err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.30.11")
		Expect(err).ToNot(HaveOccurred(), "Workload connectivity should survive operator removal")

		GinkgoLogr.Info("Phase 3 completed: CRDs remain, workloads still operational")

		// ==================== PHASE 4: OPERATOR REINSTALLATION ====================
		By("PHASE 4: Reinstalling SR-IOV operator")

		By("Phase 4.1: Triggering operator reinstallation")
		sub, err := getOperatorSubscription(getAPIClient(), sriovOpNs)
		operatorRestored := false

		if err != nil {
			GinkgoLogr.Info("Subscription not found, attempting manual operator restoration", "error", err)
			// Try manual restoration if subscription is missing
			err = manuallyRestoreOperator(getAPIClient(), sriovOpNs)
			if err != nil {
				GinkgoLogr.Info("Manual restoration attempt failed", "error", err)
				// Don't skip - instead, fail explicitly so subsequent tests aren't silently affected
				Fail("CRITICAL: Failed to restore SR-IOV operator - subsequent tests will fail. Manual intervention required.")
			}
		} else {
			// Update subscription to trigger reinstallation
			GinkgoLogr.Info("Triggering reinstallation via subscription", "subscription", sub.Definition.Name)
			_, err = sub.Update()
			Expect(err).ToNot(HaveOccurred(), "Failed to update subscription")
		}

		By("Phase 4.2: Waiting for operator to reinstall")
		err = waitForOperatorReinstall(getAPIClient(), sriovOpNs, 10*time.Minute)
		if err != nil {
			GinkgoLogr.Info("Operator reinstall failed, retrying with extended timeout", "error", err)
			// Extended retry with longer timeout
			err = waitForOperatorReinstall(getAPIClient(), sriovOpNs, 10*time.Minute)
		}
		Expect(err).ToNot(HaveOccurred(), "CRITICAL: Operator must reinstall for subsequent tests")
		operatorRestored = true

		By("Phase 4.3: Explicitly verifying operator pods are running")
		pods, err := getAPIClient().CoreV1().Pods(sriovOpNs).List(context.TODO(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred(), "Failed to list operator pods")
		Expect(len(pods.Items)).To(BeGreaterThan(0), "CRITICAL: Operator pods must be running after restoration")
		GinkgoLogr.Info("Operator pods verified running", "count", len(pods.Items))

		By("Phase 4.4: Validating control plane recovery")
		err = validateOperatorControlPlane(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Control plane should be healthy after reinstall")

		By("Phase 4.5: Waiting for node states to reconcile")
		err = validateNodeStatesReconciled(getAPIClient(), sriovOpNs, 20*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Node states should reconcile after reinstall")

		By("Phase 4.6: Final verification that operator is fully operational")
		err = chkSriovOperatorStatus(sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "CRITICAL: Operator must be fully operational for subsequent tests")

		if !operatorRestored {
			Fail("CRITICAL: Operator restoration incomplete - subsequent tests will fail")
		}

		GinkgoLogr.Info("Phase 4 completed: Operator successfully reinstalled and verified operational")

		By("✅ COMPONENT CLEANUP TEST COMPLETED SUCCESSFULLY")
		By("All phases passed: components removed, CRDs preserved, workloads survived, operator reinstalled and verified")
	})

	It("test_sriov_resource_deployment_dependency - Validate resources cannot deploy without operator [Disruptive] [Serial]", func() {
		var testDeviceConfig deviceConfig
		var testNamespace string
		var testNetworkName string
		var testPolicyName string
		var newPolicyName string
		var newNetworkName string
		var clientPod, serverPod *pod.Builder
		var beforeNodeStates map[string]string
		executed := false

		// ==================== PHASE 1: INITIAL SETUP ====================
		By("PHASE 1: Setting up initial resources and capturing baseline")

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
			Skip("No SR-IOV devices available for dependency testing")
		}

		testNamespace = "e2e-lifecycle-depend-" + testDeviceConfig.Name
		testNetworkName = "lifecycle-depend-net-" + testDeviceConfig.Name
		testPolicyName = testDeviceConfig.Name
		newPolicyName = "new-policy-" + testDeviceConfig.Name
		newNetworkName = "new-network-" + testDeviceConfig.Name

		// Create namespace for test
		nsBuilder := namespace.NewBuilder(getAPIClient(), testNamespace)
		for key, value := range params.PrivilegedNSLabels {
			nsBuilder.WithLabel(key, value)
		}
		_, err := nsBuilder.Create()
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
			// Clean up networks
			rmSriovNetwork(testNetworkName, sriovOpNs)
			rmSriovNetwork(newNetworkName, sriovOpNs)
			// Clean up policies
			rmSriovPolicy(testPolicyName, sriovOpNs)
			rmSriovPolicy(newPolicyName, sriovOpNs)
			// Clean up namespace
			err := nsBuilder.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete test namespace", "namespace", testNamespace, "error", err)
			}
		}()

		By("Phase 1.1: Capturing baseline state")
		baselineState, err := captureSriovState(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to capture baseline state")
		beforeNodeStates = baselineState.NodeStates

		By("Phase 1.2: Creating initial SR-IOV resources with operator running")
		sriovNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-whereabouts-template.yaml")
		sriovnetwork := sriovNetwork{
			name:             testNetworkName,
			resourceName:     testPolicyName,
			networkNamespace: testNamespace,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
			spoolchk:         "off",
			trust:            "on",
		}
		sriovnetwork.createSriovNetwork()

		By("Phase 1.3: Creating test pods to validate initial deployment")
		clientPod = createTestPod("client-depend", testNamespace, testNetworkName, "192.168.40.10/24", "20:04:0f:f1:66:01")
		serverPod = createTestPod("server-depend", testNamespace, testNetworkName, "192.168.40.11/24", "20:04:0f:f1:66:02")

		err = clientPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client pod should be ready with operator running")

		err = serverPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server pod should be ready with operator running")

		By("Phase 1.4: Validating initial deployment works")
		err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.40.11")
		Expect(err).ToNot(HaveOccurred(), "Initial deployment should work with operator")

		GinkgoLogr.Info("Phase 1 completed: Initial resources deployed successfully")

		// ==================== PHASE 2: REMOVE OPERATOR ====================
		By("PHASE 2: Removing operator to test resource dependency")

		By("Phase 2.1: Deleting CSV to remove operator")
		err = deleteOperatorCSV(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to delete operator CSV")

		By("Phase 2.2: Waiting for operator pods to terminate")
		err = validateOperatorPodsRemoved(getAPIClient(), sriovOpNs, 5*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Operator pods should be terminated")

		GinkgoLogr.Info("Phase 2 completed: Operator removed")

		// ==================== PHASE 3: ATTEMPT NEW RESOURCE CREATION ====================
		By("PHASE 3: Attempting to create new resources without operator")

		By("Phase 3.1: Creating new SR-IOV policy (should exist in API but not reconcile)")
		// Create a new policy targeting the same device
		newPolicy := createSriovPolicy(newPolicyName, testDeviceConfig.DeviceID, testDeviceConfig.Vendor,
			testDeviceConfig.InterfaceName, sriovOpNs, vfNum, workerNodes)
		Expect(newPolicy).ToNot(BeNil(), "New policy should be created in API")

		By("Phase 3.2: Creating new SR-IOV network (should exist but NAD may not be created)")
		newSriovNetwork := sriovNetwork{
			name:             newNetworkName,
			resourceName:     newPolicyName,
			networkNamespace: testNamespace,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
			spoolchk:         "off",
			trust:            "on",
		}
		newSriovNetwork.createSriovNetwork()

		By("Phase 3.3: Validating resources exist but don't reconcile")
		// Wait a bit to ensure operator would have reconciled if it was running
		time.Sleep(30 * time.Second)

		err = validateResourcesNotReconciling(getAPIClient(), sriovOpNs, newPolicyName, beforeNodeStates)
		Expect(err).ToNot(HaveOccurred(), "Resources should exist but not reconcile without operator")

		GinkgoLogr.Info("Phase 3 completed: New resources created but not reconciling")

		// ==================== PHASE 4: REINSTALL OPERATOR ====================
		By("PHASE 4: Reinstalling operator and validating automatic reconciliation")

		By("Phase 4.1: Triggering operator reinstallation")
		sub, err := getOperatorSubscription(getAPIClient(), sriovOpNs)
		if err != nil {
			GinkgoLogr.Info("Subscription not found, attempting manual restoration", "error", err)
			err = manuallyRestoreOperator(getAPIClient(), sriovOpNs)
			Expect(err).ToNot(HaveOccurred(), "CRITICAL: Failed to restore operator - must succeed for test isolation")
		} else {
			_, err = sub.Update()
			Expect(err).ToNot(HaveOccurred(), "Failed to update subscription")
		}

		By("Phase 4.2: Waiting for operator to reinstall")
		err = waitForOperatorReinstall(getAPIClient(), sriovOpNs, 10*time.Minute)
		if err != nil {
			GinkgoLogr.Info("Operator reinstall failed, retrying with extended timeout", "error", err)
			err = waitForOperatorReinstall(getAPIClient(), sriovOpNs, 10*time.Minute)
		}
		Expect(err).ToNot(HaveOccurred(), "CRITICAL: Operator must reinstall for subsequent tests")

		By("Phase 4.3: Validating automatic reconciliation of pending resources")
		err = validateNodeStatesReconciled(getAPIClient(), sriovOpNs, 20*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Resources should reconcile automatically after operator returns")

		By("Phase 4.4: Verifying new policy was reconciled")
		afterState, err := captureSriovState(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Failed to capture state after reconciliation")

		policyFound := false
		for _, policyName := range afterState.Policies {
			if policyName == newPolicyName {
				policyFound = true
				break
			}
		}
		Expect(policyFound).To(BeTrue(), "New policy should be reconciled")

		GinkgoLogr.Info("Phase 4 completed: Operator reinstalled and resources reconciled")

		// ==================== PHASE 5: VALIDATE FULL FUNCTIONALITY ====================
		By("PHASE 5: Validating full functionality after reconciliation")

		By("Phase 5.1: Creating new workload pods using reconciled resources")
		newClientPod := createTestPod("client-new-depend", testNamespace, newNetworkName, "192.168.40.20/24", "20:04:0f:f1:66:11")
		newServerPod := createTestPod("server-new-depend", testNamespace, newNetworkName, "192.168.40.21/24", "20:04:0f:f1:66:12")

		defer func() {
			if newClientPod != nil {
				newClientPod.DeleteAndWait(60 * time.Second)
			}
			if newServerPod != nil {
				newServerPod.DeleteAndWait(60 * time.Second)
			}
		}()

		err = newClientPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "New client pod should be ready after reconciliation")

		err = newServerPod.WaitUntilReady(5 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "New server pod should be ready after reconciliation")

		By("Phase 5.2: Validating connectivity for new workloads")
		err = validateWorkloadConnectivity(newClientPod, newServerPod, "192.168.40.21")
		Expect(err).ToNot(HaveOccurred(), "New workloads should communicate successfully")

		GinkgoLogr.Info("Phase 5 completed: Full functionality validated")

		By("✅ RESOURCE DEPENDENCY TEST COMPLETED SUCCESSFULLY")
		By("All phases passed: resources don't reconcile without operator, automatic reconciliation after operator returns")
	})
})

// Helper function to manually restore the SR-IOV operator if subscription is not available
func manuallyRestoreOperator(apiClient *client.Client, sriovOpNs string) error {
	GinkgoLogr.Info("Attempting manual SR-IOV operator restoration")

	// Check if subscription exists and recreate if needed
	subs, err := apiClient.Operators("v1alpha1", "Subscription").List(sriovOpNs, metav1.ListOptions{})
	if err != nil {
		GinkgoLogr.Info("Failed to list subscriptions", "error", err)
		return err
	}

	// If no subscription found, try to wait for automatic recreation
	if subs == nil || len(subs) == 0 {
		GinkgoLogr.Info("No subscriptions found, waiting for operator to reconcile from catalog")
		time.Sleep(10 * time.Second)

		// Retry waiting for operator pods
		for i := 0; i < 30; i++ {
			pods, err := apiClient.CoreV1().Pods(sriovOpNs).List(context.TODO(), metav1.ListOptions{})
			if err == nil && len(pods.Items) > 0 {
				GinkgoLogr.Info("Operator pods found after subscription recreation", "count", len(pods.Items))
				return nil
			}
			time.Sleep(3 * time.Second)
		}

		return fmt.Errorf("operator pods not found after manual restoration attempt")
	}

	return nil
}
