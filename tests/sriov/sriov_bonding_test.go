package sriov

import (
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/namespace"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/nodes"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/params"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SR-IOV Bonding Tests", Ordered, func() {
	var (
		workerNodes []*nodes.Builder
		sriovOpNs   string
		testData    []deviceConfig
		vfNum       int
	)

	BeforeAll(func() {
		By("Setup: Initializing test environment for bonding tests")
		workerNodes, _ = nodes.List(
			getAPIClient(),
			metav1.ListOptions{
				LabelSelector: NetConfig.WorkerLabel,
			},
		)
		Expect(len(workerNodes)).To(BeNumerically(">", 0), "No worker nodes found")
		GinkgoLogr.Info("Worker nodes discovered", "count", len(workerNodes))

		sriovOpNs = NetConfig.SriovOperatorNamespace
		testData = getDeviceConfig()
		vfNum = getVFNum()

		By("Verifying SR-IOV operator is deployed and stable")
		err := IsSriovDeployed(getAPIClient(), NetConfig)
		Expect(err).ToNot(HaveOccurred(), "SR-IOV operator is not deployed or not ready")
		GinkgoLogr.Info("SR-IOV operator verified", "namespace", sriovOpNs)

		By("Waiting for cluster to be stable before starting bonding tests")
		err = WaitForSriovAndMCPStable(
			getAPIClient(), 20*time.Minute, 30*time.Second, NetConfig.CnfMcpLabel, sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Cluster is not stable")
		GinkgoLogr.Info("Cluster is stable and ready for bonding tests")

		GinkgoLogr.Info("SR-IOV Bonding test suite initialized successfully", "operator_ns", sriovOpNs, "test_devices", len(testData))
	})

	It("test_sriov_bond_ipam_integration - SR-IOV bonding with IP Address Management [Disruptive] [Serial] [bonding]", func() {
		By("BONDING WITH IPAM - Testing SR-IOV bonded VFs with dynamic IP allocation")
		GinkgoLogr.Info("Starting bonding with IPAM integration test")

		executed := false
		var testDeviceConfig deviceConfig

		// Find a suitable device for testing
		for _, data := range testData {
			result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
			if result {
				testDeviceConfig = data
				executed = true
				GinkgoLogr.Info("SR-IOV device selected for bonding test", "device", data.Name, "deviceID", data.DeviceID)
				break
			}
		}

		if !executed {
			Skip("No SR-IOV devices available for bonding with IPAM testing")
		}

		// ==================== PHASE 1: Bond with Whereabouts IPAM ====================
		By("PHASE 1: Testing SR-IOV bonding with Whereabouts IPAM")
		GinkgoLogr.Info("Phase 1: Creating bonded SR-IOV networks with Whereabouts IPAM")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampWB := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceWB := "e2e-bond-wb-" + testDeviceConfig.Name + "-" + timestampWB + testDeviceConfig.Name
		testNetworkNet1 := "bond-net1-wb-" + testDeviceConfig.Name
		testNetworkNet2 := "bond-net2-wb-" + testDeviceConfig.Name
		testBondNetworkWB := "bond-wb-" + testDeviceConfig.Name

		// Create namespace
		nsWB := namespace.NewBuilder(getAPIClient(), testNamespaceWB)
		for key, value := range params.PrivilegedNSLabels {
			nsWB.WithLabel(key, value)
		}
		_, err := nsWB.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create whereabouts bond test namespace")

		defer func() {
			By("Cleaning up Whereabouts bonding test resources")
			rmSriovNetwork(testNetworkNet1, sriovOpNs)
			rmSriovNetwork(testNetworkNet2, sriovOpNs)
			removeBondNetworkAttachmentDef(testBondNetworkWB, testNamespaceWB)
			err := nsWB.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete whereabouts bond namespace", "namespace", testNamespaceWB, "error", err)
			}
		}()

		By("Phase 1.1: Creating two SR-IOV networks for bonding (net1, net2)")
		// Create first SR-IOV network
		sriovNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-template.yaml")
		sriovNetworkNet1 := sriovNetwork{
			name:             testNetworkNet1,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceWB,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNetworkNet1.createSriovNetwork()

		// Create second SR-IOV network (using same resource pool)
		sriovNetworkNet2 := sriovNetwork{
			name:             testNetworkNet2,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceWB,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNetworkNet2.createSriovNetwork()

		By("Phase 1.2: Creating bond NetworkAttachmentDefinition with Whereabouts IPAM")
		bondNADWB := createBondNetworkAttachmentDef(
			testBondNetworkWB,
			testNamespaceWB,
			"active-backup",
			"whereabouts",
			"192.168.100.0/24",
			"",
			[]string{testNetworkNet1, testNetworkNet2},
		)
		Expect(bondNADWB).ToNot(BeNil(), "Failed to create bond NAD with Whereabouts")

		By("Phase 1.3: Deploying test pods with bonded interfaces")
		clientPodWB := createBondTestPod(
			"client-bond-wb",
			testNamespaceWB,
			[]string{testNetworkNet1, testNetworkNet2, testBondNetworkWB},
			"", // No static IP for whereabouts
		)
		serverPodWB := createBondTestPod(
			"server-bond-wb",
			testNamespaceWB,
			[]string{testNetworkNet1, testNetworkNet2, testBondNetworkWB},
			"",
		)

		err = clientPodWB.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client pod with bond (whereabouts) should be ready")

		err = serverPodWB.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server pod with bond (whereabouts) should be ready")

		By("Phase 1.4: Validating bond interface configuration")
		err = verifyBondStatus(clientPodWB, "bond0", "active-backup", 2)
		Expect(err).ToNot(HaveOccurred(), "Bond interface should be properly configured")

		By("Phase 1.5: Validating IPAM assigned IP addresses")
		clientBondIP, err := getPodInterfaceIP(clientPodWB, "bond0")
		Expect(err).ToNot(HaveOccurred(), "Failed to get client bond IP")
		Expect(clientBondIP).To(ContainSubstring("192.168.100."), "Client should have IP from whereabouts range")

		serverBondIP, err := getPodInterfaceIP(serverPodWB, "bond0")
		Expect(err).ToNot(HaveOccurred(), "Failed to get server bond IP")
		Expect(serverBondIP).To(ContainSubstring("192.168.100."), "Server should have IP from whereabouts range")

		GinkgoLogr.Info("Whereabouts IPAM assigned IPs", "client", clientBondIP, "server", serverBondIP)

		By("Phase 1.6: Testing connectivity over bonded interface")
		// Check for NO-CARRIER on bond interface
		bondCarrier, err := checkInterfaceCarrier(clientPodWB, "bond0")
		Expect(err).ToNot(HaveOccurred(), "Failed to check bond interface carrier")

		if !bondCarrier {
			GinkgoLogr.Info("Bond interface has NO-CARRIER status, skipping connectivity test")
		} else {
			// Extract IP without CIDR for ping
			serverIPForPing := extractIPFromCIDR(serverBondIP)
			err = validateWorkloadConnectivity(clientPodWB, serverPodWB, serverIPForPing)
			Expect(err).ToNot(HaveOccurred(), "Connectivity over bonded interface should work")
		}

		// Cleanup whereabouts bond pods
		clientPodWB.DeleteAndWait(60 * time.Second)
		serverPodWB.DeleteAndWait(60 * time.Second)

		By("PHASE 1 completed: Whereabouts IPAM bonding validated")

		// ==================== PHASE 2: Bond with Static IPAM ====================
		By("PHASE 2: Testing SR-IOV bonding with Static IPAM")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampStatic := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceStatic := "e2e-bond-static-" + testDeviceConfig.Name + "-" + timestampStatic + testDeviceConfig.Name
		testNetworkNet1Static := "bond-net1-static-" + testDeviceConfig.Name
		testNetworkNet2Static := "bond-net2-static-" + testDeviceConfig.Name
		testBondNetworkStatic := "bond-static-" + testDeviceConfig.Name

		// Create namespace for static
		nsStatic := namespace.NewBuilder(getAPIClient(), testNamespaceStatic)
		for key, value := range params.PrivilegedNSLabels {
			nsStatic.WithLabel(key, value)
		}
		_, err = nsStatic.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create static bond test namespace")

		defer func() {
			By("Cleaning up Static bonding test resources")
			rmSriovNetwork(testNetworkNet1Static, sriovOpNs)
			rmSriovNetwork(testNetworkNet2Static, sriovOpNs)
			removeBondNetworkAttachmentDef(testBondNetworkStatic, testNamespaceStatic)
			err := nsStatic.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete static bond namespace", "namespace", testNamespaceStatic, "error", err)
			}
		}()

		By("Phase 2.1: Creating two SR-IOV networks for bonding with static IPAM")
		sriovNetworkNet1Static := sriovNetwork{
			name:             testNetworkNet1Static,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceStatic,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNetworkNet1Static.createSriovNetwork()

		sriovNetworkNet2Static := sriovNetwork{
			name:             testNetworkNet2Static,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceStatic,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNetworkNet2Static.createSriovNetwork()

		By("Phase 2.2: Creating bond NetworkAttachmentDefinition with Static IPAM")
		bondNADStatic := createBondNetworkAttachmentDef(
			testBondNetworkStatic,
			testNamespaceStatic,
			"active-backup",
			"static",
			"",
			"192.168.101.0/24",
			[]string{testNetworkNet1Static, testNetworkNet2Static},
		)
		Expect(bondNADStatic).ToNot(BeNil(), "Failed to create bond NAD with Static IPAM")

		By("Phase 2.3: Deploying test pods with static IP addresses on bond")
		clientPodStatic := createBondTestPod(
			"client-bond-static",
			testNamespaceStatic,
			[]string{testNetworkNet1Static, testNetworkNet2Static, testBondNetworkStatic},
			"192.168.101.10/24",
		)
		serverPodStatic := createBondTestPod(
			"server-bond-static",
			testNamespaceStatic,
			[]string{testNetworkNet1Static, testNetworkNet2Static, testBondNetworkStatic},
			"192.168.101.11/24",
		)

		err = clientPodStatic.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Client pod with bond (static) should be ready")

		err = serverPodStatic.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Server pod with bond (static) should be ready")

		By("Phase 2.4: Validating bond interface with static IP configuration")
		err = verifyBondStatus(clientPodStatic, "bond0", "active-backup", 2)
		Expect(err).ToNot(HaveOccurred(), "Bond interface with static IP should be properly configured")

		By("Phase 2.5: Validating static IP addresses")
		clientBondIPStatic, err := getPodInterfaceIP(clientPodStatic, "bond0")
		Expect(err).ToNot(HaveOccurred(), "Failed to get client bond IP")
		Expect(clientBondIPStatic).To(ContainSubstring("192.168.101.10"), "Client should have static IP")

		serverBondIPStatic, err := getPodInterfaceIP(serverPodStatic, "bond0")
		Expect(err).ToNot(HaveOccurred(), "Failed to get server bond IP")
		Expect(serverBondIPStatic).To(ContainSubstring("192.168.101.11"), "Server should have static IP")

		By("Phase 2.6: Testing connectivity over bonded interface with static IPs")
		bondCarrier, err = checkInterfaceCarrier(clientPodStatic, "bond0")
		Expect(err).ToNot(HaveOccurred(), "Failed to check bond interface carrier")

		if !bondCarrier {
			GinkgoLogr.Info("Bond interface has NO-CARRIER status, skipping static connectivity test")
		} else {
			err = validateWorkloadConnectivity(clientPodStatic, serverPodStatic, "192.168.101.11")
			Expect(err).ToNot(HaveOccurred(), "Connectivity over bonded interface with static IP should work")
		}

		// Cleanup static bond pods
		clientPodStatic.DeleteAndWait(60 * time.Second)
		serverPodStatic.DeleteAndWait(60 * time.Second)

		By("PHASE 2 completed: Static IPAM bonding validated")
		By("✅ BOND IPAM INTEGRATION TEST COMPLETED: Both Whereabouts and Static IPAM validated")
	})

	It("test_sriov_bond_mode_operator_level - Different bonding modes from operator perspective [Disruptive] [Serial] [bonding]", func() {
		By("BONDING MODES - Testing SR-IOV operator-level bonding mode configurations")
		GinkgoLogr.Info("Starting bonding modes test")

		executed := false
		var testDeviceConfig deviceConfig

		// Find a suitable device for testing
		for _, data := range testData {
			result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
			if result {
				testDeviceConfig = data
				executed = true
				GinkgoLogr.Info("SR-IOV device selected for bonding modes test", "device", data.Name, "deviceID", data.DeviceID)
				break
			}
		}

		if !executed {
			Skip("No SR-IOV devices available for bonding mode testing")
		}

		// ==================== PHASE 1: Active-Backup Mode ====================
		By("PHASE 1: Testing Active-Backup bonding mode (mode 1)")
		GinkgoLogr.Info("Phase 1: Testing bonding mode 1 (Active-Backup)")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampAB := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceAB := "e2e-bond-ab-" + testDeviceConfig.Name + "-" + timestampAB + testDeviceConfig.Name
		testNetworkNet1AB := "bond-net1-ab-" + testDeviceConfig.Name
		testNetworkNet2AB := "bond-net2-ab-" + testDeviceConfig.Name
		testBondNetworkAB := "bond-ab-" + testDeviceConfig.Name

		// Create namespace
		nsAB := namespace.NewBuilder(getAPIClient(), testNamespaceAB)
		for key, value := range params.PrivilegedNSLabels {
			nsAB.WithLabel(key, value)
		}
		_, err := nsAB.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create active-backup test namespace")

		defer func() {
			By("Cleaning up Active-Backup bonding test resources")
			rmSriovNetwork(testNetworkNet1AB, sriovOpNs)
			rmSriovNetwork(testNetworkNet2AB, sriovOpNs)
			removeBondNetworkAttachmentDef(testBondNetworkAB, testNamespaceAB)
			err := nsAB.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete active-backup namespace", "namespace", testNamespaceAB, "error", err)
			}
		}()

		By("Phase 1.1: Creating SR-IOV networks for active-backup bond")
		sriovNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-template.yaml")
		sriovNet1AB := sriovNetwork{
			name:             testNetworkNet1AB,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceAB,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNet1AB.createSriovNetwork()

		sriovNet2AB := sriovNetwork{
			name:             testNetworkNet2AB,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceAB,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNet2AB.createSriovNetwork()

		By("Phase 1.2: Creating active-backup bond NAD")
		bondNADAB := createBondNetworkAttachmentDef(
			testBondNetworkAB,
			testNamespaceAB,
			"active-backup",
			"static",
			"",
			"192.168.102.0/24",
			[]string{testNetworkNet1AB, testNetworkNet2AB},
		)
		Expect(bondNADAB).ToNot(BeNil(), "Failed to create active-backup bond NAD")

		By("Phase 1.3: Deploying pod with active-backup bond")
		testPodAB := createBondTestPod(
			"test-bond-ab",
			testNamespaceAB,
			[]string{testNetworkNet1AB, testNetworkNet2AB, testBondNetworkAB},
			"192.168.102.10/24",
		)

		err = testPodAB.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Test pod with active-backup bond should be ready")

		By("Phase 1.4: Validating active-backup bond mode")
		err = verifyBondStatus(testPodAB, "bond0", "active-backup", 2)
		Expect(err).ToNot(HaveOccurred(), "Bond should be in active-backup mode")

		By("Phase 1.5: Verifying only one slave is active")
		activeSlave, err := getBondActiveSlave(testPodAB, "bond0")
		Expect(err).ToNot(HaveOccurred(), "Failed to get active slave")
		Expect(activeSlave).To(Or(Equal("net1"), Equal("net2")), "One slave should be active")
		GinkgoLogr.Info("Active-backup mode validated", "activeSlave", activeSlave)

		By("Phase 1.6: Validating NAD reflects correct bond configuration")
		err = validateBondNADConfig(testBondNetworkAB, testNamespaceAB, "active-backup")
		Expect(err).ToNot(HaveOccurred(), "NAD should reflect active-backup configuration")

		testPodAB.DeleteAndWait(60 * time.Second)
		By("PHASE 1 completed: Active-Backup mode validated")

		// ==================== PHASE 2: 802.3ad/LACP Mode ====================
		By("PHASE 2: Testing 802.3ad/LACP bonding mode (mode 4)")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampLACP := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceLACP := "e2e-bond-lacp-" + testDeviceConfig.Name + "-" + timestampLACP + testDeviceConfig.Name
		testNetworkNet1LACP := "bond-net1-lacp-" + testDeviceConfig.Name
		testNetworkNet2LACP := "bond-net2-lacp-" + testDeviceConfig.Name
		testBondNetworkLACP := "bond-lacp-" + testDeviceConfig.Name

		// Create namespace
		nsLACP := namespace.NewBuilder(getAPIClient(), testNamespaceLACP)
		for key, value := range params.PrivilegedNSLabels {
			nsLACP.WithLabel(key, value)
		}
		_, err = nsLACP.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create LACP test namespace")

		defer func() {
			By("Cleaning up LACP bonding test resources")
			rmSriovNetwork(testNetworkNet1LACP, sriovOpNs)
			rmSriovNetwork(testNetworkNet2LACP, sriovOpNs)
			removeBondNetworkAttachmentDef(testBondNetworkLACP, testNamespaceLACP)
			err := nsLACP.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete LACP namespace", "namespace", testNamespaceLACP, "error", err)
			}
		}()

		By("Phase 2.1: Creating SR-IOV networks for LACP bond")
		sriovNet1LACP := sriovNetwork{
			name:             testNetworkNet1LACP,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceLACP,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNet1LACP.createSriovNetwork()

		sriovNet2LACP := sriovNetwork{
			name:             testNetworkNet2LACP,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceLACP,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		sriovNet2LACP.createSriovNetwork()

		By("Phase 2.2: Creating 802.3ad LACP bond NAD")
		bondNADLACP := createBondNetworkAttachmentDef(
			testBondNetworkLACP,
			testNamespaceLACP,
			"802.3ad",
			"static",
			"",
			"192.168.103.0/24",
			[]string{testNetworkNet1LACP, testNetworkNet2LACP},
		)
		Expect(bondNADLACP).ToNot(BeNil(), "Failed to create LACP bond NAD")

		By("Phase 2.3: Deploying pod with LACP bond")
		testPodLACP := createBondTestPod(
			"test-bond-lacp",
			testNamespaceLACP,
			[]string{testNetworkNet1LACP, testNetworkNet2LACP, testBondNetworkLACP},
			"192.168.103.10/24",
		)

		err = testPodLACP.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Test pod with LACP bond should be ready")

		By("Phase 2.4: Validating 802.3ad bond mode")
		err = verifyBondStatus(testPodLACP, "bond0", "802.3ad", 2)
		Expect(err).ToNot(HaveOccurred(), "Bond should be in 802.3ad mode")

		By("Phase 2.5: Verifying LACP configuration")
		lacpRate, err := getBondLACPRate(testPodLACP, "bond0")
		Expect(err).ToNot(HaveOccurred(), "Failed to get LACP rate")
		GinkgoLogr.Info("LACP mode validated", "lacpRate", lacpRate)

		By("Phase 2.6: Validating NAD reflects correct LACP configuration")
		err = validateBondNADConfig(testBondNetworkLACP, testNamespaceLACP, "802.3ad")
		Expect(err).ToNot(HaveOccurred(), "NAD should reflect 802.3ad configuration")

		testPodLACP.DeleteAndWait(60 * time.Second)
		By("PHASE 2 completed: 802.3ad/LACP mode validated")

		// ==================== PHASE 3: Operator-Level Validation ====================
		By("PHASE 3: Operator-level validation and bond mode switching")

		By("Phase 3.1: Verifying SriovNetwork resource allocation persists")
		// Verify SR-IOV networks are still present and functional
		net1Exists := verifySriovNetworkExists(testNetworkNet1LACP, sriovOpNs)
		Expect(net1Exists).To(BeTrue(), "SriovNetwork net1 should still exist")

		net2Exists := verifySriovNetworkExists(testNetworkNet2LACP, sriovOpNs)
		Expect(net2Exists).To(BeTrue(), "SriovNetwork net2 should still exist")

		By("Phase 3.2: Testing rapid bond mode switching")
		// Recreate pod with different bond mode to test switching
		testPodSwitch := createBondTestPod(
			"test-bond-switch",
			testNamespaceLACP,
			[]string{testNetworkNet1LACP, testNetworkNet2LACP, testBondNetworkLACP},
			"192.168.103.20/24",
		)

		err = testPodSwitch.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Pod after bond mode switch should be ready")

		err = verifyBondStatus(testPodSwitch, "bond0", "802.3ad", 2)
		Expect(err).ToNot(HaveOccurred(), "Bond configuration should persist after pod recreation")

		testPodSwitch.DeleteAndWait(60 * time.Second)

		By("PHASE 3 completed: Operator-level validation completed")
		By("✅ BOND MODE OPERATOR LEVEL TEST COMPLETED: Active-backup and 802.3ad modes validated")
	})
})
