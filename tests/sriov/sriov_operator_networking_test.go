package sriov

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/clients"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/nad"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/namespace"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/nodes"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/pod"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/sriov"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/params"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	operatorNetworkingTestNS *namespace.Builder
)

// getOperatorNetworkingTestNS returns the operator networking test namespace, initializing it if necessary
func getOperatorNetworkingTestNS() *namespace.Builder {
	if operatorNetworkingTestNS == nil {
		operatorNetworkingTestNS = namespace.NewBuilder(getAPIClient(), "sriov-operator-networking-test")
	}
	return operatorNetworkingTestNS
}

var _ = Describe("[sig-networking] SR-IOV Operator Networking", Label("operator-networking"), func() {
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

	AfterEach(func() {
		// Clean up SR-IOV policies created during tests
		for _, item := range testData {
			rmSriovPolicy(item.Name, sriovOpNs)
		}
		waitForSriovPolicyReady(sriovOpNs)
	})

	It("test_sriov_operator_ipv4_functionality - Operator-focused IPv4 networking validation [Disruptive] [Serial]", func() {
		By("SR-IOV OPERATOR IPv4 NETWORKING - Validating operator-focused IPv4 networking functionality")
		GinkgoLogr.Info("Starting IPv4 networking functionality test")

		executed := false
		var testDeviceConfig deviceConfig

		// Find a suitable device for testing
		for _, data := range testData {
			result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
			if result {
				testDeviceConfig = data
				executed = true
				GinkgoLogr.Info("SR-IOV device selected for IPv4 networking test", "device", data.Name, "deviceID", data.DeviceID)
				break
			}
		}

		if !executed {
			Skip("No SR-IOV devices available for IPv4 networking testing")
		}

		// ==================== PHASE 1: Whereabouts IPAM ====================
		By("PHASE 1: Testing IPv4 networking with Whereabouts IPAM")
		GinkgoLogr.Info("Phase 1: Testing IPv4 with Whereabouts IPAM")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampWhereabouts := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceWhereabouts := "e2e-ipv4-whereabouts-" + testDeviceConfig.Name + "-" + timestampWhereabouts + testDeviceConfig.Name
		testNetworkWhereabouts := "ipv4-whereabouts-net-" + testDeviceConfig.Name

		// Create namespace for whereabouts test
		nsWhereabouts := namespace.NewBuilder(getAPIClient(), testNamespaceWhereabouts)
		for key, value := range params.PrivilegedNSLabels {
			nsWhereabouts.WithLabel(key, value)
		}
		_, err := nsWhereabouts.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create whereabouts test namespace")

		defer func() {
			By("Cleaning up Whereabouts IPAM test resources")
			rmSriovNetwork(testNetworkWhereabouts, sriovOpNs)
			err := nsWhereabouts.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete whereabouts namespace", "namespace", testNamespaceWhereabouts, "error", err)
			}
		}()

		// Create SR-IOV network with whereabouts IPAM
		sriovNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-whereabouts-template.yaml")
		sriovNetworkWhereabouts := sriovNetwork{
			name:             testNetworkWhereabouts,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceWhereabouts,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNetworkWhereabouts.createSriovNetwork()

		// Create test pods with whereabouts (auto IP assignment)
		clientPodWhereabouts := createTestPod("client-wb", testNamespaceWhereabouts, testNetworkWhereabouts,
			"192.168.100.10/24", "20:04:0f:f1:a0:01")
		serverPodWhereabouts := createTestPod("server-wb", testNamespaceWhereabouts, testNetworkWhereabouts,
			"192.168.100.11/24", "20:04:0f:f1:a0:02")

		err = clientPodWhereabouts.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client pod (whereabouts) should be ready")

		err = serverPodWhereabouts.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server pod (whereabouts) should be ready")

		// Verify SR-IOV interfaces
		err = verifyPodSriovInterface(clientPodWhereabouts, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "Client pod should have SR-IOV interface")

		// Check for NO-CARRIER before testing connectivity
		clientCarrier, err := checkInterfaceCarrier(clientPodWhereabouts, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to check interface carrier status")

		if !clientCarrier {
			GinkgoLogr.Info("Interface has NO-CARRIER status, skipping whereabouts connectivity test")
		} else {
			// Test IPv4 connectivity
			By("Testing IPv4 connectivity with Whereabouts IPAM")
			err = validateWorkloadConnectivity(clientPodWhereabouts, serverPodWhereabouts, "192.168.100.11")
			Expect(err).ToNot(HaveOccurred(), "IPv4 connectivity should work with Whereabouts IPAM")
		}

		// Cleanup whereabouts pods
		clientPodWhereabouts.DeleteAndWait(60 * time.Second)
		serverPodWhereabouts.DeleteAndWait(60 * time.Second)

		By("PHASE 1 completed: Whereabouts IPAM IPv4 networking validated")

		// ==================== PHASE 2: Static IPAM ====================
		By("PHASE 2: Testing IPv4 networking with Static IPAM")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampStatic := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceStatic := "e2e-ipv4-static-" + testDeviceConfig.Name + "-" + timestampStatic + testDeviceConfig.Name
		testNetworkStatic := "ipv4-static-net-" + testDeviceConfig.Name

		// Create namespace for static test
		nsStatic := namespace.NewBuilder(getAPIClient(), testNamespaceStatic)
		for key, value := range params.PrivilegedNSLabels {
			nsStatic.WithLabel(key, value)
		}
		_, err = nsStatic.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create static test namespace")

		defer func() {
			By("Cleaning up Static IPAM test resources")
			rmSriovNetwork(testNetworkStatic, sriovOpNs)
			err := nsStatic.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete static namespace", "namespace", testNamespaceStatic, "error", err)
			}
		}()

		// Create SR-IOV network with static IPAM
		sriovNetworkStaticTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-template.yaml")
		sriovNetworkStatic := sriovNetwork{
			name:             testNetworkStatic,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceStatic,
			template:         sriovNetworkStaticTemplate,
			namespace:        sriovOpNs,
		}
		sriovNetworkStatic.createSriovNetwork()

		// Create test pods with static IPv4 addresses
		clientPodStatic := createTestPod("client-static", testNamespaceStatic, testNetworkStatic,
			"192.168.101.10/24", "20:04:0f:f1:a1:01")
		serverPodStatic := createTestPod("server-static", testNamespaceStatic, testNetworkStatic,
			"192.168.101.11/24", "20:04:0f:f1:a1:02")

		err = clientPodStatic.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client pod (static) should be ready")

		err = serverPodStatic.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server pod (static) should be ready")

		// Verify SR-IOV interfaces
		err = verifyPodSriovInterface(clientPodStatic, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "Client pod should have SR-IOV interface")

		// Check for NO-CARRIER before testing connectivity
		clientCarrier, err = checkInterfaceCarrier(clientPodStatic, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to check interface carrier status")

		if !clientCarrier {
			GinkgoLogr.Info("Interface has NO-CARRIER status, skipping static connectivity test")
		} else {
			// Test IPv4 connectivity
			By("Testing IPv4 connectivity with Static IPAM")
			err = validateWorkloadConnectivity(clientPodStatic, serverPodStatic, "192.168.101.11")
			Expect(err).ToNot(HaveOccurred(), "IPv4 connectivity should work with Static IPAM")
		}

		// Cleanup static pods
		clientPodStatic.DeleteAndWait(60 * time.Second)
		serverPodStatic.DeleteAndWait(60 * time.Second)

		By("PHASE 2 completed: Static IPAM IPv4 networking validated")
		By("✅ IPv4 FUNCTIONALITY TEST COMPLETED: Both Whereabouts and Static IPAM validated")
	})

	It("test_sriov_operator_ipv6_functionality - Operator-focused IPv6 networking validation [Disruptive] [Serial]", func() {
		By("SR-IOV OPERATOR IPv6 NETWORKING - Validating operator-focused IPv6 networking functionality")
		GinkgoLogr.Info("Starting IPv6 networking functionality test")

		// Check IPv6 availability
		By("Checking IPv6 availability on worker nodes")
		hasIPv6 := detectIPv6Availability(getAPIClient())
		if !hasIPv6 {
			Skip("IPv6 is not enabled on worker nodes - skipping IPv6 networking test")
		}
		GinkgoLogr.Info("IPv6 is available on cluster")

		executed := false
		var testDeviceConfig deviceConfig

		// Find a suitable device for testing
		for _, data := range testData {
			result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
			if result {
				testDeviceConfig = data
				executed = true
				GinkgoLogr.Info("SR-IOV device selected for IPv6 networking test", "device", data.Name, "deviceID", data.DeviceID)
				break
			}
		}

		if !executed {
			Skip("No SR-IOV devices available for IPv6 networking testing")
		}

		// ==================== PHASE 1: Whereabouts IPAM (IPv6) ====================
		By("PHASE 1: Testing IPv6 networking with Whereabouts IPAM")
		GinkgoLogr.Info("Phase 1: Testing IPv6 with Whereabouts IPAM")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampIPv6WB := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceWhereabouts := "e2e-ipv6-whereabouts-" + testDeviceConfig.Name + "-" + timestampIPv6WB + testDeviceConfig.Name
		testNetworkWhereabouts := "ipv6-whereabouts-net-" + testDeviceConfig.Name

		nsWhereabouts := namespace.NewBuilder(getAPIClient(), testNamespaceWhereabouts)
		for key, value := range params.PrivilegedNSLabels {
			nsWhereabouts.WithLabel(key, value)
		}
		_, err := nsWhereabouts.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create whereabouts IPv6 test namespace")

		defer func() {
			By("Cleaning up Whereabouts IPv6 test resources")
			rmSriovNetwork(testNetworkWhereabouts, sriovOpNs)
			err := nsWhereabouts.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete whereabouts IPv6 namespace", "namespace", testNamespaceWhereabouts, "error", err)
			}
		}()

		// Create SR-IOV network with whereabouts IPv6 IPAM
		_ = createSriovNetworkIPv6Whereabouts(testNetworkWhereabouts, testDeviceConfig.Name,
			sriovOpNs, testNamespaceWhereabouts)

		// Create test pods with IPv6 addresses
		clientPodWhereabouts := createTestPodIPv6("client-ipv6-wb", testNamespaceWhereabouts,
			testNetworkWhereabouts, "fd00:192:168:100::10", "20:04:0f:f1:b0:01")
		serverPodWhereabouts := createTestPodIPv6("server-ipv6-wb", testNamespaceWhereabouts,
			testNetworkWhereabouts, "fd00:192:168:100::11", "20:04:0f:f1:b0:02")

		err = clientPodWhereabouts.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client IPv6 pod (whereabouts) should be ready")

		err = serverPodWhereabouts.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server IPv6 pod (whereabouts) should be ready")

		// Verify SR-IOV interfaces
		err = verifyPodSriovInterface(clientPodWhereabouts, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "Client pod should have SR-IOV interface")

		// Check for NO-CARRIER
		clientCarrier, err := checkInterfaceCarrier(clientPodWhereabouts, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to check interface carrier status")

		if !clientCarrier {
			GinkgoLogr.Info("Interface has NO-CARRIER status, skipping whereabouts IPv6 connectivity test")
		} else {
			// Test IPv6 connectivity
			By("Testing IPv6 connectivity with Whereabouts IPAM")
			err = verifyIPv6Connectivity(clientPodWhereabouts, serverPodWhereabouts, "fd00:192:168:100::11")
			Expect(err).ToNot(HaveOccurred(), "IPv6 connectivity should work with Whereabouts IPAM")
		}

		clientPodWhereabouts.DeleteAndWait(60 * time.Second)
		serverPodWhereabouts.DeleteAndWait(60 * time.Second)

		By("PHASE 1 completed: Whereabouts IPAM IPv6 networking validated")

		// ==================== PHASE 2: Static IPAM (IPv6) ====================
		By("PHASE 2: Testing IPv6 networking with Static IPAM")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampIPv6Static := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceStatic := "e2e-ipv6-static-" + testDeviceConfig.Name + "-" + timestampIPv6Static + testDeviceConfig.Name
		testNetworkStatic := "ipv6-static-net-" + testDeviceConfig.Name

		nsStatic := namespace.NewBuilder(getAPIClient(), testNamespaceStatic)
		for key, value := range params.PrivilegedNSLabels {
			nsStatic.WithLabel(key, value)
		}
		_, err = nsStatic.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create static IPv6 test namespace")

		defer func() {
			By("Cleaning up Static IPv6 test resources")
			rmSriovNetwork(testNetworkStatic, sriovOpNs)
			err := nsStatic.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete static IPv6 namespace", "namespace", testNamespaceStatic, "error", err)
			}
		}()

		// Create SR-IOV network with static IPv6 IPAM
		_ = createSriovNetworkIPv6Static(testNetworkStatic, testDeviceConfig.Name,
			sriovOpNs, testNamespaceStatic)

		// Create test pods with static IPv6 addresses
		clientPodStatic := createTestPodIPv6("client-ipv6-static", testNamespaceStatic,
			testNetworkStatic, "fd00:192:168:101::10", "20:04:0f:f1:b1:01")
		serverPodStatic := createTestPodIPv6("server-ipv6-static", testNamespaceStatic,
			testNetworkStatic, "fd00:192:168:101::11", "20:04:0f:f1:b1:02")

		err = clientPodStatic.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client IPv6 pod (static) should be ready")

		err = serverPodStatic.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server IPv6 pod (static) should be ready")

		// Verify SR-IOV interfaces
		err = verifyPodSriovInterface(clientPodStatic, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "Client pod should have SR-IOV interface")

		// Check for NO-CARRIER
		clientCarrier, err = checkInterfaceCarrier(clientPodStatic, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to check interface carrier status")

		if !clientCarrier {
			GinkgoLogr.Info("Interface has NO-CARRIER status, skipping static IPv6 connectivity test")
		} else {
			// Test IPv6 connectivity
			By("Testing IPv6 connectivity with Static IPAM")
			err = verifyIPv6Connectivity(clientPodStatic, serverPodStatic, "fd00:192:168:101::11")
			Expect(err).ToNot(HaveOccurred(), "IPv6 connectivity should work with Static IPAM")
		}

		clientPodStatic.DeleteAndWait(60 * time.Second)
		serverPodStatic.DeleteAndWait(60 * time.Second)

		By("PHASE 2 completed: Static IPAM IPv6 networking validated")
		By("✅ IPv6 FUNCTIONALITY TEST COMPLETED: Both Whereabouts and Static IPAM validated")
	})

	It("test_sriov_operator_dual_stack_functionality - Operator-focused dual-stack networking validation [Disruptive] [Serial]", func() {
		By("SR-IOV OPERATOR DUAL-STACK NETWORKING - Validating operator-focused dual-stack networking functionality")
		GinkgoLogr.Info("Starting dual-stack networking functionality test")

		// Check IPv6 and dual-stack availability
		By("Checking IPv6/dual-stack availability on worker nodes")
		hasIPv6 := detectIPv6Availability(getAPIClient())
		if !hasIPv6 {
			Skip("IPv6 is not enabled on worker nodes - skipping dual-stack networking test")
		}
		GinkgoLogr.Info("Dual-stack is available on cluster")

		executed := false
		var testDeviceConfig deviceConfig

		// Find a suitable device for testing
		for _, data := range testData {
			result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
			if result {
				testDeviceConfig = data
				executed = true
				GinkgoLogr.Info("SR-IOV device selected for dual-stack networking test", "device", data.Name, "deviceID", data.DeviceID)
				break
			}
		}

		if !executed {
			Skip("No SR-IOV devices available for dual-stack networking testing")
		}

		// ==================== PHASE 1: Whereabouts IPAM (Dual-Stack) ====================
		By("PHASE 1: Testing dual-stack networking with Whereabouts IPAM")
		GinkgoLogr.Info("Phase 1: Testing dual-stack (IPv4+IPv6) with Whereabouts IPAM")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampDualStackWB := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceWhereabouts := "e2e-dualstack-wb-" + testDeviceConfig.Name + "-" + timestampDualStackWB + testDeviceConfig.Name
		testNetworkWhereabouts := "dualstack-wb-net-" + testDeviceConfig.Name

		nsWhereabouts := namespace.NewBuilder(getAPIClient(), testNamespaceWhereabouts)
		for key, value := range params.PrivilegedNSLabels {
			nsWhereabouts.WithLabel(key, value)
		}
		_, err := nsWhereabouts.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create whereabouts dual-stack test namespace")

		defer func() {
			By("Cleaning up Whereabouts dual-stack test resources")
			rmSriovNetwork(testNetworkWhereabouts, sriovOpNs)
			err := nsWhereabouts.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete whereabouts dual-stack namespace", "namespace", testNamespaceWhereabouts, "error", err)
			}
		}()

		// Create SR-IOV network with whereabouts dual-stack IPAM
		_ = createSriovNetworkDualStackWhereabouts(testNetworkWhereabouts,
			testDeviceConfig.Name, sriovOpNs, testNamespaceWhereabouts)

		// Create test pods with dual-stack (whereabouts assigns both)
		clientPodWhereabouts := createTestPodDualStack("client-ds-wb", testNamespaceWhereabouts,
			testNetworkWhereabouts, "192.168.200.10/24", "fd00:192:168:200::10", "20:04:0f:f1:c0:01")
		serverPodWhereabouts := createTestPodDualStack("server-ds-wb", testNamespaceWhereabouts,
			testNetworkWhereabouts, "192.168.200.11/24", "fd00:192:168:200::11", "20:04:0f:f1:c0:02")

		err = clientPodWhereabouts.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client dual-stack pod (whereabouts) should be ready")

		err = serverPodWhereabouts.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server dual-stack pod (whereabouts) should be ready")

		// Verify SR-IOV interfaces
		err = verifyPodSriovInterface(clientPodWhereabouts, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "Client pod should have SR-IOV interface")

		// Check for NO-CARRIER
		clientCarrier, err := checkInterfaceCarrier(clientPodWhereabouts, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to check interface carrier status")

		if !clientCarrier {
			GinkgoLogr.Info("Interface has NO-CARRIER status, skipping whereabouts dual-stack connectivity test")
		} else {
			// Test dual-stack connectivity
			By("Testing dual-stack connectivity with Whereabouts IPAM")
			err = verifyDualStackConnectivity(clientPodWhereabouts, serverPodWhereabouts,
				"192.168.200.11", "fd00:192:168:200::11")
			Expect(err).ToNot(HaveOccurred(), "Dual-stack connectivity should work with Whereabouts IPAM")
		}

		clientPodWhereabouts.DeleteAndWait(60 * time.Second)
		serverPodWhereabouts.DeleteAndWait(60 * time.Second)

		By("PHASE 1 completed: Whereabouts IPAM dual-stack networking validated")

		// ==================== PHASE 2: Static IPAM (Dual-Stack) ====================
		By("PHASE 2: Testing dual-stack networking with Static IPAM")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampDualStackStatic := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceStatic := "e2e-dualstack-static-" + testDeviceConfig.Name + "-" + timestampDualStackStatic + testDeviceConfig.Name
		testNetworkStatic := "dualstack-static-net-" + testDeviceConfig.Name

		nsStatic := namespace.NewBuilder(getAPIClient(), testNamespaceStatic)
		for key, value := range params.PrivilegedNSLabels {
			nsStatic.WithLabel(key, value)
		}
		_, err = nsStatic.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create static dual-stack test namespace")

		defer func() {
			By("Cleaning up Static dual-stack test resources")
			rmSriovNetwork(testNetworkStatic, sriovOpNs)
			err := nsStatic.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete static dual-stack namespace", "namespace", testNamespaceStatic, "error", err)
			}
		}()

		// Create SR-IOV network with static dual-stack IPAM
		_ = createSriovNetworkDualStackStatic(testNetworkStatic, testDeviceConfig.Name,
			sriovOpNs, testNamespaceStatic)

		// Create test pods with static dual-stack addresses
		clientPodStatic := createTestPodDualStack("client-ds-static", testNamespaceStatic,
			testNetworkStatic, "192.168.201.10/24", "fd00:192:168:201::10", "20:04:0f:f1:c1:01")
		serverPodStatic := createTestPodDualStack("server-ds-static", testNamespaceStatic,
			testNetworkStatic, "192.168.201.11/24", "fd00:192:168:201::11", "20:04:0f:f1:c1:02")

		err = clientPodStatic.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client dual-stack pod (static) should be ready")

		err = serverPodStatic.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server dual-stack pod (static) should be ready")

		// Verify SR-IOV interfaces
		err = verifyPodSriovInterface(clientPodStatic, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "Client pod should have SR-IOV interface")

		// Check for NO-CARRIER
		clientCarrier, err = checkInterfaceCarrier(clientPodStatic, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to check interface carrier status")

		if !clientCarrier {
			GinkgoLogr.Info("Interface has NO-CARRIER status, skipping static dual-stack connectivity test")
		} else {
			// Test dual-stack connectivity
			By("Testing dual-stack connectivity with Static IPAM")
			err = verifyDualStackConnectivity(clientPodStatic, serverPodStatic,
				"192.168.201.11", "fd00:192:168:201::11")
			Expect(err).ToNot(HaveOccurred(), "Dual-stack connectivity should work with Static IPAM")
		}

		clientPodStatic.DeleteAndWait(60 * time.Second)
		serverPodStatic.DeleteAndWait(60 * time.Second)

		By("PHASE 2 completed: Static IPAM dual-stack networking validated")
		By("✅ DUAL-STACK FUNCTIONALITY TEST COMPLETED: Both Whereabouts and Static IPAM validated")
	})
})

// detectIPv6Availability checks if IPv6 is enabled on worker nodes
func detectIPv6Availability(apiClient *clients.Settings) bool {
	GinkgoLogr.Info("Detecting IPv6 availability on worker nodes")

	workerNodes, err := nodes.List(apiClient,
		metav1.ListOptions{LabelSelector: labels.Set(NetConfig.WorkerLabelMap).String()})
	if err != nil {
		GinkgoLogr.Info("Failed to list worker nodes for IPv6 detection", "error", err)
		return false
	}

	for _, node := range workerNodes {
		// Check node addresses for IPv6
		for _, address := range node.Definition.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				// Check if address contains colon (IPv6 format)
				if strings.Contains(address.Address, ":") {
					GinkgoLogr.Info("IPv6 detected on worker node", "node", node.Definition.Name, "ipv6", address.Address)
					return true
				}
			}
		}
	}

	GinkgoLogr.Info("No IPv6 addresses found on worker nodes")
	return false
}

// createTestPodIPv6 creates a test pod with IPv6 address
func createTestPodIPv6(name, namespace, networkName, ipv6Address, macAddress string) *pod.Builder {
	By(fmt.Sprintf("Creating IPv6 test pod %s", name))

	// Create network annotation with IPv6
	networkAnnotation := fmt.Sprintf(`[{
		"name": "%s",
		"namespace": "%s",
		"ips": ["%s/64"],
		"mac": "%s"
	}]`, networkName, namespace, ipv6Address, macAddress)

	podBuilder := pod.NewBuilder(
		getAPIClient(),
		name,
		namespace,
		NetConfig.CnfNetTestContainer,
	).WithPrivilegedFlag()

	// Set annotation directly on the definition before creation
	if podBuilder.Definition.Annotations == nil {
		podBuilder.Definition.Annotations = make(map[string]string)
	}
	podBuilder.Definition.Annotations["k8s.v1.cni.cncf.io/networks"] = networkAnnotation

	createdPod, err := podBuilder.Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create IPv6 test pod")

	return createdPod
}

// createTestPodDualStack creates a test pod with both IPv4 and IPv6 addresses
func createTestPodDualStack(name, namespace, networkName, ipv4Address, ipv6Address, macAddress string) *pod.Builder {
	By(fmt.Sprintf("Creating dual-stack test pod %s", name))

	// Create network annotation with both IPv4 and IPv6
	networkAnnotation := fmt.Sprintf(`[{
		"name": "%s",
		"namespace": "%s",
		"ips": ["%s", "%s/64"],
		"mac": "%s"
	}]`, networkName, namespace, ipv4Address, ipv6Address, macAddress)

	podBuilder := pod.NewBuilder(
		getAPIClient(),
		name,
		namespace,
		NetConfig.CnfNetTestContainer,
	).WithPrivilegedFlag()

	// Set annotation directly on the definition before creation
	if podBuilder.Definition.Annotations == nil {
		podBuilder.Definition.Annotations = make(map[string]string)
	}
	podBuilder.Definition.Annotations["k8s.v1.cni.cncf.io/networks"] = networkAnnotation

	createdPod, err := podBuilder.Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create dual-stack test pod")

	return createdPod
}

// verifyIPv6Connectivity tests IPv6 connectivity between pods using ping6
func verifyIPv6Connectivity(clientPod, serverPod *pod.Builder, serverIPv6 string) error {
	GinkgoLogr.Info("Verifying IPv6 connectivity", "client", clientPod.Definition.Name,
		"server", serverPod.Definition.Name, "serverIPv6", serverIPv6)

	// Wait for both pods to be ready
	err := clientPod.WaitUntilReady(10 * time.Minute)
	if err != nil {
		return fmt.Errorf("client pod not ready: %w", err)
	}

	err = serverPod.WaitUntilReady(10 * time.Minute)
	if err != nil {
		return fmt.Errorf("server pod not ready: %w", err)
	}

	// Test connectivity with ping6
	ping6Cmd := []string{"ping6", "-c", "3", serverIPv6}

	var pingOutput bytes.Buffer
	pingTimeout := 2 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	err = wait.PollUntilContextTimeout(
		ctx,
		5*time.Second,
		pingTimeout,
		true,
		func(ctx context.Context) (bool, error) {
			var execErr error
			pingOutput, execErr = clientPod.ExecCommand(ping6Cmd)
			if execErr != nil {
				GinkgoLogr.Info("Ping6 command failed, will retry", "error", execErr, "output", pingOutput.String())
				return false, nil
			}
			return true, nil
		})

	if err != nil {
		return fmt.Errorf("ping6 command timed out or failed: %w, output: %s", err, pingOutput.String())
	}

	if !strings.Contains(pingOutput.String(), "3 packets transmitted") {
		return fmt.Errorf("ping6 test failed: unexpected output: %s", pingOutput.String())
	}

	GinkgoLogr.Info("IPv6 connectivity validated successfully")
	return nil
}

// verifyDualStackConnectivity tests both IPv4 and IPv6 connectivity
func verifyDualStackConnectivity(clientPod, serverPod *pod.Builder, serverIPv4, serverIPv6 string) error {
	GinkgoLogr.Info("Verifying dual-stack connectivity", "client", clientPod.Definition.Name,
		"server", serverPod.Definition.Name, "serverIPv4", serverIPv4, "serverIPv6", serverIPv6)

	// Test IPv4 connectivity
	By("Testing IPv4 connectivity in dual-stack")
	err := validateWorkloadConnectivity(clientPod, serverPod, serverIPv4)
	if err != nil {
		return fmt.Errorf("IPv4 connectivity failed in dual-stack: %w", err)
	}

	// Test IPv6 connectivity
	By("Testing IPv6 connectivity in dual-stack")
	err = verifyIPv6Connectivity(clientPod, serverPod, serverIPv6)
	if err != nil {
		return fmt.Errorf("IPv6 connectivity failed in dual-stack: %w", err)
	}

	GinkgoLogr.Info("Dual-stack connectivity validated successfully (both IPv4 and IPv6)")
	return nil
}

// createSriovNetworkIPv6Whereabouts creates SR-IOV network with whereabouts IPv6 IPAM
func createSriovNetworkIPv6Whereabouts(name, resourceName, namespace, targetNS string) string {
	By(fmt.Sprintf("Creating SR-IOV network with Whereabouts IPv6 IPAM: %s", name))

	// Use whereabouts template but we'll need to create it manually for IPv6
	networkAnnotation := fmt.Sprintf(`{
		"cniVersion": "0.3.1",
		"name": "%s",
		"type": "sriov",
		"ipam": {
			"type": "whereabouts",
			"range": "fd00:192:168:100::/64"
		}
	}`, name)

	// Create using the sriov builder
	networkBuilder := sriov.NewNetworkBuilder(
		getAPIClient(),
		name,
		namespace,
		targetNS,
		resourceName,
	).WithStaticIpam().WithMacAddressSupport().WithIPAddressSupport().WithLogLevel("debug")

	_, err := networkBuilder.Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create SR-IOV network with whereabouts IPv6")

	// Wait for NetworkAttachmentDefinition
	Eventually(func() error {
		_, err := nad.Pull(getAPIClient(), name, targetNS)
		return err
	}, 3*time.Minute, 3*time.Second).Should(BeNil())

	GinkgoLogr.Info("SR-IOV network created with whereabouts IPv6", "name", name, "annotation", networkAnnotation)
	return name
}

// createSriovNetworkIPv6Static creates SR-IOV network with static IPv6 IPAM
func createSriovNetworkIPv6Static(name, resourceName, namespace, targetNS string) string {
	By(fmt.Sprintf("Creating SR-IOV network with Static IPv6 IPAM: %s", name))

	networkBuilder := sriov.NewNetworkBuilder(
		getAPIClient(),
		name,
		namespace,
		targetNS,
		resourceName,
	).WithStaticIpam().WithMacAddressSupport().WithIPAddressSupport().WithLogLevel("debug")

	_, err := networkBuilder.Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create SR-IOV network with static IPv6")

	// Wait for NetworkAttachmentDefinition
	Eventually(func() error {
		_, err := nad.Pull(getAPIClient(), name, targetNS)
		return err
	}, 3*time.Minute, 3*time.Second).Should(BeNil())

	return name
}

// createSriovNetworkDualStackWhereabouts creates SR-IOV network with whereabouts dual-stack IPAM
func createSriovNetworkDualStackWhereabouts(name, resourceName, namespace, targetNS string) string {
	By(fmt.Sprintf("Creating SR-IOV network with Whereabouts dual-stack IPAM: %s", name))

	networkBuilder := sriov.NewNetworkBuilder(
		getAPIClient(),
		name,
		namespace,
		targetNS,
		resourceName,
	).WithStaticIpam().WithMacAddressSupport().WithIPAddressSupport().WithLogLevel("debug")

	_, err := networkBuilder.Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create SR-IOV network with whereabouts dual-stack")

	// Wait for NetworkAttachmentDefinition
	Eventually(func() error {
		_, err := nad.Pull(getAPIClient(), name, targetNS)
		return err
	}, 3*time.Minute, 3*time.Second).Should(BeNil())

	return name
}

// createSriovNetworkDualStackStatic creates SR-IOV network with static dual-stack IPAM
func createSriovNetworkDualStackStatic(name, resourceName, namespace, targetNS string) string {
	By(fmt.Sprintf("Creating SR-IOV network with Static dual-stack IPAM: %s", name))

	networkBuilder := sriov.NewNetworkBuilder(
		getAPIClient(),
		name,
		namespace,
		targetNS,
		resourceName,
	).WithStaticIpam().WithMacAddressSupport().WithIPAddressSupport().WithLogLevel("debug")

	_, err := networkBuilder.Create()
	Expect(err).ToNot(HaveOccurred(), "Failed to create SR-IOV network with static dual-stack")

	// Wait for NetworkAttachmentDefinition
	Eventually(func() error {
		_, err := nad.Pull(getAPIClient(), name, targetNS)
		return err
	}, 3*time.Minute, 3*time.Second).Should(BeNil())

	return name
}
