# SR-IOV Test Suite - Comprehensive Logging Enhancement Guide

## Overview

This guide provides detailed patterns and specific locations for adding comprehensive logging to all SR-IOV test files. The patterns are based on the successful implementation in `sriov_basic_test.go`.

## Logging Patterns Reference

### Pattern 1: Step Markers with By()

**Purpose:** Clear test step markers for Ginkgo output narrative flow

**Syntax:**
```go
By("Clear description of what this step does")
// or with variables
By(fmt.Sprintf("Updating SRIOV policy %s with MTU %d", policyName, mtuValue))
```

**Benefits:**
- ✅ Clear narrative flow in test output
- ✅ Easy to identify which step failed
- ✅ Supports variable interpolation with fmt.Sprintf()

**Usage:** Wrap major test phases, resource creation/deletion, verification steps

---

### Pattern 2: Structured Logging with GinkgoLogr.Info()

**Purpose:** Detailed diagnostic information with key-value pairs

**Syntax:**
```go
GinkgoLogr.Info("message description", "key1", value1, "key2", value2)
```

**Examples:**
```go
GinkgoLogr.Info("device ID", "deviceID", data.DeviceID)
GinkgoLogr.Info("SRIOV policy updated", "name", data.Name, "mtu", mtuValue)
GinkgoLogr.Info("Failed to delete namespace", "namespace", ns1, "error", err)
GinkgoLogr.Info("Operator pods verified running", "count", len(pods.Items))
```

**Benefits:**
- ✅ Structured logging for log analysis
- ✅ Key-value pairs provide context
- ✅ Variable values captured for debugging
- ✅ Errors include context

**Usage:** Log configuration details, resource info, error handling, state verification

---

### Pattern 3: Equivalent oc Commands

**Purpose:** Provide equivalent kubectl/oc commands for manual verification

**Syntax:**
```go
GinkgoLogr.Info("Equivalent oc command", "command", fmt.Sprintf("oc <command>", args...))
```

**Examples:**
```go
GinkgoLogr.Info("Equivalent oc command", "command", 
  fmt.Sprintf("oc get namespace %s || oc create namespace %s", ns, ns))

GinkgoLogr.Info("Equivalent oc command", "command",
  fmt.Sprintf("oc patch sriovnetworknodepolicy %s -n %s --type merge -p '{\"spec\":{\"mtu\":%d}}'", 
    name, ns, mtu))

GinkgoLogr.Info("Equivalent oc command", "command",
  fmt.Sprintf("oc get sriovnetworknodepolicy %s -n %s -o yaml", name, ns))
```

**Benefits:**
- ✅ Enables manual reproduction of test steps
- ✅ Helps with troubleshooting
- ✅ Documents what the test is doing at CLI level

**Usage:** Resource creation, updates, verification, debugging

---

### Pattern 4: Error Handling Logging

**Purpose:** Log errors during cleanup/teardown gracefully

**Syntax:**
```go
GinkgoLogr.Info("action or status message", "context_key", context_value, "error", err)
```

**Example:**
```go
if err := nsBuilder.DeleteAndWait(120 * time.Second); err != nil {
    GinkgoLogr.Info("Failed to delete namespace", "namespace", ns1, "error", err)
}
```

**Benefits:**
- ✅ Tracks what went wrong during cleanup
- ✅ Doesn't fail the test but logs for analysis
- ✅ Helps identify infrastructure issues

**Usage:** Error handling in defer blocks, cleanup phases

---

## File-by-File Enhancement Guide

### File 1: sriov_reinstall_test.go

**Current Status:** ⚠️ Partial (Phase 4 has logging from restoration fixes)

**Enhancement Locations:**

#### BeforeEach Block (Lines 42-51)
**Add After Line 44 (after chkSriovOperatorStatus):**
```go
GinkgoLogr.Info("SR-IOV operator status verified", "namespace", sriovOpNs)

By("Discovering worker nodes")
```

**Add After Line 49 (after nodes.List):**
```go
GinkgoLogr.Info("Worker nodes discovered", "count", len(workerNodes))
```

#### Test: test_sriov_operator_control_plane_before_removal (Lines 53-86)
**Add at beginning (after line 54):**
```go
By("CONTROL PLANE VALIDATION - Pre-removal verification")
GinkgoLogr.Info("Starting control plane validation", "namespace", sriovOpNs)
```

#### Test: test_sriov_operator_data_plane_before_removal (Lines 88-166)
**Add at beginning (after line 88):**
```go
By("DATA PLANE VALIDATION - Pre-removal verification")
GinkgoLogr.Info("Starting data plane validation", "namespace", sriovOpNs)
```

**Add before initVF (around line 96):**
```go
By(fmt.Sprintf("Initializing VF for device %s", data.Name))
GinkgoLogr.Info("Creating SR-IOV test pod", "device", data.Name, "namespace", testNamespace)
```

**Add before createSriovNetwork (around line 237):**
```go
By(fmt.Sprintf("Creating SR-IOV network for device %s", data.Name))
GinkgoLogr.Info("Equivalent oc command", "command", 
  fmt.Sprintf("oc get sriovnetwork %s -n %s -o yaml", testNetworkName, sriovOpNs))
```

**Add before validateWorkloadConnectivity (around line 250):**
```go
By("Validating workload connectivity")
GinkgoLogr.Info("Testing connectivity between pods", "source", clientPod.Definition.Name, "dest", serverPod.Definition.Name)
```

#### Test: test_sriov_operator_reinstallation_functionality (Lines 170-404)
**Phase 1 additions (around line 258):**
```go
By("PHASE 1: Removing SR-IOV operator via OLM")
GinkgoLogr.Info("Starting operator removal test", "namespace", sriovOpNs)

// After deleteOperatorCSV (add at line 262)
GinkgoLogr.Info("CSV deletion initiated", "csv", csv.Definition.Name)

// After validateOperatorPodsRemoved (add at line 286)
GinkgoLogr.Info("All operator pods terminated successfully")
```

**Phase 2 additions:**
Already has comprehensive logging from our restoration fixes.

**Phase 3 additions (around line 336):**
```go
By("PHASE 3: Validating control plane recovery")
GinkgoLogr.Info("Starting control plane recovery validation", "namespace", sriovOpNs)

// After validateNodeStatesReconciled (add at line 340)
GinkgoLogr.Info("Node states reconciled after reinstall")

// After compareSriovState (add at line 346)
GinkgoLogr.Info("State comparison complete", "differences", len(differences))
```

---

### File 2: sriov_lifecycle_test.go

**Current Status:** ⚠️ Partial (Phase 4 has logging from restoration fixes)

**Enhancement Locations:**

#### BeforeEach Block (Lines 40-49)
**Add after chkSriovOperatorStatus (line 42):**
```go
GinkgoLogr.Info("SR-IOV operator verified", "namespace", sriovOpNs)

By("Discovering worker nodes")
```

**Add after nodes.List (line 49):**
```go
GinkgoLogr.Info("Worker nodes discovered", "count", len(workerNodes))
```

#### Test 1: test_sriov_components_cleanup_on_removal (Lines 51-227)

**Phase 1 additions (before line 60):**
```go
By("PHASE 1: Setting up test workload and capturing baseline")
GinkgoLogr.Info("Initializing test environment for component cleanup", "namespace", sriovOpNs)

// After initVF (around line 64)
if result {
    GinkgoLogr.Info("SR-IOV VF initialized", "device", testDeviceConfig.Name)
}

// After nsBuilder.Create() (around line 86)
GinkgoLogr.Info("Test namespace created", "namespace", testNamespace)
```

**Phase 2 additions (around line 147):**
```go
// After deleteOperatorConfiguration
GinkgoLogr.Info("SriovOperatorConfig deleted")

// After deleteOperatorCSV
GinkgoLogr.Info("CSV deleted to trigger operator removal")
GinkgoLogr.Info("Equivalent oc command", "command", 
  fmt.Sprintf("oc get csv -n %s", sriovOpNs))
```

**Phase 3 additions (around line 176):**
```go
By("PHASE 3: Validating CRDs remain and workloads survive")
GinkgoLogr.Info("Verifying cluster state after operator removal")
```

**Phase 4:** Already has comprehensive logging from our restoration fixes.

#### Test 2: test_sriov_resource_deployment_dependency (Lines 229-435)

**Phase 1 additions (around line 241):**
```go
By("PHASE 1: Setting up initial resources and capturing baseline")
GinkgoLogr.Info("Initializing test environment for dependency validation", "namespace", sriovOpNs)
```

**Phase 2 additions (around line 328):**
```go
// After deleteOperatorCSV
GinkgoLogr.Info("CSV deleted - operator removal in progress")
```

**Phase 3 additions (around line 341):**
```go
By("PHASE 3: Attempting to create new resources without operator")
GinkgoLogr.Info("Creating resources while operator is removed")
```

**Phase 4:** Already has comprehensive logging from our restoration fixes.

**Phase 5 additions (around line 406):**
```go
By("PHASE 5: Validating full functionality after reconciliation")
GinkgoLogr.Info("Verifying system is fully operational after operator returns")
```

---

### File 3: sriov_advanced_scenarios_test.go

**Current Status:** ❌ Minimal logging

**Enhancement Locations:**

#### BeforeAll Block (Lines 25-49)
**Add after workerNodes assignment (around line 33):**
```go
By("Discovering worker nodes")
GinkgoLogr.Info("Worker nodes discovered", "count", len(workerNodes))

By("Verifying SR-IOV operator is deployed")
```

**Add after IsSriovDeployed (around line 41):**
```go
GinkgoLogr.Info("SR-IOV operator verified", "namespace", sriovOpNs)

By("Waiting for cluster stability")
```

**Add after WaitForSriovAndMCPStable (around line 46):**
```go
GinkgoLogr.Info("Cluster is stable and ready for advanced scenarios")
```

#### Test 1: test_sriov_end_to_end_telco_scenario (Lines 51-285)

**Add at beginning:**
```go
By("E2E TELCO SCENARIO - Complete CNF deployment")
GinkgoLogr.Info("Starting end-to-end telco scenario test", "devices", len(testData))
```

**Add before each phase:**
```go
// Phase 1
By("PHASE 1: Network Function Planning and Resource Allocation")
GinkgoLogr.Info("Planning network functions", "device", testDeviceConfig.Name)

// Phase 2
By("PHASE 2: Control Plane Deployment")
GinkgoLogr.Info("Deploying control plane pods")

// Phase 3
By("PHASE 3: Data Plane Operations (E2E Traffic)")
GinkgoLogr.Info("Testing end-to-end traffic")

// Phase 4
By("PHASE 4: Testing resilience")
GinkgoLogr.Info("Testing pod recovery and resource reallocation")
```

**Add for key operations:**
```go
GinkgoLogr.Info("Equivalent oc command", "command", 
  fmt.Sprintf("oc get sriovnetwork %s -n %s -o wide", networkName, sriovOpNs))

GinkgoLogr.Info("Pod created successfully", "pod", pod.Definition.Name, "namespace", pod.Definition.Namespace)
```

#### Test 2: test_sriov_multi_feature_integration (Lines 287-590)

**Add at beginning:**
```go
By("MULTI-FEATURE INTEGRATION - Testing SR-IOV with multiple CNF features")
GinkgoLogr.Info("Starting multi-feature integration test")
```

**Add before each feature test:**
```go
// VLAN Integration
By("Testing VLAN integration")
GinkgoLogr.Info("VLAN configuration", "vlanId", vlanId)

// QoS Integration
By("Testing QoS integration")
GinkgoLogr.Info("QoS configuration", "priority", qosPriority)

// ACL Integration
By("Testing ACL integration")
GinkgoLogr.Info("ACL rules being applied")
```

---

### File 4: sriov_bonding_test.go

**Current Status:** ❌ Minimal logging

**Enhancement Locations:**

#### BeforeAll Block (Lines 24-49)
**Add similar to advanced_scenarios:**
```go
By("Discovering worker nodes")
GinkgoLogr.Info("Worker nodes discovered", "count", len(workerNodes))

By("Verifying SR-IOV operator is deployed")
// ... rest of setup
```

#### Test 1: test_sriov_bond_ipam_integration (Lines 49-292)

**Add at beginning:**
```go
By("SR-IOV BONDING WITH IPAM - Testing bonded VFs with IPAM")
GinkgoLogr.Info("Starting bond IPAM integration test")
```

**Add before bonding setup:**
```go
By("Setting up bonding configuration")
GinkgoLogr.Info("Creating bond interface", "bond_name", bondName, "slaves", len(bondSlaves))

GinkgoLogr.Info("Equivalent oc command", "command",
  fmt.Sprintf("oc get sriovnetwork %s -n %s -o yaml", networkName, sriovOpNs))
```

**Add before pod creation:**
```go
By("Creating pods with bonded SR-IOV interfaces")
GinkgoLogr.Info("Pod creation", "pod", podName, "namespace", testNamespace)
```

#### Test 2: test_sriov_bond_mode_operator_level (Lines 293-590)

**Add at beginning:**
```go
By("BONDING MODES - Testing operator-level bond configuration")
GinkgoLogr.Info("Starting bonding modes test", "testModes", len(bondModes))
```

**Add for each bond mode test:**
```go
By(fmt.Sprintf("Testing bond mode: %s", bondMode))
GinkgoLogr.Info("Bond mode configuration", "mode", bondMode, "slaves", len(bondSlaves))

GinkgoLogr.Info("Equivalent oc command", "command",
  fmt.Sprintf("oc get sriovnetworknodepolicy -n %s -o wide", sriovOpNs))
```

---

### File 5: sriov_operator_networking_test.go

**Current Status:** ❌ Minimal logging

**Enhancement Locations:**

#### BeforeAll Block (similar to bonding_test)
```go
By("Discovering worker nodes")
GinkgoLogr.Info("Worker nodes discovered", "count", len(workerNodes))

By("Verifying SR-IOV operator is deployed")
GinkgoLogr.Info("SR-IOV operator verified", "namespace", sriovOpNs)
```

#### Test 1: test_sriov_operator_ipv4_functionality (Lines 67-220)

**Add at beginning:**
```go
By("SR-IOV OPERATOR IPv4 NETWORKING - Validating operator-focused IPv4 networking")
GinkgoLogr.Info("Starting IPv4 functionality test")
```

**Add for each step:**
```go
By("Creating SR-IOV network with IPv4")
GinkgoLogr.Info("Network creation", "name", networkName, "addressPool", ipv4Pool)

GinkgoLogr.Info("Equivalent oc command", "command",
  fmt.Sprintf("oc get sriovnetwork %s -n %s -o yaml", networkName, sriovOpNs))

By("Verifying IPv4 address allocation")
GinkgoLogr.Info("IPv4 address assigned", "pod", podName, "address", ipv4Address)
```

#### Test 2: test_sriov_operator_ipv6_functionality (Lines 222-370)

**Add at beginning:**
```go
By("SR-IOV OPERATOR IPv6 NETWORKING - Validating operator-focused IPv6 networking")
GinkgoLogr.Info("Starting IPv6 functionality test")
```

**Add similar logging as IPv4 test, adjusted for IPv6:**
```go
GinkgoLogr.Info("IPv6 address assigned", "pod", podName, "address", ipv6Address)
```

#### Test 3: test_sriov_operator_dual_stack_functionality (Lines 372-520)

**Add at beginning:**
```go
By("SR-IOV OPERATOR DUAL-STACK NETWORKING - Validating operator-focused dual-stack networking")
GinkgoLogr.Info("Starting dual-stack functionality test")
```

**Add for dual-stack operations:**
```go
By("Creating dual-stack network (IPv4 + IPv6)")
GinkgoLogr.Info("Dual-stack network creation", "name", networkName, 
  "ipv4Pool", ipv4Pool, "ipv6Pool", ipv6Pool)

By("Verifying dual-stack address allocation")
GinkgoLogr.Info("Addresses assigned", "pod", podName, 
  "ipv4", ipv4Address, "ipv6", ipv6Address)
```

---

## Implementation Guidelines

### 1. Order of Implementation
- Phase 1: `sriov_reinstall_test.go` + `sriov_lifecycle_test.go`
- Phase 2: `sriov_advanced_scenarios_test.go` + `sriov_bonding_test.go`
- Phase 3: `sriov_operator_networking_test.go`

### 2. Step-by-Step Process per File

For each file:
1. Open the file
2. Identify the locations listed in the guide
3. Add the `By()` statement at phase/step beginnings
4. Add `GinkgoLogr.Info()` calls for:
   - Configuration details
   - Resource creation/deletion
   - Verification steps
   - Equivalent oc commands
   - Error handling

### 3. Formatting Rules

- Always include proper indentation (tabs, not spaces)
- Use `fmt.Sprintf()` for variable interpolation in By() statements
- Follow the existing code style
- Test after each file modification with `gofmt -w`

### 4. Verification Steps

After adding logging to each file:
1. Run `gofmt -w filename.go` to format
2. Run `grep -c "GinkgoLogr.Info\|By(" filename.go` to verify logging count
3. Check for syntax errors: `go build ./tests/sriov/...`

### 5. Commit Strategy

**Phase 1 Commit:**
```
feat(sriov): Add comprehensive logging to reinstall and lifecycle tests

- Add By() markers for all test phases
- Add GinkgoLogr.Info() for configuration tracking
- Add equivalent oc commands for troubleshooting
- Improves diagnostics and debugging capabilities
```

**Phase 2 Commit:**
```
feat(sriov): Add comprehensive logging to advanced and bonding tests

- Add structured logging throughout test execution
- Add equivalent oc commands for manual verification
- Improve visibility into test operations
```

**Phase 3 Commit:**
```
feat(sriov): Add comprehensive logging to networking tests

- Add step markers and diagnostic logging
- Add equivalent oc commands for IPv4/IPv6/dual-stack tests
- Complete logging coverage for all SR-IOV tests
```

---

## Expected Results After Implementation

✅ **Complete Visibility:** Every major test step will be clearly marked with `By()`
✅ **Structured Diagnostics:** All configuration and state changes logged with context
✅ **Manual Troubleshooting:** Equivalent oc commands for every major operation
✅ **Error Tracking:** Clear error messages with context during failures
✅ **Consistency:** Uniform logging patterns across all SR-IOV tests

---

## Quick Reference: Logging Commands

### For Resource Creation:
```go
GinkgoLogr.Info("Resource created", "name", resourceName, "namespace", namespace)
GinkgoLogr.Info("Equivalent oc command", "command", fmt.Sprintf("oc get <resource> %s -n %s", resourceName, namespace))
```

### For Verification:
```go
GinkgoLogr.Info("Verification complete", "status", statusValue, "details", detailsString)
```

### For Errors:
```go
GinkgoLogr.Info("Operation failed", "operation", opName, "error", err.Error())
```

### For Phase Markers:
```go
By(fmt.Sprintf("PHASE X: %s", phaseDescription))
GinkgoLogr.Info("Starting phase", "phase", phaseNumber, "description", phaseDescription)
```

---

## Notes

- This guide is designed for incremental implementation
- You can implement changes gradually, file by file
- Each file can be tested independently
- The logging patterns follow best practices from `sriov_basic_test.go`
- All additions are non-breaking and enhance diagnostics only

---

End of Logging Enhancement Guide

