# SR-IOV Advanced Scenarios Test - Physical Connectivity Requirements

**Test File:** `tests/sriov/sriov_advanced_scenarios_test.go`  
**Complexity Level:** ADVANCED (much higher than basic networking test)  
**Test Count:** 2 major test scenarios with multiple phases

---

## Executive Summary

The advanced scenarios test requires **significantly more physical infrastructure** than the basic networking test. It tests real-world telco deployment patterns with multiple SR-IOV networks, VLAN tagging, jumbo frames, DPDK support, and high-throughput scenarios.

### Quick Requirements Checklist

#### Test 1: End-to-End Telco Scenario
- ✅ **3 separate SR-IOV networks** (management, user plane, signaling)
- ✅ **VLAN support** on physical network (VLAN 100 and VLAN 200)
- ✅ **Multiple IP subnets routable** (10.10.10.0/24, 192.168.50.0/24, 192.168.51.0/24)
- ✅ **Jumbo frames** (MTU 9000) on user plane
- ✅ **Minimum 6 Virtual Functions** (3 pods × 2 VFs each)
- ✅ **Switch support** for VLAN tagging and jumbo frames

#### Test 2: Multi-Feature Integration
- ✅ **DPDK support** (vfio-pci driver)
- ✅ **Multiple SR-IOV networks per pod** (3 networks simultaneously)
- ✅ **Advanced VLAN support** (multiple VLANs: 10, 20)
- ✅ **OVN-Kubernetes integration** (mixed networking)
- ✅ **Minimum 8+ Virtual Functions** (scaling test)
- ✅ **iperf3 throughput testing** (requires high-speed network)

---

## Test 1: End-to-End Telco Scenario (Lines 51-287)

### Overview

This test simulates a real telco deployment with separate control plane, user plane, and gateway functions communicating over different SR-IOV networks with specific VLAN and MTU requirements.

### Network Architecture

```
Physical Network
    │
    ├─ Managed Switch (supports VLAN tagging + jumbo frames)
    │  │
    │  ├─ VLAN 100 (User Plane, MTU 9000)
    │  ├─ VLAN 200 (Signaling, MTU 1500)
    │  └─ Native/Management (MTU 1500)
    │
    └─ Worker Node
       │
       ├─ PF (Physical Function): ens1f0
       │  │
       │  ├─ VF 0 → Control Plane Pod
       │  │  ├─ net1: Management Network (10.10.10.10/24)
       │  │  └─ net2: Signaling Network (192.168.51.10/24, VLAN 200)
       │  │
       │  ├─ VF 1 → User Plane Pod
       │  │  ├─ net1: Management Network (10.10.10.11/24)
       │  │  └─ net2: User Plane Network (192.168.50.10/24, VLAN 100, MTU 9000)
       │  │
       │  └─ VF 2 → Gateway Pod
       │     └─ net1: User Plane Network (192.168.50.11/24, VLAN 100, MTU 9000)
       │
       └─ Pod Connectivity Paths:
          Control Plane ↔ User Plane (via Management: 10.10.10.0/24)
          User Plane ↔ Gateway (via User Plane: 192.168.50.0/24, VLAN 100, MTU 9000)
```

### Phase 1: Setup Telco Network Topology (Lines 69-134)

#### Phase 1.1: Management Network (Static IPAM)
```go
// Line 98-106
mgmtNetwork := sriovNetwork{
    name:             mgmtNetworkName,
    resourceName:     testDeviceConfig.Name,
    networkNamespace: testNamespace,
    template:         sriovnetwork-template.yaml,
    namespace:        sriovOpNs,
}
```

**Requirements:**
- ✅ SR-IOV VF available
- ✅ Network: 10.10.10.0/24 routable
- ✅ Static IP assignment
- ✅ MTU: 1500 (standard)

#### Phase 1.2: User Plane Network (VLAN 100, MTU 9000)
```go
// Line 108-119
userPlaneNetwork := createSriovNetworkWithVLANAndMTU(
    userPlaneNetworkName,
    testDeviceConfig.Name,
    sriovOpNs,
    testNamespace,
    100,          // ← VLAN ID
    9000,         // ← Jumbo Frame MTU
    "whereabouts",
    "192.168.50.0/24",
)
```

**Requirements:**
- ✅ **VLAN tagging support** on physical network (VLAN 100)
- ✅ **Jumbo frames (MTU 9000)** enabled on:
  - Physical NIC
  - Switch port
  - Network interface
- ✅ Network: 192.168.50.0/24 routable
- ✅ Whereabouts IPAM (automatic IP assignment)
- ✅ VF with MTU 9000 support

**Why MTU 9000 Matters:**
- Reduces packet fragmentation
- Improves throughput for high-speed data
- Typical in telco/NFV deployments
- Requires end-to-end support (NIC → Switch → Pod)

#### Phase 1.3: Signaling Network (VLAN 200, MTU 1500)
```go
// Line 121-132
signalingNetwork := createSriovNetworkWithVLANAndMTU(
    signalingNetworkName,
    testDeviceConfig.Name,
    sriovOpNs,
    testNamespace,
    200,          // ← Different VLAN ID
    1500,         // ← Standard MTU
    "whereabouts",
    "192.168.51.0/24",
)
```

**Requirements:**
- ✅ **VLAN tagging support** on physical network (VLAN 200)
- ✅ Network: 192.168.51.0/24 routable
- ✅ Whereabouts IPAM
- ✅ Standard MTU 1500

### Phase 2: Deploy Telco Workload Simulation (Lines 136-182)

#### Control Plane Pod (Lines 141-153)
```go
controlPlanePod = createMultiInterfacePod(
    "control-plane",
    testNamespace,
    []string{mgmtNetworkName, signalingNetworkName},  // ← 2 SR-IOV networks
    map[string]string{
        mgmtNetworkName:      "10.10.10.10/24",
        signalingNetworkName: "192.168.51.10/24",
    },
)
```

**Physical Requirements:**
- ✅ 2 Virtual Functions allocated
- ✅ Pod has: eth0 (primary) + net1 (mgmt) + net2 (signaling)
- ✅ Carrier status must be UP on both interfaces

#### User Plane Pod (Lines 155-167)
```go
userPlanePod = createMultiInterfacePod(
    "user-plane",
    testNamespace,
    []string{mgmtNetworkName, userPlaneNetworkName},  // ← 2 SR-IOV networks
    map[string]string{
        mgmtNetworkName:      "10.10.10.11/24",
        userPlaneNetworkName: "192.168.50.10/24",
    },
)
```

**Physical Requirements:**
- ✅ 2 Virtual Functions allocated
- ✅ User Plane VF must support MTU 9000
- ✅ VLAN 100 tag must work on this VF
- ✅ Pod has: eth0 (primary) + net1 (mgmt) + net2 (user plane)

#### Gateway Pod (Lines 169-180)
```go
gatewayPod = createMultiInterfacePod(
    "gateway",
    testNamespace,
    []string{userPlaneNetworkName},  // ← 1 SR-IOV network
    map[string]string{
        userPlaneNetworkName: "192.168.50.11/24",
    },
)
```

**Physical Requirements:**
- ✅ 1 Virtual Function allocated
- ✅ VF must support MTU 9000
- ✅ Pod has: eth0 (primary) + net1 (user plane)

### Phase 3: Validate E2E Telco Scenario (Lines 184-244)

#### Phase 3.1: Interface Count Verification (Lines 187-190)
```go
interfaceCount, err := countPodInterfaces(controlPlanePod)
Expect(interfaceCount).To(BeNumerically(">=", 3), 
    "Control plane should have at least 3 interfaces (eth0 + 2 SR-IOV)")
```

**Verification:** Pod must have eth0 + net1 + net2 = 3 interfaces minimum

#### Phase 3.2: Management Network Connectivity (Lines 192-202)
```go
cpCarrier, err := checkInterfaceCarrier(controlPlanePod, "net1")
if !cpCarrier {
    // Skip connectivity test
} else {
    err = validateWorkloadConnectivity(controlPlanePod, userPlanePod, "10.10.10.11")
    // Control plane pings User Plane via Management Network
}
```

**Test:**
- Control Plane Pod (10.10.10.10) → User Plane Pod (10.10.10.11)
- Network: 10.10.10.0/24 (Management, MTU 1500)
- Physical requirement: Carrier must be UP

#### Phase 3.3: User Plane Connectivity with VLAN 100 (Lines 204-213)
```go
upCarrier, err := checkInterfaceCarrier(userPlanePod, "net2")
if !upCarrier {
    // Skip connectivity test
} else {
    err = validateWorkloadConnectivity(userPlanePod, gatewayPod, "192.168.50.11")
    // User Plane Pod pings Gateway via User Plane Network
}
```

**Test:**
- User Plane Pod (192.168.50.10) → Gateway Pod (192.168.50.11)
- Network: 192.168.50.0/24 (VLAN 100, MTU 9000)
- Physical requirements:
  - ✅ VLAN 100 tagging working
  - ✅ MTU 9000 end-to-end
  - ✅ Carrier must be UP

#### Phase 3.4: VLAN Configuration Validation (Lines 215-219)
```go
err = validateVLANConfig(userPlanePod, "net2", 100)
if err != nil {
    GinkgoLogr.Info("VLAN validation skipped or not supported", "error", err)
}
```

**Verification:** Check that interface net2 is tagged with VLAN 100

#### Phase 3.5: MTU Validation (Lines 221-225)
```go
err = validateMTU(userPlanePod, "net2", 9000)
if err != nil {
    GinkgoLogr.Info("MTU validation warning", "error", err)
}
```

**Verification:** Check that interface net2 has MTU 9000 configured

#### Phase 3.6: Signaling Network Verification (Lines 227-232)
```go
sigCarrier, err := checkInterfaceCarrier(controlPlanePod, "net2")
GinkgoLogr.Info("Signaling interface status", "carrier", sigCarrier)
```

**Verification:** Signaling network (VLAN 200) has carrier status

#### Phase 3.7: Throughput Test with iperf3 (Lines 234-242)
```go
if upCarrier {
    throughput, err := runIperf3Test(userPlanePod, gatewayPod, "192.168.50.11")
    GinkgoLogr.Info("Throughput test completed", "throughput", throughput)
}
```

**Physical Requirements:**
- ✅ iperf3 installed in pod image
- ✅ High-speed network connection (to measure meaningful throughput)
- ✅ MTU 9000 working (for high throughput)
- ✅ Network path: User Plane → Gateway with VLAN 100

### Phase 4: Resilience Testing (Lines 246-286)

#### Pod Recovery Scenario
```go
// Delete user plane pod
_, err = userPlanePod.DeleteAndWait(60 * time.Second)

// Recreate it with different IP
userPlanePodNew := createMultiInterfacePod(...)

// Verify it gets SR-IOV resources again
```

**Physical Requirements:**
- ✅ VF must be released and re-allocated
- ✅ New pod must get MTU 9000 configuration
- ✅ VLAN 100 tagging must still work
- ✅ Network connectivity must be re-established

---

## Test 2: Multi-Feature Integration (Lines 289-612)

### Overview

This test covers advanced use cases: DPDK, multiple networks per pod, mixed networking, and scaling.

### PHASE 1: SR-IOV with DPDK Integration (Lines 307-367)

#### DPDK Requirements

```go
dpdkResult := initDpdkVF(dpdkPolicyName, testDeviceConfig.DeviceID,
    testDeviceConfig.InterfaceName, testDeviceConfig.Vendor, sriovOpNs, vfNum, workerNodes)

if !dpdkResult {
    Skip("DPDK VF initialization failed - hardware may not support DPDK")
}
```

**Physical Requirements:**
- ✅ **IOMMU enabled** (Intel VT-d or AMD-Vi)
- ✅ **vfio-pci driver support**
- ✅ VF must support DPDK
- ✅ DPDK-compatible NICs:
  - Intel 82599ES / X520
  - Mellanox ConnectX-3/4/5/6+
  - Broadcom NetXtreme

**Why DPDK Matters:**
- Bypasses kernel for faster packet processing
- Requires VF to be bound to vfio-pci driver
- Used in telco/NFV for high-performance networking
- Not all hardware supports DPDK

**DPDK Network Creation (Line 343-351):**
```go
dpdkNetwork := sriovNetwork{
    name:             dpdkNetworkName,
    resourceName:     dpdkPolicyName,  // ← Different from standard resourceName
    networkNamespace: testNamespaceDPDK,
    template:         sriov-dpdk-template.yaml,  // ← Special DPDK template
    namespace:        sriovOpNs,
}
```

**DPDK Pod Requirements:**
- ✅ Pod must have privileged access
- ✅ Pod must be DPDK-enabled image
- ✅ VF bound to vfio-pci (not standard driver)

### PHASE 2: Multiple SR-IOV Networks per Pod (Lines 369-464)

#### Three Separate Networks on One Pod

```go
// Phase 2.1-2.3: Create 3 networks
// Network A: VLAN 10, MTU 1500
// Network B: VLAN 20, MTU 1500
// Network C: No VLAN, MTU 1500

// Phase 2.4: Deploy pod with 3 SR-IOV interfaces
multiNetPod := createMultiInterfacePod(
    "multi-net-pod",
    testNamespaceMulti,
    []string{netA, netB, netC},  // ← 3 networks!
    map[string]string{
        netA: "10.10.10.10/24",
        netB: "10.10.20.10/24",
        netC: "10.10.30.10/24",
    },
)
```

**Physical Requirements:**
- ✅ **3 Virtual Functions minimum** per pod
- ✅ **Pod Interface Count:** eth0 (primary) + net1 + net2 + net3 = 4 interfaces
- ✅ **Multiple VLAN support** (VLAN 10 and VLAN 20)
- ✅ Each VF must have different VLAN tagging:
  - net1: VLAN 10
  - net2: VLAN 20
  - net3: No VLAN

**Network Architecture for Multi-Network Pod:**
```
Physical NIC (PF)
    │
    ├─ VF 0 (netA, VLAN 10) → 10.10.10.10/24
    ├─ VF 1 (netB, VLAN 20) → 10.10.20.10/24
    └─ VF 2 (netC, native) → 10.10.30.10/24
        ↓
    Single Pod with 3 interfaces
    eth0 (default route via OVN-K)
    net1 (VLAN 10)
    net2 (VLAN 20)
    net3 (native)
```

#### Verification (Lines 434-461):
```go
// Verify each interface has correct IP
net1IP, err := getPodInterfaceIP(multiNetPod, "net1")
Expect(net1IP).To(ContainSubstring("10.10.10.10"))

net2IP, err := getPodInterfaceIP(multiNetPod, "net2")
Expect(net2IP).To(ContainSubstring("10.10.20.10"))

net3IP, err := getPodInterfaceIP(multiNetPod, "net3")
Expect(net3IP).To(ContainSubstring("10.10.30.10"))
```

**Physical Requirements:**
- ✅ All 3 VFs must be successfully allocated
- ✅ All 3 networks must have correct IP assignment
- ✅ VLAN tags must be correctly applied to net1 and net2

### PHASE 3: Mixed Networking (SR-IOV + OVN-K) (Lines 466-531)

#### Architecture: Primary OVN-K + Secondary SR-IOV

```
                Physical Network
                       │
    ┌──────────────────┼──────────────────┐
    │ (OVN-K Control)  │ (SR-IOV Network) │
    │                  │                  │
Pod Primary Network    │   Secondary Network
(OVN-K via eth0)       │   (SR-IOV via net1)
    │                  │                  │
    eth0 (primary)     │     net1 (SR-IOV)
    10.0.0.0/24        │     10.20.30.0/24
    (default route)    │     (data plane)
```

```go
// Phase 3.2: Create two pods with mixed networking
mixedPod1 := createTestPod("mixed-pod1", testNamespaceMixed, mixedNetworkName,
    "10.20.30.10/24", "20:04:0f:f1:ee:01")
mixedPod2 := createTestPod("mixed-pod2", testNamespaceMixed, mixedNetworkName,
    "10.20.30.11/24", "20:04:0f:f1:ee:02")
```

**Physical Requirements:**
- ✅ OVN-K cluster networking (primary)
- ✅ SR-IOV network (secondary, for data plane)
- ✅ Pod must have: eth0 (OVN-K) + net1 (SR-IOV)
- ✅ Default route via eth0 (primary network)

#### Verification (Lines 513-526):
```go
// Phase 3.3: Verify default route uses primary
defaultRoute, err := getPodDefaultRoute(mixedPod1)
Expect(defaultRoute).To(ContainSubstring("eth0"), 
    "Default route should use eth0 (primary network)")

// Phase 3.4: Test SR-IOV secondary network
mixedCarrier, err := checkInterfaceCarrier(mixedPod1, "net1")
if !mixedCarrier {
    // Skip connectivity test
} else {
    err = validateWorkloadConnectivity(mixedPod1, mixedPod2, "10.20.30.11")
}
```

**Physical Requirements:**
- ✅ Carrier status must be UP on SR-IOV interface (net1)
- ✅ IP routing must allow SR-IOV network traffic (10.20.30.0/24)
- ✅ Network must be completely separate from OVN-K (different subnets)

### PHASE 4: Resource Management and Scaling (Lines 533-611)

#### Scaling Test: 3 Pods with SR-IOV

```go
// Phase 4.2: Deploy 3 pods to test resource allocation
var scalePods []*pod.Builder
podCount := 3

for i := 0; i < podCount; i++ {
    podName := fmt.Sprintf("scale-pod-%d", i)
    ipAddr := fmt.Sprintf("10.30.40.%d/24", 10+i)
    macAddr := fmt.Sprintf("20:04:0f:f1:ff:%02d", i)
    
    scalePod := createTestPod(podName, testNamespaceScale, scaleNetworkName, ipAddr, macAddr)
    scalePods = append(scalePods, scalePod)
}
```

**Physical Requirements:**
- ✅ **Minimum 3 Virtual Functions** available (1 per pod)
- ✅ Network: 10.30.40.0/24 routable
- ✅ Each pod gets unique IP and MAC

#### Resource Release Test (Lines 590-602):
```go
// Phase 4.4: Delete first pod (should release VF)
_, err = scalePods[0].DeleteAndWait(60 * time.Second)

// Phase 4.5: Create new pod (should reuse released VF)
newScalePod := createTestPod("scale-pod-new", testNamespaceScale, 
    scaleNetworkName, "10.30.40.20/24", "20:04:0f:f1:ff:20")

err = newScalePod.WaitUntilReady(10 * time.Minute)
```

**Physical Requirements:**
- ✅ VF must be properly released when pod is deleted
- ✅ VF must be re-allocated to new pod
- ✅ New pod must get correct IP and network configuration

---

## Physical Network Requirements Summary

### Telco Scenario (Test 1)

| Component | Required | Details |
|-----------|----------|---------|
| **Virtual Functions** | 6 minimum | 2 per pod (3 pods total) |
| **VLAN Support** | Yes | VLAN 100 (user plane), VLAN 200 (signaling) |
| **IP Subnets** | 3 subnets | 10.10.10.0/24, 192.168.50.0/24, 192.168.51.0/24 |
| **MTU 9000** | Required | User plane network must support jumbo frames |
| **Carrier Status** | UP | Physical cable must be connected |
| **Switch Config** | VLAN support | Must handle VLAN tagging + jumbo frames |

### Multi-Feature Integration (Test 2)

#### DPDK Phase
| Component | Required | Details |
|-----------|----------|---------|
| **IOMMU** | Yes | Intel VT-d or AMD-Vi enabled in BIOS |
| **vfio-pci Support** | Yes | VF must support vfio-pci driver |
| **VF Count** | 1+ | Dedicated DPDK VF |

#### Multi-Network Phase
| Component | Required | Details |
|-----------|----------|---------|
| **Virtual Functions** | 3+ | One per network |
| **VLAN Support** | Yes | VLAN 10 and VLAN 20 |
| **Network Subnets** | 3 | 10.10.10.0/24, 10.10.20.0/24, 10.10.30.0/24 |

#### Mixed Networking Phase
| Component | Required | Details |
|-----------|----------|---------|
| **OVN-K** | Yes | Primary cluster networking |
| **SR-IOV** | Yes | Secondary data plane |
| **Network Isolation** | Yes | Different subnets required |

#### Scaling Phase
| Component | Required | Details |
|-----------|----------|---------|
| **Virtual Functions** | 3+ | Minimum 3 for test, can be limited |
| **VF Release/Realloc** | Working | Must properly release/reallocate VFs |

---

## Complex Physical Connectivity Scenarios

### Scenario 1: VLAN Tagging with Multiple Networks

```
Physical NIC
    │
    ├─ VF 0 (VLAN 100, MTU 9000)
    │  │
    │  └─ Pod Interface (net1)
    │     └─ 192.168.50.10/24 (User Plane)
    │
    ├─ VF 1 (VLAN 200, MTU 1500)
    │  │
    │  └─ Pod Interface (net2)
    │     └─ 192.168.51.10/24 (Signaling)
    │
    └─ VF 2 (Native/Untagged, MTU 1500)
       │
       └─ Pod Interface (net1)
          └─ 10.10.10.10/24 (Management)
```

**Physical Network Requirements:**
- ✅ Switch must support VLAN trunking
- ✅ VLAN 100 and VLAN 200 must be configured on switch port
- ✅ Port must support both jumbo frames (9000) and standard (1500)
- ✅ MTU negotiation must work between OS, NIC, switch

### Scenario 2: iperf3 High Throughput Testing

```
User Plane Pod (192.168.50.10)
    │
    └─ net1 (SR-IOV VF, VLAN 100, MTU 9000)
       │
       └─ Physical NIC
          │
          └─ Switch (must handle jumbo frames)
             │
             └─ Physical NIC (gateway end)
                │
                └─ net1 (SR-IOV VF, VLAN 100, MTU 9000)
                   │
                   └─ Gateway Pod (192.168.50.11)
                      │
                      └─ iperf3 server/client
```

**Requirements:**
- ✅ End-to-end MTU 9000 support
- ✅ No packet loss or fragmentation
- ✅ Switch should not limit throughput
- ✅ Network should achieve reasonable throughput (>1Gbps for testing)

---

## Hardware Recommendations for Advanced Tests

### Minimum Configuration

```
Worker Node:
  ├─ NIC: Intel X520 or Mellanox ConnectX-5+
  ├─ VFs: 16+ (minimum 8 for tests)
  ├─ IOMMU: Enabled (for DPDK)
  ├─ BIOS: SR-IOV, VT-d enabled
  └─ Kernel: SR-IOV driver loaded

Physical Network:
  ├─ Switch: Managed (VLAN support)
  ├─ MTU: 9000 or higher
  ├─ VLAN Trunking: Enabled
  ├─ Subnets: 3+ configured
  └─ Connection: 10GbE+ recommended
```

### Recommended Configuration

```
Worker Node(s):
  ├─ NIC: Mellanox ConnectX-6 or Intel E810
  ├─ VFs: 32+
  ├─ IOMMU: AMD IOMMU or Intel VT-d
  ├─ Memory: 32GB+
  └─ CPU: 8+ cores

Physical Network:
  ├─ Switch: Advanced (QoS, VLAN, Jumbo Frames)
  ├─ MTU: 9000 (jumbo frames)
  ├─ Bandwidth: 25GbE+
  ├─ VLAN: 100+ VLANs supported
  └─ Quality: Enterprise-grade (low latency, no loss)
```

---

## Test Failure Scenarios

### Scenario 1: VLAN Not Configured on Switch

```
Expected: Pods communicate via VLAN 100 (user plane)
Actual: Packets dropped at switch
Result: validateWorkloadConnectivity() timeout
        "User plane should reach gateway"
Recovery: Enable VLAN 100 on switch port
```

### Scenario 2: MTU Mismatch

```
Expected: 9000 byte frames on user plane
Actual: MTU 1500 on switch port
Result: Packets fragmented/dropped
        iperf3 throughput < 100Mbps
        validateMTU() fails
Recovery: Set switch port MTU to 9000
```

### Scenario 3: Insufficient Virtual Functions

```
Expected: 3 pods × 2 VFs = 6 VFs needed
Actual: Only 4 VFs available
Result: 3rd pod creation fails
        "Pod should be ready" expectation fails
Recovery: Configure more VFs or use fewer pods
```

### Scenario 4: DPDK Not Supported

```
Expected: vfio-pci driver binding
Actual: NIC doesn't support DPDK or IOMMU disabled
Result: initDpdkVF() returns false
        Skip("DPDK VF initialization failed")
Recovery: Enable IOMMU, use DPDK-capable NIC
```

### Scenario 5: No Carrier on SR-IOV Interface

```
Expected: Physical cable connected
Actual: Cable unplugged or switch port down
Result: checkInterfaceCarrier() returns false
        Connectivity tests skipped
        Test still passes (graceful degradation)
Recovery: Connect cable, enable switch port
```

---

## Monitoring and Diagnostics

### Commands to Verify Physical Setup

```bash
# Check VF availability
ip link show | grep vf

# Check VLAN configuration in pod
cat /proc/net/vlan/config

# Check MTU in pod
ip link show net1
ip link show net2

# Check interface carrier status
ethtool ens1f0 | grep "Link detected"

# Check throughput with iperf3
iperf3 -c 192.168.50.11 -t 10

# Validate VLAN tag
tcpdump -i net1 -nn "vlan"

# Check MAC addresses
ip link show
# Should show VF with specific MAC from pod annotation
```

---

## Summary

### Telco Scenario Requires:
- ✅ Multiple SR-IOV networks (3)
- ✅ VLAN support (VLAN 100, 200)
- ✅ Jumbo frames (MTU 9000)
- ✅ Multiple IP subnets
- ✅ Minimum 6 VFs
- ✅ Advanced switch capabilities

### Multi-Feature Integration Requires:
- ✅ DPDK support (IOMMU + vfio-pci)
- ✅ Multiple networks per pod (3+)
- ✅ VLAN support (VLAN 10, 20)
- ✅ Mixed networking (OVN-K + SR-IOV)
- ✅ Resource management (VF release/realloc)
- ✅ Minimum 8+ VFs
- ✅ High-speed network for throughput testing

### Key Differences from Basic Test:
| Feature | Basic | Advanced |
|---------|-------|----------|
| VF Count | 2-6 | 8+ |
| Networks | Single | Multiple |
| VLAN | No | Yes (multiple) |
| MTU | 1500 | Up to 9000 |
| DPDK | No | Yes |
| Mixed Networking | No | Yes |
| Throughput Test | No | Yes (iperf3) |

These advanced tests are designed to validate production-grade telco deployments and require significantly more complex infrastructure than basic networking tests.


