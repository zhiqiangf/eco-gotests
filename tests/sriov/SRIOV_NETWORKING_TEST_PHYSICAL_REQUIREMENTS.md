# SR-IOV Operator Networking Test - Physical Connectivity Requirements

**Test File:** `tests/sriov/sriov_operator_networking_test.go`  
**Focus:** IPv4, IPv6, and Dual-stack networking over SR-IOV interfaces

---

## Executive Summary

This test requires **actual SR-IOV capable hardware** with **physical network connectivity**. It cannot run on virtual machines without SR-IOV passthrough or on clusters without SR-IOV NICs.

### Quick Requirements Checklist

- ✅ **SR-IOV Capable NICs** on worker nodes (Intel or Mellanox)
- ✅ **Physical Network Interface** connected to a switch/network
- ✅ **Virtual Functions (VFs)** allocated from Physical Functions (PFs)
- ✅ **Carrier Status** must be UP (not NO-CARRIER)
- ✅ **For IPv6 tests:** IPv6 enabled on physical network (optional)

---

## Physical Hardware Requirements

### 1. **SR-IOV Capable Network Interface Cards**

The test requires worker nodes to have SR-IOV capable NICs:

#### Supported Hardware:
```
Intel:
  • Intel 82599ES 10-Gigabit Ethernet Controller (x520)
  • Intel I350 Gigabit Network Connection (i350)
  • Intel 82576 Gigabit Network Connection (i210/i211)
  • Intel X520 series

Mellanox:
  • ConnectX-3 and later
  • ConnectX-4 and later
  • ConnectX-5 and later
  • ConnectX-6 and later

Other:
  • Any NIC with VT-d (Intel) or IOMMU (AMD) support
```

#### How Test Detects This:
```go
// From helpers.go - initVF() function
// The test calls:
result := initVF(
    data.Name,              // e.g., "intelnics"
    data.DeviceID,          // e.g., "1583"
    data.InterfaceName,     // e.g., "ens1f0"
    data.Vendor,            // e.g., "8086"
    sriovOpNs,
    vfNum,                  // number of VFs to create
    workerNodes
)

// If initVF returns false, test skips:
if !executed {
    Skip("No SR-IOV devices available for IPv4 networking testing")
}
```

### 2. **Physical Network Interface Connectivity**

The SR-IOV PF (Physical Function) must be connected to a physical network:

#### Physical Connection Setup:
```
Worker Node
  │
  ├─ NIC Slot 1 (SR-IOV Capable PF)
  │  │
  │  ├─ Ethernet Cable to Network Switch
  │  │
  │  ├─ PF (ens1f0) - Physical Function
  │  └─ VF 0-15 (Virtual Functions)
  │
  └─ NIC Slot 2 (Standard NIC)
     └─ Ethernet Cable to Network Switch
```

#### Carrier Status Check:
```go
// Test checks if link is UP (has carrier):
clientCarrier, err := checkInterfaceCarrier(clientPodWhereabouts, "net1")
Expect(err).ToNot(HaveOccurred(), "Failed to check interface carrier status")

if !clientCarrier {
    GinkgoLogr.Info("Interface has NO-CARRIER status, skipping connectivity test")
    // Test CONTINUES but skips ping tests
}
```

**What this means:**
- If physical cable is unplugged: `NO-CARRIER` → connectivity tests skipped
- If physical cable is connected: `CARRIER UP` → connectivity tests run

### 3. **Virtual Function Allocation**

The test allocates Virtual Functions from the Physical Function:

```
Physical Function (PF): ens1f0
  │
  ├─ Virtual Function (VF) 0 → Pod 1
  ├─ Virtual Function (VF) 1 → Pod 2
  ├─ Virtual Function (VF) 2 → (available)
  └─ ...
```

#### VF Allocation Code:
```go
vfNum := getVFNum()  // Gets number of VFs to create

result := initVF(
    data.Name,
    data.DeviceID,
    data.InterfaceName,
    data.Vendor,
    sriovOpNs,
    vfNum,            // ← How many VFs to create
    workerNodes
)
```

**Number of VFs Needed:**
- **Per test scenario:** 2 VFs minimum (client pod + server pod)
- **Total for test:** ~4-6 VFs (varies by IPAM method)
- **Per node:** Depends on hardware (typically 32-64 VFs supported)

---

## Network Architecture for the Test

### Data Flow During Test

```
┌─────────────────────────────────────────────────────────────┐
│ EXTERNAL NETWORK (Physical)                                 │
│ 192.168.100.0/24 (IPv4)                                    │
│ fd00:192:168:100::/64 (IPv6)                               │
└─────────────┬───────────────────────────────────────────────┘
              │
    ┌─────────┴─────────┐
    │ Network Switch    │
    └─────────┬─────────┘
              │
    ┌─────────┴─────────────────────────┐
    │ Worker Node Physical NIC (PF)     │
    │ ens1f0: up, RUNNING, LOWER_UP     │
    └─────────┬───────────────────────┬─┘
              │                       │
    ┌─────────┴──────┐     ┌──────────┴────────┐
    │ VF 0           │     │ VF 1             │
    │ Pod: client-wb │     │ Pod: server-wb   │
    │ eth1: net1     │     │ eth1: net1       │
    │ IP: 192.168... │────→│ IP: 192.168...   │
    │                │     │                  │
    └────────────────┘     └──────────────────┘
    
    SR-IOV Interface (net1) carries IPAM-assigned IPs
```

### Test Connectivity Paths

#### IPv4 Test:
```
Client Pod                Server Pod
(192.168.100.10)         (192.168.100.11)
  │                         │
  ├─ net1 (VF 0)           ├─ net1 (VF 1)
  │  └─ SR-IOV             │  └─ SR-IOV
  │    └─ ens1f0 (PF)      └─ ens1f0 (PF)
  │       └─ Physical Network
  └────── ping 192.168.100.11 ──────→
         (successful if carrier=UP)
```

#### IPv6 Test:
```
Client Pod                Server Pod
(fd00:192:168:100::10)   (fd00:192:168:100::11)
  │                         │
  ├─ net1 (VF 0)           ├─ net1 (VF 1)
  │  └─ SR-IOV             │  └─ SR-IOV
  │    └─ ens1f0 (PF)      └─ ens1f0 (PF)
  │       └─ Physical Network
  └────── ping6 fd00:192:168:100::11 ──────→
         (requires IPv6 on physical network)
```

---

## Specific Connectivity Requirements by Test Scenario

### Scenario 1: IPv4 with Whereabouts IPAM

**Lines 67-227**

```go
// Phase 1: Whereabouts IPv4
testNamespaceWhereabouts := "e2e-ipv4-whereabouts-..."
testNetworkWhereabouts := "ipv4-whereabouts-net-..."

// Create SR-IOV network with Whereabouts IPAM
sriovNetworkTemplate := "testdata/networking/sriov/sriovnetwork-whereabouts-template.yaml"

// Create pods
clientPodWhereabouts := createTestPod("client-wb", ns, net, "192.168.100.10/24", MAC)
serverPodWhereabouts := createTestPod("server-wb", ns, net, "192.168.100.11/24", MAC)

// Test connectivity
err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.100.11")
```

**Requirements:**
- PF carrier status: **MUST BE UP**
- IP range: **192.168.100.0/24** must be routable
- Two VFs: One for each pod
- IPAM: Whereabouts (automatic IP assignment within range)

**What Fails If:**
- ❌ Cable unplugged → `NO-CARRIER` → test skips but passes
- ❌ No VFs → test skipped at initVF stage
- ❌ NAD not created → pods stuck in ContainerCreating

---

### Scenario 2: IPv4 with Static IPAM

**Lines 157-225**

```go
// Phase 2: Static IPv4
testNetworkStatic := "ipv4-static-net-..."

// Create SR-IOV network with Static IPAM
sriovNetworkStaticTemplate := "testdata/networking/sriov/sriovnetwork-template.yaml"

// Create pods (explicit IPs)
clientPodStatic := createTestPod("client-static", ns, net, "192.168.101.10/24", MAC)
serverPodStatic := createTestPod("server-static", ns, net, "192.168.101.11/24", MAC)

// Test connectivity
err = validateWorkloadConnectivity(clientPod, serverPod, "192.168.101.11")
```

**Requirements:**
- PF carrier status: **MUST BE UP**
- IP range: **192.168.101.0/24** must be routable
- Two VFs: One for each pod
- IPAM: Static (manual IP assignment in pod annotation)

---

### Scenario 3: IPv6 with Whereabouts IPAM

**Lines 254-314**

```go
// IPv6 detection
hasIPv6 := detectIPv6Availability(apiClient)
if !hasIPv6 {
    Skip("IPv6 is not enabled on worker nodes - skipping IPv6 networking test")
}

// Phase 1: Whereabouts IPv6
testNetworkWhereabouts := "ipv6-whereabouts-net-..."

// Create pods with IPv6 addresses
clientPodWhereabouts := createTestPodIPv6("client-ipv6-wb", ns, net, 
    "fd00:192:168:100::10", MAC)
serverPodWhereabouts := createTestPodIPv6("server-ipv6-wb", ns, net, 
    "fd00:192:168:100::11", MAC)

// Test connectivity
err = verifyIPv6Connectivity(clientPod, serverPod, "fd00:192:168:100::11")
```

**Requirements:**
- IPv6 enabled on cluster nodes (detected at lines 230-235)
- IPv6 enabled on physical network
- PF carrier status: **MUST BE UP**
- IPv6 range: **fd00:192:168:100::/64** must be routable
- Two VFs: One for each pod
- **ping6** command must work in test pods

**IPv6 Detection Code:**
```go
func detectIPv6Availability(apiClient *clients.Settings) bool {
    for _, node := range workerNodes {
        for _, address := range node.Definition.Status.Addresses {
            if address.Type == corev1.NodeInternalIP {
                if strings.Contains(address.Address, ":") {
                    // IPv6 detected
                    return true
                }
            }
        }
    }
    return false  // IPv6 not detected → test skipped
}
```

---

### Scenario 4: Dual-Stack (IPv4 + IPv6)

**Lines 380-531**

```go
// Combines both IPv4 and IPv6
// Phase 1: Dual-stack Whereabouts
clientPodWhereabouts := createTestPodDualStack("client-ds-wb", ns, net,
    "192.168.200.10/24",        // IPv4
    "fd00:192:168:200::10",     // IPv6
    MAC)

// Phase 2: Dual-stack Static
clientPodStatic := createTestPodDualStack("client-ds-static", ns, net,
    "192.168.201.10/24",        // IPv4
    "fd00:192:168:201::10",     // IPv6
    MAC)

// Test both
err = verifyDualStackConnectivity(clientPod, serverPod,
    "192.168.200.11",           // IPv4 address
    "fd00:192:168:200::11")     // IPv6 address
```

**Requirements:**
- **All IPv4 requirements** (Whereabouts + Static)
- **All IPv6 requirements** (Whereabouts + Static)
- **Four VFs total** (2 for each test type)
- Physical network supports dual-stack

---

## Key Physical Connectivity Checks in Test

### 1. Carrier Status Check (Critical)

```go
// Line 138-143, 209-213, 299-303, 361-365, 450-453, 513-516
clientCarrier, err := checkInterfaceCarrier(clientPodWhereabouts, "net1")
Expect(err).ToNot(HaveOccurred(), "Failed to check interface carrier status")

if !clientCarrier {
    GinkgoLogr.Info("Interface has NO-CARRIER status, skipping connectivity test")
} else {
    // Run ping/ping6 tests
}
```

**What This Tests:**
- Is the physical link UP?
- Can the VF communicate with the physical network?
- Is the SR-IOV interface properly connected?

**If NO-CARRIER:**
- Test logs message but **does not fail**
- Connectivity tests skipped
- Test continues with next phase

### 2. Pod Creation with SR-IOV Annotation

```go
// Line 585-616
podBuilder.Definition.Annotations["k8s.v1.cni.cni.cncf.io/networks"] = networkAnnotation

// Example annotation for Whereabouts IPv4:
// [{"name": "ipv4-whereabouts-net-intelnics", 
//   "namespace": "openshift-sriov-network-operator", 
//   "ips": ["192.168.100.10/24"],
//   "mac": "20:04:0f:f1:a0:01"}]
```

**Physical Requirements:**
- SR-IOV device must be allocated to the pod
- NetworkAttachmentDefinition must exist
- Virtual Function must be available and assigned

### 3. Ping/Ping6 Connectivity Test

```go
// IPv4 connectivity
pingCmd := []string{"ping", "-c", "3", serverIP}

// IPv6 connectivity  
ping6Cmd := []string{"ping6", "-c", "3", serverIPv6}

// Wait up to 2 minutes for ping to succeed
err = wait.PollUntilContextTimeout(
    context.TODO(),
    5*time.Second,      // Retry every 5 seconds
    2*time.Minute,      // Timeout after 2 minutes
    true,
    func(ctx context.Context) (bool, error) {
        pingOutput, execErr := clientPod.ExecCommand(pingCmd)
        if execErr != nil {
            return false, nil  // Retry
        }
        return true, nil      // Success
    })
```

**Physical Requirements:**
- Pods must be able to send packets over VF
- VF must receive packets from physical network
- Server pod must respond to ping/ping6

---

## What Happens If Physical Connectivity Is Missing

| Missing Component | Test Behavior | Line | Outcome |
|---|---|---|---|
| SR-IOV NIC | initVF returns false | 73-79 | **Skip: "No SR-IOV devices available"** |
| VF allocation | initVF returns false | 73-79 | **Skip: "No SR-IOV devices available"** |
| Physical cable (NO-CARRIER) | Carrier check fails | 138, 209, 299, 361, 450, 513 | **Pass: Connectivity tests skipped** |
| IPv6 on network | detectIPv6Availability returns false | 230-235 | **Skip: "IPv6 is not enabled"** |
| NAD not created | Pod creation fails or hangs | 127-131 | **Fail: Pod stuck in ContainerCreating** |
| Pod networking | Pod cannot reach server IP | 146-147 | **Fail: ping command times out** |

---

## Minimal Physical Setup for Testing

### Minimum Configuration:

```
Cluster:
  - Master node (1x any NIC)
  - Worker node 1 (1x SR-IOV capable NIC, connected to network)
  - Worker node 2 (optional, for multi-node testing)

Network:
  - Physical switch or network
  - Subnets available:
    * 192.168.100.0/24 (IPv4 Whereabouts)
    * 192.168.101.0/24 (IPv4 Static)
    * 192.168.200.0/24 (IPv4 Dual-stack Whereabouts)
    * 192.168.201.0/24 (IPv4 Dual-stack Static)
    * fd00:192:168:100::/64 (IPv6 Whereabouts)
    * fd00:192:168:101::/64 (IPv6 Static)
    * fd00:192:168:200::/64 (IPv6 Dual-stack Whereabouts)
    * fd00:192:168:201::/64 (IPv6 Dual-stack Static)

VF Requirements:
  - Minimum 2 VFs per test phase
  - Total: 6-8 VFs across all phases
```

### Recommended Configuration:

```
Worker Node:
  - Intel X520 10GbE card (or Mellanox ConnectX-5+)
  - Supports 64 VFs
  - Connected to managed switch with:
    * IPv4 and IPv6 routing enabled
    * Port security disabled (for VF MAC addresses)
    * Allow unicast flooding

Operating System:
  - RHEL 8.4+ or equivalent
  - IOMMU enabled (Intel VT-d or AMD-Vi)
  - SR-IOV driver installed
```

---

## Test Graceful Degradation

The test is designed to gracefully degrade based on available hardware:

```
Full Test Suite Requires:
  ├─ SR-IOV hardware (hardware-dependent)
  ├─ IPv4 connectivity (network-dependent)
  ├─ IPv6 connectivity (network-dependent)
  ├─ Physical link carrier (cable-dependent)
  └─ All 8 test combinations

If Missing:
  ├─ No SR-IOV hardware → All tests skip
  ├─ No IPv6 → IPv6/Dual-stack tests skip
  ├─ No carrier → Connectivity tests skip (but pass)
  └─ One phase fails → Continue to next phase
```

---

## Summary

### Must Have:
1. ✅ SR-IOV capable NIC on worker node
2. ✅ Physical network connection to NIC
3. ✅ Virtual Functions allocated from Physical Function
4. ✅ NetworkAttachmentDefinition created by operator
5. ✅ Carrier status UP on physical interface

### Optional:
- ✅ IPv6 on physical network (for IPv6 tests)
- ✅ Multiple worker nodes (for multi-node testing)
- ✅ High-speed network (for performance testing)

### Cannot Run Without:
- ❌ No SR-IOV hardware (test skipped)
- ❌ No NAD creation (pods hang, test fails)
- ❌ No Virtual Functions (test skipped)


