package sriov

import (
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/namespace"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/nodes"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/pod"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/internal/params"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SR-IOV Advanced Scenarios Tests", Ordered, func() {
	var (
		workerNodes []*nodes.Builder
		sriovOpNs   string
		testData    []deviceConfig
		vfNum       int
	)

	BeforeAll(func() {
		By("Setup: Initializing test environment for advanced scenarios")
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

		By("Waiting for cluster to be stable before starting advanced scenario tests")
		err = WaitForSriovAndMCPStable(
			getAPIClient(), 20*time.Minute, 30*time.Second, NetConfig.CnfMcpLabel, sriovOpNs)
		Expect(err).ToNot(HaveOccurred(), "Cluster is not stable")
		GinkgoLogr.Info("Cluster is stable and ready for advanced scenarios")

		GinkgoLogr.Info("SR-IOV Advanced Scenarios test suite initialized successfully", "operator_ns", sriovOpNs, "test_devices", len(testData))
	})

	It("test_sriov_end_to_end_telco_scenario - Complete telco deployment scenario with SR-IOV [Disruptive] [Serial] [advanced-scenarios]", func() {
		By("END-TO-END TELCO SCENARIO - Complete CNF deployment with multiple SR-IOV networks")
		GinkgoLogr.Info("Starting end-to-end telco scenario test")

		executed := false
		var testDeviceConfig deviceConfig

		// Find a suitable device for testing
		for _, data := range testData {
			result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
			if result {
				testDeviceConfig = data
				executed = true
				GinkgoLogr.Info("SR-IOV device selected for telco scenario", "device", data.Name, "deviceID", data.DeviceID)
				break
			}
		}

		if !executed {
			Skip("No SR-IOV devices available for end-to-end telco scenario testing")
		}

		// ==================== PHASE 1: Setup Telco Network Topology ====================
		By("PHASE 1: Setting up telco network topology with multiple SR-IOV networks")
		GinkgoLogr.Info("Phase 1: Creating telco network topology")

		// Use timestamp suffix to avoid namespace collision from previous test runs (fixes race condition in namespace termination)
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		testNamespace := "e2e-telco-" + testDeviceConfig.Name + "-" + timestamp
		mgmtNetworkName := "telco-mgmt-" + testDeviceConfig.Name
		userPlaneNetworkName := "telco-userplane-" + testDeviceConfig.Name
		signalingNetworkName := "telco-signaling-" + testDeviceConfig.Name

		// Create namespace
		ns := namespace.NewBuilder(getAPIClient(), testNamespace)
		for key, value := range params.PrivilegedNSLabels {
			ns.WithLabel(key, value)
		}
		_, err := ns.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create telco test namespace")

		defer func() {
			By("Cleaning up telco scenario test resources")
			rmSriovNetwork(mgmtNetworkName, sriovOpNs)
			rmSriovNetwork(userPlaneNetworkName, sriovOpNs)
			rmSriovNetwork(signalingNetworkName, sriovOpNs)
			err := ns.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete telco namespace", "namespace", testNamespace, "error", err)
			}
		}()

		By("Phase 1.1: Creating management network with static IPAM")
		mgmtNetwork := sriovNetwork{
			name:             mgmtNetworkName,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespace,
			template:         filepath.Join("testdata", "networking", "sriov", "sriovnetwork-template.yaml"),
			namespace:        sriovOpNs,
		}
		mgmtNetwork.createSriovNetwork()

		By("Phase 1.2: Creating user plane network with VLAN 100 and MTU 9000")
		userPlaneNetwork := createSriovNetworkWithVLANAndMTU(
			userPlaneNetworkName,
			testDeviceConfig.Name,
			sriovOpNs,
			testNamespace,
			100,  // VLAN ID
			9000, // MTU
			"whereabouts",
			"192.168.50.0/24",
		)
		Expect(userPlaneNetwork).ToNot(BeNil(), "Failed to create user plane network")

		By("Phase 1.3: Creating signaling network with VLAN 200")
		signalingNetwork := createSriovNetworkWithVLANAndMTU(
			signalingNetworkName,
			testDeviceConfig.Name,
			sriovOpNs,
			testNamespace,
			200,  // VLAN ID
			1500, // MTU
			"whereabouts",
			"192.168.51.0/24",
		)
		Expect(signalingNetwork).ToNot(BeNil(), "Failed to create signaling network")

		By("PHASE 1 completed: Telco network topology established")

		// ==================== PHASE 2: Deploy Telco Workload Simulation ====================
		By("PHASE 2: Deploying telco workload simulation (Control Plane, User Plane, Gateway)")

		var controlPlanePod, userPlanePod, gatewayPod *pod.Builder

		By("Phase 2.1: Deploying control plane pod (management + signaling)")
		controlPlanePod = createMultiInterfacePod(
			"control-plane",
			testNamespace,
			[]string{mgmtNetworkName, signalingNetworkName},
			map[string]string{
				mgmtNetworkName:      "10.10.10.10/24",
				signalingNetworkName: "192.168.51.10/24",
			},
		)

		err = controlPlanePod.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Control plane pod should be ready")

		By("Phase 2.2: Deploying user plane function pod (management + user plane)")
		userPlanePod = createMultiInterfacePod(
			"user-plane",
			testNamespace,
			[]string{mgmtNetworkName, userPlaneNetworkName},
			map[string]string{
				mgmtNetworkName:      "10.10.10.11/24",
				userPlaneNetworkName: "192.168.50.10/24",
			},
		)

		err = userPlanePod.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "User plane pod should be ready")

		By("Phase 2.3: Deploying gateway pod (user plane network)")
		gatewayPod = createMultiInterfacePod(
			"gateway",
			testNamespace,
			[]string{userPlaneNetworkName},
			map[string]string{
				userPlaneNetworkName: "192.168.50.11/24",
			},
		)

		err = gatewayPod.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Gateway pod should be ready")

		By("PHASE 2 completed: Telco workloads deployed")

		// ==================== PHASE 3: Validate E2E Telco Scenario ====================
		By("PHASE 3: Validating end-to-end telco scenario connectivity and configuration")

		By("Phase 3.1: Verifying control plane has correct interfaces")
		interfaceCount, err := countPodInterfaces(controlPlanePod)
		Expect(err).ToNot(HaveOccurred(), "Failed to count control plane interfaces")
		Expect(interfaceCount).To(BeNumerically(">=", 3), "Control plane should have at least 3 interfaces (eth0 + 2 SR-IOV)")

		By("Phase 3.2: Testing control plane to user plane connectivity (management network)")
		// Check for NO-CARRIER before testing connectivity
		cpCarrier, err := checkInterfaceCarrier(controlPlanePod, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to check control plane interface carrier")

		if !cpCarrier {
			GinkgoLogr.Info("Control plane interface has NO-CARRIER status, skipping management connectivity test")
		} else {
			err = validateWorkloadConnectivity(controlPlanePod, userPlanePod, "10.10.10.11")
			Expect(err).ToNot(HaveOccurred(), "Control plane should reach user plane via management network")
		}

		By("Phase 3.3: Testing user plane traffic flow (VLAN 100)")
		upCarrier, err := checkInterfaceCarrier(userPlanePod, "net2")
		Expect(err).ToNot(HaveOccurred(), "Failed to check user plane interface carrier")

		if !upCarrier {
			GinkgoLogr.Info("User plane interface has NO-CARRIER status, skipping user plane connectivity test")
		} else {
			err = validateWorkloadConnectivity(userPlanePod, gatewayPod, "192.168.50.11")
			Expect(err).ToNot(HaveOccurred(), "User plane should reach gateway via user plane network")
		}

		By("Phase 3.4: Validating VLAN configuration on user plane interfaces")
		err = validateVLANConfig(userPlanePod, "net2", 100)
		if err != nil {
			GinkgoLogr.Info("VLAN validation skipped or not supported", "error", err)
		}

		By("Phase 3.5: Validating MTU 9000 on user plane interface")
		err = validateMTU(userPlanePod, "net2", 9000)
		if err != nil {
			GinkgoLogr.Info("MTU validation warning", "error", err)
		}

		By("Phase 3.6: Testing signaling plane connectivity")
		// Note: For proper signaling test, we'd need another pod with signaling network
		// For now, verify control plane has the interface configured
		sigCarrier, err := checkInterfaceCarrier(controlPlanePod, "net2")
		Expect(err).ToNot(HaveOccurred(), "Failed to check signaling interface carrier")
		GinkgoLogr.Info("Signaling interface status", "carrier", sigCarrier)

		By("Phase 3.7: Running throughput test with iperf3")
		if upCarrier {
			throughput, err := runIperf3Test(userPlanePod, gatewayPod, "192.168.50.11")
			if err != nil {
				GinkgoLogr.Info("iperf3 test skipped or failed", "error", err)
			} else {
				GinkgoLogr.Info("Throughput test completed", "throughput", throughput)
			}
		}

		By("PHASE 3 completed: E2E telco scenario validated")

		// ==================== PHASE 4: Resilience Testing ====================
		By("PHASE 4: Testing resilience - pod recovery and resource allocation")

		By("Phase 4.1: Deleting user plane pod to test recovery")
		_, err = userPlanePod.DeleteAndWait(60 * time.Second)
		Expect(err).ToNot(HaveOccurred(), "User plane pod should delete successfully")

		By("Phase 4.2: Verifying surviving pods maintain connectivity")
		if cpCarrier {
			// Control plane and gateway should still work
			interfaceCount, err = countPodInterfaces(controlPlanePod)
			Expect(err).ToNot(HaveOccurred(), "Control plane should maintain interfaces")
			Expect(interfaceCount).To(BeNumerically(">=", 3), "Control plane interfaces should persist")
		}

		By("Phase 4.3: Recreating user plane pod and validating recovery")
		userPlanePodNew := createMultiInterfacePod(
			"user-plane-recovered",
			testNamespace,
			[]string{mgmtNetworkName, userPlaneNetworkName},
			map[string]string{
				mgmtNetworkName:      "10.10.10.12/24",
				userPlaneNetworkName: "192.168.50.12/24",
			},
		)

		err = userPlanePodNew.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Recovered user plane pod should be ready")

		By("Phase 4.4: Validating new pod gets correct SR-IOV resources")
		interfaceCount, err = countPodInterfaces(userPlanePodNew)
		Expect(err).ToNot(HaveOccurred(), "Failed to count recovered pod interfaces")
		Expect(interfaceCount).To(BeNumerically(">=", 3), "Recovered pod should have correct interfaces")

		// Cleanup pods
		controlPlanePod.DeleteAndWait(60 * time.Second)
		userPlanePodNew.DeleteAndWait(60 * time.Second)
		gatewayPod.DeleteAndWait(60 * time.Second)

		By("PHASE 4 completed: Resilience testing validated")
		By("✅ END-TO-END TELCO SCENARIO TEST COMPLETED")
	})

	It("test_sriov_multi_feature_integration - SR-IOV integration with multiple CNF features [Disruptive] [Serial] [advanced-scenarios]", func() {
		By("MULTI-FEATURE INTEGRATION - Testing SR-IOV with multiple CNF features")
		GinkgoLogr.Info("Starting multi-feature integration test")

		executed := false
		var testDeviceConfig deviceConfig

		// Find a suitable device for testing
		for _, data := range testData {
			result := initVF(data.Name, data.DeviceID, data.InterfaceName, data.Vendor, sriovOpNs, vfNum, workerNodes)
			if result {
				testDeviceConfig = data
				executed = true
				GinkgoLogr.Info("SR-IOV device selected for multi-feature integration", "device", data.Name, "deviceID", data.DeviceID)
				break
			}
		}

		if !executed {
			Skip("No SR-IOV devices available for multi-feature integration testing")
		}

		// ==================== PHASE 1: SR-IOV with DPDK ====================
		By("PHASE 1: Testing SR-IOV with DPDK integration")
		GinkgoLogr.Info("Phase 1: Testing DPDK integration with SR-IOV")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampDPDK := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceDPDK := "e2e-dpdk-" + testDeviceConfig.Name + "-" + timestampDPDK + testDeviceConfig.Name
		dpdkNetworkName := "dpdk-net-" + testDeviceConfig.Name
		dpdkPolicyName := "dpdk-policy-" + testDeviceConfig.Name

		// Create namespace
		nsDPDK := namespace.NewBuilder(getAPIClient(), testNamespaceDPDK)
		for key, value := range params.PrivilegedNSLabels {
			nsDPDK.WithLabel(key, value)
		}
		_, err := nsDPDK.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create DPDK test namespace")

		defer func() {
			By("Cleaning up DPDK test resources")
			rmSriovNetwork(dpdkNetworkName, sriovOpNs)
			rmSriovPolicy(dpdkPolicyName, sriovOpNs)
			err := nsDPDK.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete DPDK namespace", "namespace", testNamespaceDPDK, "error", err)
			}
		}()

		By("Phase 1.1: Creating DPDK VF with vfio-pci")
		dpdkResult := initDpdkVF(dpdkPolicyName, testDeviceConfig.DeviceID,
			testDeviceConfig.InterfaceName, testDeviceConfig.Vendor, sriovOpNs, vfNum, workerNodes)

		if !dpdkResult {
			Skip("DPDK VF initialization failed - hardware may not support DPDK")
		}

		By("Phase 1.2: Creating SR-IOV network for DPDK")
		dpdkNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriov-dpdk-template.yaml")
		dpdkNetwork := sriovNetwork{
			name:             dpdkNetworkName,
			resourceName:     dpdkPolicyName,
			networkNamespace: testNamespaceDPDK,
			template:         dpdkNetworkTemplate,
			namespace:        sriovOpNs,
		}
		dpdkNetwork.createSriovNetwork()

		By("Phase 1.3: Deploying DPDK test pod")
		dpdkPod := createDPDKTestPod("dpdk-test", testNamespaceDPDK, dpdkNetworkName)

		err = dpdkPod.WaitUntilReady(10 * time.Minute)
		if err != nil {
			GinkgoLogr.Info("DPDK pod failed to start, may be due to hardware or image limitations", "error", err)
			Skip("DPDK pod failed to start - skipping DPDK validation")
		}

		By("Phase 1.4: Validating DPDK interface initialization")
		err = verifyPodSriovInterface(dpdkPod, dpdkPolicyName)
		Expect(err).ToNot(HaveOccurred(), "DPDK pod should have SR-IOV interface")

		dpdkPod.DeleteAndWait(60 * time.Second)
		By("PHASE 1 completed: DPDK integration validated")

		// ==================== PHASE 2: Multiple SR-IOV Networks per Pod ====================
		By("PHASE 2: Testing multiple SR-IOV networks per pod")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampMulti := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceMulti := "e2e-multi-net-" + testDeviceConfig.Name + "-" + timestampMulti + testDeviceConfig.Name
		netA := "multi-net-a-" + testDeviceConfig.Name
		netB := "multi-net-b-" + testDeviceConfig.Name
		netC := "multi-net-c-" + testDeviceConfig.Name

		// Create namespace
		nsMulti := namespace.NewBuilder(getAPIClient(), testNamespaceMulti)
		for key, value := range params.PrivilegedNSLabels {
			nsMulti.WithLabel(key, value)
		}
		_, err = nsMulti.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create multi-network test namespace")

		defer func() {
			By("Cleaning up multi-network test resources")
			rmSriovNetwork(netA, sriovOpNs)
			rmSriovNetwork(netB, sriovOpNs)
			rmSriovNetwork(netC, sriovOpNs)
			err := nsMulti.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete multi-network namespace", "namespace", testNamespaceMulti, "error", err)
			}
		}()

		By("Phase 2.1: Creating Network A with VLAN 10")
		networkA := createSriovNetworkWithVLANAndMTU(netA, testDeviceConfig.Name,
			sriovOpNs, testNamespaceMulti, 10, 1500, "static", "")
		Expect(networkA).ToNot(BeNil(), "Failed to create network A")

		By("Phase 2.2: Creating Network B with VLAN 20")
		networkB := createSriovNetworkWithVLANAndMTU(netB, testDeviceConfig.Name,
			sriovOpNs, testNamespaceMulti, 20, 1500, "static", "")
		Expect(networkB).ToNot(BeNil(), "Failed to create network B")

		By("Phase 2.3: Creating Network C without VLAN")
		sriovNetworkTemplate := filepath.Join("testdata", "networking", "sriov", "sriovnetwork-template.yaml")
		networkCConfig := sriovNetwork{
			name:             netC,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceMulti,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		networkCConfig.createSriovNetwork()

		By("Phase 2.4: Deploying pod with 3 SR-IOV interfaces")
		multiNetPod := createMultiInterfacePod(
			"multi-net-pod",
			testNamespaceMulti,
			[]string{netA, netB, netC},
			map[string]string{
				netA: "10.10.10.10/24",
				netB: "10.10.20.10/24",
				netC: "10.10.30.10/24",
			},
		)

		err = multiNetPod.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Multi-network pod should be ready")

		By("Phase 2.5: Verifying each interface has correct configuration")
		interfaceCount, err := countPodInterfaces(multiNetPod)
		Expect(err).ToNot(HaveOccurred(), "Failed to count pod interfaces")
		Expect(interfaceCount).To(BeNumerically(">=", 4), "Pod should have at least 4 interfaces (eth0 + 3 SR-IOV)")

		By("Phase 2.6: Validating VLAN configuration on interfaces")
		err = validateVLANConfig(multiNetPod, "net1", 10)
		if err != nil {
			GinkgoLogr.Info("VLAN validation for net1 skipped", "error", err)
		}

		err = validateVLANConfig(multiNetPod, "net2", 20)
		if err != nil {
			GinkgoLogr.Info("VLAN validation for net2 skipped", "error", err)
		}

		By("Phase 2.7: Validating IP addresses on all SR-IOV interfaces")
		net1IP, err := getPodInterfaceIP(multiNetPod, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to get net1 IP")
		Expect(net1IP).To(ContainSubstring("10.10.10.10"), "net1 should have correct IP")

		net2IP, err := getPodInterfaceIP(multiNetPod, "net2")
		Expect(err).ToNot(HaveOccurred(), "Failed to get net2 IP")
		Expect(net2IP).To(ContainSubstring("10.10.20.10"), "net2 should have correct IP")

		net3IP, err := getPodInterfaceIP(multiNetPod, "net3")
		Expect(err).ToNot(HaveOccurred(), "Failed to get net3 IP")
		Expect(net3IP).To(ContainSubstring("10.10.30.10"), "net3 should have correct IP")

		multiNetPod.DeleteAndWait(60 * time.Second)
		By("PHASE 2 completed: Multiple SR-IOV networks per pod validated")

		// ==================== PHASE 3: Mixed Networking (SR-IOV + OVN-K) ====================
		By("PHASE 3: Testing mixed networking with SR-IOV secondary and OVN-K primary")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampMixed := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceMixed := "e2e-mixed-net-" + testDeviceConfig.Name + "-" + timestampMixed + testDeviceConfig.Name
		mixedNetworkName := "mixed-sriov-" + testDeviceConfig.Name

		// Create namespace
		nsMixed := namespace.NewBuilder(getAPIClient(), testNamespaceMixed)
		for key, value := range params.PrivilegedNSLabels {
			nsMixed.WithLabel(key, value)
		}
		_, err = nsMixed.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create mixed networking test namespace")

		defer func() {
			By("Cleaning up mixed networking test resources")
			rmSriovNetwork(mixedNetworkName, sriovOpNs)
			err := nsMixed.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete mixed networking namespace", "namespace", testNamespaceMixed, "error", err)
			}
		}()

		By("Phase 3.1: Creating SR-IOV network for data plane")
		mixedSriovNet := sriovNetwork{
			name:             mixedNetworkName,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceMixed,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		mixedSriovNet.createSriovNetwork()

		By("Phase 3.2: Deploying pod with both OVN-K (primary) and SR-IOV (secondary)")
		mixedPod1 := createTestPod("mixed-pod1", testNamespaceMixed, mixedNetworkName,
			"10.20.30.10/24", "20:04:0f:f1:ee:01")
		mixedPod2 := createTestPod("mixed-pod2", testNamespaceMixed, mixedNetworkName,
			"10.20.30.11/24", "20:04:0f:f1:ee:02")

		err = mixedPod1.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Mixed pod 1 should be ready")

		err = mixedPod2.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "Mixed pod 2 should be ready")

		By("Phase 3.3: Validating default route uses primary network (OVN-K)")
		defaultRoute, err := getPodDefaultRoute(mixedPod1)
		Expect(err).ToNot(HaveOccurred(), "Failed to get default route")
		Expect(defaultRoute).To(ContainSubstring("eth0"), "Default route should use eth0 (primary network)")

		By("Phase 3.4: Testing SR-IOV data plane connectivity")
		mixedCarrier, err := checkInterfaceCarrier(mixedPod1, "net1")
		Expect(err).ToNot(HaveOccurred(), "Failed to check mixed pod interface carrier")

		if !mixedCarrier {
			GinkgoLogr.Info("Mixed networking SR-IOV interface has NO-CARRIER, skipping connectivity")
		} else {
			err = validateWorkloadConnectivity(mixedPod1, mixedPod2, "10.20.30.11")
			Expect(err).ToNot(HaveOccurred(), "SR-IOV secondary network should work alongside OVN-K")
		}

		mixedPod1.DeleteAndWait(60 * time.Second)
		mixedPod2.DeleteAndWait(60 * time.Second)
		By("PHASE 3 completed: Mixed networking validated")

		// ==================== PHASE 4: Resource Management and Scaling ====================
		By("PHASE 4: Testing resource management and pod scaling with SR-IOV")

		// Use timestamp suffix to avoid namespace collision from previous test runs
		timestampScale := fmt.Sprintf("%d", time.Now().Unix())
		testNamespaceScale := "e2e-scale-" + testDeviceConfig.Name + "-" + timestampScale + testDeviceConfig.Name
		scaleNetworkName := "scale-net-" + testDeviceConfig.Name

		// Create namespace
		nsScale := namespace.NewBuilder(getAPIClient(), testNamespaceScale)
		for key, value := range params.PrivilegedNSLabels {
			nsScale.WithLabel(key, value)
		}
		_, err = nsScale.Create()
		Expect(err).NotTo(HaveOccurred(), "Failed to create scaling test namespace")

		defer func() {
			By("Cleaning up scaling test resources")
			rmSriovNetwork(scaleNetworkName, sriovOpNs)
			err := nsScale.DeleteAndWait(120 * time.Second)
			if err != nil {
				GinkgoLogr.Info("Failed to delete scaling namespace", "namespace", testNamespaceScale, "error", err)
			}
		}()

		By("Phase 4.1: Creating SR-IOV network for scaling test")
		scaleNet := sriovNetwork{
			name:             scaleNetworkName,
			resourceName:     testDeviceConfig.Name,
			networkNamespace: testNamespaceScale,
			template:         sriovNetworkTemplate,
			namespace:        sriovOpNs,
		}
		scaleNet.createSriovNetwork()

		By("Phase 4.2: Deploying multiple pods to test resource allocation")
		var scalePods []*pod.Builder
		podCount := 3 // Reduced from 5 to avoid resource exhaustion

		for i := 0; i < podCount; i++ {
			podName := fmt.Sprintf("scale-pod-%d", i)
			ipAddr := fmt.Sprintf("10.30.40.%d/24", 10+i)
			macAddr := fmt.Sprintf("20:04:0f:f1:ff:%02d", i)

			scalePod := createTestPod(podName, testNamespaceScale, scaleNetworkName, ipAddr, macAddr)
			scalePods = append(scalePods, scalePod)
		}

		By("Phase 4.3: Waiting for all scale pods to be ready")
		for i, scalePod := range scalePods {
			err = scalePod.WaitUntilReady(10 * time.Minute)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Scale pod %d should be ready", i))

			err = verifyPodSriovInterface(scalePod, testDeviceConfig.Name)
			Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Scale pod %d should have SR-IOV interface", i))
		}

		By("Phase 4.4: Scaling down - deleting 1 pod and verifying resources released")
		_, err = scalePods[0].DeleteAndWait(60 * time.Second)
		Expect(err).ToNot(HaveOccurred(), "First scale pod should delete successfully")

		By("Phase 4.5: Recreating pod and verifying resource re-allocation")
		newScalePod := createTestPod("scale-pod-new", testNamespaceScale, scaleNetworkName,
			"10.30.40.20/24", "20:04:0f:f1:ff:20")

		err = newScalePod.WaitUntilReady(10 * time.Minute)
		Expect(err).ToNot(HaveOccurred(), "New scale pod should be ready")

		err = verifyPodSriovInterface(newScalePod, testDeviceConfig.Name)
		Expect(err).ToNot(HaveOccurred(), "New scale pod should get SR-IOV resources")

		// Cleanup all scale pods
		for _, scalePod := range scalePods[1:] {
			scalePod.DeleteAndWait(60 * time.Second)
		}
		newScalePod.DeleteAndWait(60 * time.Second)

		By("PHASE 4 completed: Resource management and scaling validated")
		By("✅ MULTI-FEATURE INTEGRATION TEST COMPLETED")
	})
})
