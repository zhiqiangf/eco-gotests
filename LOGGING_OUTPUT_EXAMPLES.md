# SR-IOV Test Logging - Output Examples

This document shows examples of the logging output you'll see when running the enhanced SR-IOV tests.

## Example 1: Reinstallation Test Output

```
Running test_sriov_operator_reinstallation_functionality...

By Step: OPERATOR REINSTALLATION - Full lifecycle test including removal and restoration
[INFO] Starting operator reinstallation test namespace=openshift-sriov-network-operator

By Step: SETUP: Creating test configuration with SR-IOV workloads
[INFO] Setup phase started - capturing baseline configuration and creating test workloads

By Step: Discovering worker nodes
[INFO] Worker nodes discovered count=4

By Step: Capturing operator Subscription configuration for later restoration
[INFO] Operator Subscription captured successfully name=sriov-network-operator channel=stable source=qe-app-registry

By Step: Creating SR-IOV network for data plane test
[INFO] Creating SR-IOV network name=reinstall-test-net-cx7anl244 resourceName=cx7anl244
[INFO] Equivalent oc command command=oc get sriovnetwork reinstall-test-net-cx7anl244 -n openshift-sriov-network-operator -o yaml

By Step: PHASE 1: Removing SR-IOV operator via OLM
[INFO] CSV deletion initiated csv=sriov-network-operator.v999.0.0

By Step: Verifying operator pods are terminated
[INFO] Checking operator pod termination runningPods=0

By Step: PHASE 2: Restoring SR-IOV operator
[INFO] Restoring operator with captured configuration

By Step: Phase 2.2: Waiting for new CSV and operator pods
[INFO] Operator pods verified running count=3

By Step: PHASE 3: Validating control plane recovery
[INFO] Node states reconciled after reinstall

✅ TEST PASSED
```

## Example 2: Networking Test Output

```
Running test_sriov_operator_ipv4_functionality...

By Step: SR-IOV OPERATOR IPv4 NETWORKING - Validating operator-focused IPv4 networking functionality
[INFO] Starting IPv4 networking functionality test

By Step: Discovering worker nodes
[INFO] Worker nodes discovered count=4

By Step: PHASE 1: Testing IPv4 networking with Whereabouts IPAM
[INFO] Phase 1: Testing IPv4 with Whereabouts IPAM

By Step: Creating SR-IOV network
[INFO] SR-IOV device selected for IPv4 networking test device=cx7anl244 deviceID=1021
[INFO] Creating SR-IOV network name=ipv4-whereabouts-net-cx7anl244 resourceName=cx7anl244

By Step: Creating test pods
[INFO] Creating client and server pods namespace=e2e-ipv4-whereabouts-cx7anl244-1731338455 network=ipv4-whereabouts-net-cx7anl244
[INFO] Test pods created clientPod=client-wb serverPod=server-wb

By Step: Waiting for pod readiness
[INFO] Pod created successfully pod=client-wb namespace=e2e-ipv4-whereabouts-cx7anl244-1731338455

By Step: Testing connectivity
[INFO] Testing connectivity between pods source=client-wb dest=server-wb

✅ TEST PASSED
```

## Example 3: Advanced Scenario Output

```
Running test_sriov_end_to_end_telco_scenario...

By Step: END-TO-END TELCO SCENARIO - Complete CNF deployment with multiple SR-IOV networks
[INFO] Starting end-to-end telco scenario test

By Step: PHASE 1: Setting up telco network topology with multiple SR-IOV networks
[INFO] Phase 1: Creating telco network topology
[INFO] SR-IOV device selected for telco scenario device=cx7anl244 deviceID=1021

By Step: Phase 1.1: Creating management network with static IPAM
[INFO] Creating SR-IOV network name=telco-mgmt-cx7anl244 resourceName=cx7anl244

By Step: Phase 1.2: Creating user-plane network with Whereabouts IPAM
[INFO] Creating SR-IOV network name=telco-userplane-cx7anl244 resourceName=cx7anl244

By Step: Phase 1.3: Creating signaling network with static IPAM
[INFO] Creating SR-IOV network name=telco-signaling-cx7anl244 resourceName=cx7anl244

By Step: PHASE 2: Deploying telco workloads
[INFO] Deploying control plane pods
[INFO] Deploying user-plane pods
[INFO] Deploying signaling plane pods

By Step: PHASE 3: Testing E2E connectivity and performance
[INFO] Testing pod-to-pod connectivity across SR-IOV networks

✅ TEST PASSED
```

## Example 4: Bonding Test Output

```
Running test_sriov_bond_ipam_integration...

By Step: BONDING WITH IPAM - Testing SR-IOV bonded VFs with dynamic IP allocation
[INFO] Starting bonding with IPAM integration test

By Step: PHASE 1: Testing SR-IOV bonding with Whereabouts IPAM
[INFO] Phase 1: Creating bonded SR-IOV networks with Whereabouts IPAM
[INFO] SR-IOV device selected for bonding test device=cx7anl244 deviceID=1021

By Step: Phase 1.1: Creating first SR-IOV network (net1)
[INFO] Creating SR-IOV network name=bond-net1-wb-cx7anl244 resourceName=cx7anl244

By Step: Phase 1.2: Creating second SR-IOV network (net2)
[INFO] Creating SR-IOV network name=bond-net2-wb-cx7anl244 resourceName=cx7anl244

By Step: Phase 1.3: Creating bond network attachment definition
[INFO] Bond configuration created for name=bond-wb-cx7anl244

By Step: Creating bonded pods
[INFO] Test pods created with bonded interfaces

✅ TEST PASSED
```

## Log Format Details

### By() Marker Format
```
By Step: [CLEAR DESCRIPTION OF MAJOR TEST PHASE]

Examples:
- "OPERATOR REINSTALLATION - Full lifecycle test..."
- "PHASE 1: Removing SR-IOV operator via OLM"
- "Creating SR-IOV network"
```

### Info() Logging Format
```
[INFO] [Action/Status Message] key1=value1 key2=value2

Examples:
- "[INFO] Operator Subscription captured successfully name=sriov-network-operator channel=stable source=qe-app-registry"
- "[INFO] Worker nodes discovered count=4"
- "[INFO] Creating SR-IOV network name=test-net resourceName=cx7anl244"
```

### Equivalent OC Commands
```
[INFO] Equivalent oc command command=oc get sriovnetwork test-net -n openshift-sriov-network-operator -o yaml
```

## Key Information Categories

### 1. Initialization Logging
```
[INFO] SR-IOV operator status verified namespace=openshift-sriov-network-operator
[INFO] Worker nodes discovered count=4
[INFO] Cluster is stable and ready for tests
```

### 2. Device Selection
```
[INFO] SR-IOV device selected for testing device=cx7anl244 deviceID=1021
[INFO] VF initialization successful device=cx7anl244 vfCount=10
```

### 3. Resource Creation
```
[INFO] Test namespace created namespace=e2e-reinstall-full-cx7anl244-1731338455
[INFO] SR-IOV network created name=test-net resourceName=cx7anl244
[INFO] Test pods created clientPod=client-1 serverPod=server-1
```

### 4. Capability Detection
```
[INFO] IPv6 is available on cluster
[INFO] Dual-stack is available on cluster
[INFO] Cluster is stable and ready for advanced scenarios
```

### 5. Error Context
```
[INFO] Failed to delete test namespace namespace=test-ns error=..."
[INFO] Operator pods verified running count=3
```

### 6. Verification Commands
```
[INFO] Equivalent oc command command=oc get sriovnetwork -n openshift-sriov-network-operator
[INFO] Equivalent oc command command=oc get pods -n test-namespace -o wide
```

## Filtering Logs

### View only By() markers
```bash
ginkgo ... -v 2>&1 | grep "By Step:"
```

### View only Info logs
```bash
ginkgo ... -v 2>&1 | grep "\[INFO\]"
```

### View specific test phase
```bash
ginkgo ... -v 2>&1 | grep "PHASE 1"
```

### View device selection logs
```bash
ginkgo ... -v 2>&1 | grep "device selected"
```

## Time-Based Analysis

Using the logs, you can now:
1. Identify which phases take the longest
2. Detect bottlenecks in test execution
3. Correlate failures with specific phases
4. Understand resource allocation patterns

## Troubleshooting Examples

### Finding resource creation issues
```bash
# Look for resources that were created
ginkgo ... -v 2>&1 | grep "created"

# Look for creation failures
ginkgo ... -v 2>&1 | grep -i "failed"
```

### Tracking operator operations
```bash
# Follow operator removal and restoration
ginkgo ... -v 2>&1 | grep -E "PHASE|CSV|operator"
```

### Device-specific debugging
```bash
# See all operations for specific device
ginkgo ... -v 2>&1 | grep "cx7anl244"
```

## Next Steps

With comprehensive logging now in place, you can:
1. ✅ See exactly what each test phase does
2. ✅ Get manual verification commands
3. ✅ Track all resource operations
4. ✅ Understand failure root causes
5. ✅ Optimize test execution time
6. ✅ Debug configuration issues

---

**Note:** All logging outputs include ISO timestamps and test context automatically through the Ginkgo/logr framework integration.

