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
		GinkgoLogr.Info("SR-IOV operator status verified", "namespace", sriovOpNs)

		By("Discovering worker nodes")
		var err error
		workerNodes, err = nodes.List(getAPIClient(),
			metav1.ListOptions{LabelSelector: labels.Set(NetConfig.WorkerLabelMap).String()})
		Expect(err).ToNot(HaveOccurred(), "Failed to discover worker nodes")
		GinkgoLogr.Info("Worker nodes discovered", "count", len(workerNodes))
	})

	It("test_sriov_components_cleanup_on_removal - Validate complete cleanup when operator removed [Disruptive] [Serial]", func() {
		By("COMPONENT CLEANUP ON REMOVAL - Testing resource cleanup when operator is removed")
		GinkgoLogr.Info("Starting component cleanup validation test", "namespace", sriovOpNs)

		var testDeviceConfig deviceConfig
		var testNamespace string
		var testNetworkName string
		var testPolicyName string
		var clientPod, serverPod *pod.Builder
		executed := false

		// ==================== PHASE 1: SETUP AND BASELINE ====================
		By("PHASE 1: Setting up test workload and capturing baseline")
		GinkgoLogr.Info("Phase 1: Initializing test environment for component cleanup", "namespace", sriovOpNs)

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
				"source", capturedSubscription.Definition.Spec.CatalogSource)
		}

		// IMPORTANT: For private registry environments, capture IDMS to ensure operator images can be pulled
		// without this, operator pods would fail to start with ImagePullBackOff after restoration
		By("Capturing ImageDigestMirrorSet configuration for private registry support")
		capturedIDMS, err := captureImageDigestMirrorSets(getAPIClient())
		if err != nil {
			GinkgoLogr.Info("No ImageDigestMirrorSet found, test likely uses public registries", "error", err)
		} else {
			GinkgoLogr.Info("ImageDigestMirrorSet configuration captured successfully", "count", len(capturedIDMS))
			for i, idms := range capturedIDMS {
				GinkgoLogr.Info("IDMS captured", "index", i, "name", idms.Name, "mirrors_count", len(idms.Spec.ImageDigestMirrors))
			}
		}

		// Use timestamp suffix to avoid namespace collision from previous test runs (fixes race condition in namespace termination)
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		testNamespace = "e2e-lifecycle-cleanup-" + testDeviceConfig.Name + "-" + timestamp
		testNetworkName = "lifecycle-cleanup-net-" + testDeviceConfig.Name
		testPolicyName = testDeviceConfig.Name

		// Create namespace for test
		nsBuilder := namespace.NewBuilder(getAPIClient(), testNamespace)
		for key, value := range params.PrivilegedNSLabels {
			nsBuilder.WithLabel(key, value)
		}
		_, err = nsBuilder.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

		// Phase 2 Enhancement: Ensure namespace is ready before proceeding
		By("Waiting for namespace to reach Active phase (Phase 2 Enhancement - Namespace Initialization)")
		err = ensureNamespaceReady(getAPIClient(), testNamespace, 30*time.Second)
		Expect(err).ToNot(HaveOccurred(), "Namespace should reach Active phase")
		GinkgoLogr.Info("Namespace is ready and Active", "namespace", testNamespace)

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

		err = clientPod.WaitUntilReady(15 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client pod should be ready")

		err = serverPod.WaitUntilReady(15 * time.Minute)
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
		// Increased timeout to 15 minutes to account for DaemonSet pod termination
		// and potential API rate limiting during cleanup
		err = validateOperatorPodsRemoved(getAPIClient(), sriovOpNs, 15*time.Minute)
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

		// CRITICAL: Restore IDMS BEFORE operator reinstallation
		// For private registry environments, IDMS must be in place for operator images to be pulled correctly
		By("Phase 4.0: Restoring ImageDigestMirrorSet configuration for private registry support")
		if capturedIDMS != nil && len(capturedIDMS) > 0 {
			GinkgoLogr.Info("Restoring ImageDigestMirrorSet configuration", "count", len(capturedIDMS))
			err = restoreImageDigestMirrorSets(getAPIClient(), capturedIDMS)
			Expect(err).ToNot(HaveOccurred(), "Failed to restore ImageDigestMirrorSet configuration")
			GinkgoLogr.Info("ImageDigestMirrorSet configuration restored successfully")
		} else {
			GinkgoLogr.Info("No IDMS to restore, test uses public registries")
		}

		By("Phase 4.1: Triggering operator reinstallation using captured Subscription configuration")
		operatorRestored := false

		// Use the subscription we captured BEFORE deletion to ensure exact restoration
		if capturedSubscription != nil {
			GinkgoLogr.Info("Restoring operator with captured Subscription configuration",
				"name", capturedSubscription.Definition.Name,
				"channel", capturedSubscription.Definition.Spec.Channel,
				"source", capturedSubscription.Definition.Spec.CatalogSource)
			// Update subscription to trigger reinstallation
			_, err = capturedSubscription.Update()
			Expect(err).ToNot(HaveOccurred(), "Failed to update captured subscription for reinstallation")
		} else {
			GinkgoLogr.Info("Captured Subscription was nil, attempting manual operator restoration")
			// Try manual restoration if subscription was not captured
			err = manuallyRestoreOperatorWithCapturedConfig(getAPIClient(), sriovOpNs, nil)
			if err != nil {
				GinkgoLogr.Info("Manual restoration attempt failed", "error", err)
				// Don't skip - instead, fail explicitly so subsequent tests aren't silently affected
				Fail("CRITICAL: Failed to restore SR-IOV operator - subsequent tests will fail. Manual intervention required.")
			}
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
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		podList := &corev1.PodList{}
		err = getAPIClient().Client.List(ctx, podList, &client.ListOptions{Namespace: sriovOpNs})
		Expect(err).ToNot(HaveOccurred(), "Failed to list operator pods")
		Expect(len(podList.Items)).To(BeNumerically(">", 0), "CRITICAL: Operator pods must be running after restoration")
		GinkgoLogr.Info("Operator pods verified running", "count", len(podList.Items))

		By("Phase 4.4: Validating control plane recovery")
		err = validateOperatorControlPlane(getAPIClient(), sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Control plane should be healthy after reinstall")

		By("Phase 4.5: Waiting for node states to reconcile")
		err = validateNodeStatesReconciled(getAPIClient(), sriovOpNs, 20*time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Node states should reconcile after reinstall")

		By("Phase 4.6: Final verification that operator is fully operational")
		chkSriovOperatorStatus(sriovOpNs)

		if !operatorRestored {
			Fail("CRITICAL: Operator restoration incomplete - subsequent tests will fail")
		}

		GinkgoLogr.Info("Phase 4 completed: Operator successfully reinstalled and verified operational")

		By("✅ COMPONENT CLEANUP TEST COMPLETED SUCCESSFULLY")
		By("All phases passed: components removed, CRDs preserved, workloads survived, operator reinstalled and verified")
	})

	It("test_sriov_resource_deployment_dependency - Validate resources cannot deploy without operator [Disruptive] [Serial]", func() {
		By("RESOURCE DEPLOYMENT DEPENDENCY - Testing resource deployment constraints without operator")
		GinkgoLogr.Info("Starting resource deployment dependency test", "namespace", sriovOpNs)

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
		GinkgoLogr.Info("Phase 1: Initializing test environment for dependency validation", "namespace", sriovOpNs)

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

		// Use timestamp suffix to avoid namespace collision from previous test runs (fixes race condition in namespace termination)
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		testNamespace = "e2e-lifecycle-depend-" + testDeviceConfig.Name + "-" + timestamp
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

		// Phase 2 Enhancement: Ensure namespace is ready before proceeding
		By("Waiting for namespace to reach Active phase (Phase 2 Enhancement - Namespace Initialization)")
		err = ensureNamespaceReady(getAPIClient(), testNamespace, 30*time.Second)
		Expect(err).ToNot(HaveOccurred(), "Namespace should reach Active phase")
		GinkgoLogr.Info("Namespace is ready and Active", "namespace", testNamespace)

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

		err = clientPod.WaitUntilReady(15 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client pod should be ready with operator running")

		err = serverPod.WaitUntilReady(15 * time.Minute)
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
		// Increased timeout to 15 minutes to account for DaemonSet pod termination
		// and potential API rate limiting during cleanup
		err = validateOperatorPodsRemoved(getAPIClient(), sriovOpNs, 15*time.Minute)
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

		err = newClientPod.WaitUntilReady(15 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "New client pod should be ready after reconciliation")

		err = newServerPod.WaitUntilReady(15 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "New server pod should be ready after reconciliation")

		By("Phase 5.2: Validating connectivity for new workloads")
		err = validateWorkloadConnectivity(newClientPod, newServerPod, "192.168.40.21")
		Expect(err).ToNot(HaveOccurred(), "New workloads should communicate successfully")

		GinkgoLogr.Info("Phase 5 completed: Full functionality validated")

		By("✅ RESOURCE DEPENDENCY TEST COMPLETED SUCCESSFULLY")
		By("All phases passed: resources don't reconcile without operator, automatic reconciliation after operator returns")
	})
})
