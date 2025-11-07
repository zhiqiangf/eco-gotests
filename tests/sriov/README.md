# SRIOV Tests

This directory contains SRIOV tests adapted from the OpenShift tests private repository and additional test suites for SR-IOV operator testing. The tests have been modified to work with the eco-gotests framework and infrastructure.

## Test Files

- `sriov_basic_test.go` - Main test file containing the SRIOV basic test cases
- `sriov_reinstall_test.go` - SR-IOV operator reinstallation test suite
- `sriov_lifecycle_test.go` - SR-IOV component lifecycle test suite
- `sriov_operator_networking_test.go` - SR-IOV operator-level networking tests (IPv4, IPv6, dual-stack)
- `helpers.go` - Helper functions for SRIOV test operations
- `testdata/` - Template files and test data

## Test Cases

### Basic Test Suite (`sriov_basic_test.go`)

The following test cases are included:

1. **SR-IOV VF with spoof checking enabled** - Tests SRIOV VF with spoof checking enabled
2. **SR-IOV VF with spoof checking disabled** - Tests SRIOV VF with spoof checking disabled
3. **SR-IOV VF with trust disabled** - Tests SRIOV VF with trust disabled
4. **SR-IOV VF with trust enabled** - Tests SRIOV VF with trust enabled
5. **SR-IOV VF with VLAN and rate limiting configuration** - Tests SRIOV VF with VLAN and rate limiting
6. **SR-IOV VF with auto link state** - Tests SRIOV VF with auto link state
7. **SR-IOV VF with enabled link state** - Tests SRIOV VF with enabled link state
8. **MTU configuration for SR-IOV policy** - Tests SRIOV VF with custom MTU settings
9. **DPDK SR-IOV VF functionality validation** - Tests SRIOV VF with DPDK

### Reinstallation Test Suite (`sriov_reinstall_test.go`)

The reinstallation test suite validates SR-IOV operator reinstallation functionality using OLM:

1. **test_sriov_operator_control_plane_before_removal** - Validates control plane operational before removal
   - Validates operator pods are running
   - Checks CSV (ClusterServiceVersion) status is "Succeeded"
   - Verifies Subscription is active and healthy
   - Lists and validates SriovNetworkNodeState resources show "Succeeded" sync status
   - Confirms existing SriovNetworkNodePolicy CRs are present
   - Captures baseline configuration state

2. **test_sriov_operator_data_plane_before_removal** - Validates data plane operational before removal
   - Creates SR-IOV policies for test devices
   - Creates SriovNetwork CR with network-attachment-definition
   - Deploys test pods using SR-IOV network interfaces
   - Validates pods are running and have SR-IOV interfaces attached
   - Performs traffic validation between pods (ping tests)

3. **test_sriov_operator_reinstallation_functionality** - Validates functionality after reinstallation
   - **Phase 1: Operator Removal** (using OLM)
     - Deletes CSV in openshift-sriov-network-operator namespace
     - Verifies operator pods are terminated
     - Confirms CRDs remain (SriovNetwork, SriovNetworkNodePolicy, etc.)
     - Verifies running workload pods remain operational despite operator removal
   - **Phase 2: Operator Reinstallation** (using OLM)
     - Triggers CSV reinstallation via subscription update
     - Waits for new operator pods to start
     - Verifies CSV reaches "Succeeded" phase
     - Waits for operator to reconcile existing CRs
   - **Phase 3: Control Plane Validation**
     - Verifies all SriovNetworkNodeState resources sync successfully
     - Confirms existing policies are recognized and applied
     - Checks node configuration matches pre-removal state
   - **Phase 4: Data Plane Validation**
     - Verifies existing workload pods still function correctly
     - Tests traffic between existing pods
     - Creates new test pods and validates connectivity
     - Confirms new workloads can use SR-IOV networks

**Note:** All reinstallation tests are marked as `[Disruptive]` and `[Serial]` as they remove and reinstall the operator.

### Lifecycle Test Suite (`sriov_lifecycle_test.go`)

The lifecycle test suite validates SR-IOV component cleanup and resource deployment dependencies:

1. **test_sriov_components_cleanup_on_removal** - Validate complete cleanup when operator removed
   - **Phase 1: Setup and Baseline**
     - Captures baseline state of all operator components (pods, daemonsets, webhooks, operator config)
     - Creates test SR-IOV network and policy
     - Deploys test pods with SR-IOV interfaces
     - Validates initial connectivity between test workloads
   - **Phase 2: Operator Removal**
     - Deletes SriovOperatorConfig
     - Deletes CSV to trigger operator removal
     - Validates all operator components are removed:
       * Operator pods terminated (sriov-network-operator, network-resources-injector)
       * DaemonSets removed (sriov-network-config-daemon, sriov-device-plugin)
       * Webhooks removed (mutating and validating webhooks)
   - **Phase 3: Validate CRDs and Workload Survival**
     - Verifies CRDs still exist (standard OLM behavior)
     - Confirms existing workload pods continue running
     - Validates workload connectivity still works despite operator removal
   - **Phase 4: Operator Reinstallation**
     - Triggers operator reinstallation via subscription update
     - Waits for operator to reinstall and control plane to recover
     - Validates node states reconcile successfully

2. **test_sriov_resource_deployment_dependency** - Validate resources cannot deploy without operator
   - **Phase 1: Initial Setup**
     - Captures baseline state
     - Creates initial SR-IOV resources with operator running
     - Deploys test pods to validate initial deployment works
   - **Phase 2: Remove Operator**
     - Deletes CSV to remove operator
     - Waits for operator pods to terminate
   - **Phase 3: Attempt New Resource Creation**
     - Creates new SriovNetworkNodePolicy (exists in API but doesn't reconcile)
     - Creates new SriovNetwork (exists but NAD may not be created)
     - Validates resources exist but don't reconcile without operator:
       * New policy not applied to nodes
       * Node states don't update with new configuration
       * No config-daemon to apply changes
   - **Phase 4: Reinstall Operator**
     - Triggers operator reinstallation
     - Waits for operator to restart
   - **Phase 5: Validate Automatic Reconciliation**
     - Previously created resources now reconcile automatically
     - Node states update with new configuration
     - Creates new workload pods using reconciled resources
     - Validates full functionality restored

**Note:** All lifecycle tests are marked as `[Disruptive]` and `[Serial]` as they remove and reinstall the operator.

### Operator Networking Test Suite (`sriov_operator_networking_test.go`)

The operator networking test suite validates SR-IOV operator's networking capabilities across different IP address families and IPAM methods:

1. **test_sriov_operator_ipv4_functionality** - Operator-focused IPv4 networking validation
   - **Phase 1: Whereabouts IPAM**
     - Creates SR-IOV network with whereabouts IPv4 IPAM (subnet: 192.168.100.0/24)
     - Deploys client and server test pods (whereabouts auto-assigns IPs)
     - Validates SR-IOV interface attachment and IPv4 connectivity (ping test)
     - Verifies NetworkAttachmentDefinition and VF resource allocation
   - **Phase 2: Static IPAM**
     - Creates SR-IOV network with static IPv4 IPAM
     - Deploys pods with static IPv4 addresses (192.168.101.10/24, 192.168.101.11/24)
     - Validates connectivity with static IP assignment
   - **Assertions:** Both IPAM methods work correctly, ping succeeds with 0% packet loss

2. **test_sriov_operator_ipv6_functionality** - Operator-focused IPv6 networking validation
   - **Prerequisites Check:** Detects IPv6 availability on worker nodes; skips gracefully if not available
   - **Phase 1: Whereabouts IPAM (IPv6)**
     - Creates SR-IOV network with whereabouts IPv6 IPAM (ULA subnet: fd00:192:168:100::/64)
     - Deploys client and server pods with IPv6 addresses
     - Validates IPv6 connectivity using ping6
   - **Phase 2: Static IPAM (IPv6)**
     - Creates SR-IOV network with static IPv6 IPAM
     - Deploys pods with static IPv6 addresses (fd00:192:168:101::10/64, fd00:192:168:101::11/64)
     - Validates IPv6 connectivity with static assignment
   - **Assertions:** Both IPAM methods work for IPv6, ping6 succeeds

3. **test_sriov_operator_dual_stack_functionality** - Operator-focused dual-stack networking validation
   - **Prerequisites Check:** Verifies IPv6 and dual-stack support; skips gracefully if not available
   - **Phase 1: Whereabouts IPAM (Dual-Stack)**
     - Creates SR-IOV network with whereabouts dual-stack IPAM
     - IPv4 subnet: 192.168.200.0/24, IPv6 subnet: fd00:192:168:200::/64
     - Deploys pods with both IPv4 and IPv6 addresses
     - Validates both IPv4 (ping) and IPv6 (ping6) connectivity simultaneously
   - **Phase 2: Static IPAM (Dual-Stack)**
     - Creates SR-IOV network with static dual-stack IPAM
     - Deploys pods with custom dual-stack annotations containing both address families
     - Client: 192.168.201.10/24 + fd00:192:168:201::10/64
     - Server: 192.168.201.11/24 + fd00:192:168:201::11/64
     - Validates both protocols work independently and simultaneously
   - **Assertions:** Both IPAM methods support dual-stack, both IPv4 and IPv6 addresses present, both pings succeed

**Key Features:**
- Tests both **Whereabouts** and **Static** IPAM methods for each IP family
- Uses Unique Local Addresses (ULA) in `fd00::/8` range for IPv6 testing
- Gracefully skips IPv6/dual-stack tests if IPv6 is not enabled on the cluster
- Validates NO-CARRIER status and skips connectivity tests when physical link is down
- Comprehensive validation of SR-IOV interface attachment and IP address assignment

**Note:** All operator networking tests are marked as `[Disruptive]` and `[Serial]` as they create SR-IOV policies and networks that modify cluster configuration.

## Device Configuration

The tests support both environment variable configuration and default device configurations:

### Environment Variables

#### SRIOV_DEVICES
Set `SRIOV_DEVICES` environment variable with the format:
```bash
export SRIOV_DEVICES="name1:deviceid1:vendor1:interface1,name2:deviceid2:vendor2:interface2,..."
```

Example:
```bash
export SRIOV_DEVICES="e810xxv:159b:8086:ens2f0,e810c:1593:8086:ens2f2"
```

#### ECO_SRIOV_TEST_CONTAINER
Override the default test container image (useful for ARM64/multi-arch support):
```bash
export ECO_SRIOV_TEST_CONTAINER="quay.io/ocp-edge-qe/eco-gotests-network-client:v4.15.2"
```

**Note:** The default image `quay.io/openshift-kni/cnf-tests:4.16` may not support ARM64 architecture. For ARM64 clusters, use an image that supports multi-arch.

**ARM64 Image Options:**
- `quay.io/ocp-edge-qe/eco-gotests-network-client:v4.15.2` - Recommended for ARM64, multi-arch support
- `quay.io/ocp-edge-qe/eco-gotests-network-client:latest` - Latest version (may have newer ARM64 fixes)
- `quay.io/ocp-edge-qe/eco-gotests-network-client:v4.16.0` - If available, newer version

**Troubleshooting ARM64 Issues:**
If containers keep restarting on ARM64:
1. Verify image architecture compatibility:
   ```bash
   docker manifest inspect quay.io/ocp-edge-qe/eco-gotests-network-client:v4.15.2 | grep architecture
   ```
2. Check if the image has `/bin/bash` or `/bin/sh`:
   ```bash
   docker run --rm --entrypoint ls quay.io/ocp-edge-qe/eco-gotests-network-client:v4.15.2 /bin/bash /bin/sh
   ```
3. Try using a different tag or check pod logs:
   ```bash
   kubectl logs <pod-name> -n <namespace> --previous
   ```

### Default Devices
If no environment variable is set, the following default devices are used:
- e810xxv (159b:8086) - ens2f0
- e810c (1593:8086) - ens2f2
- x710 (1572:8086) - ens5f0
- bcm57414 (16d7:14e4) - ens4f1np1
- bcm57508 (1750:14e4) - ens3f0np0
- e810back (1591:8086) - ens4f2
- cx7anl244 (1021:15b3) - ens2f0np0

## Prerequisites

- SRIOV operator must be deployed and running
- Worker nodes must have SRIOV-capable network interfaces
- Test images must be available on the cluster
- Sufficient privileges to create SRIOV policies and networks

### Additional Prerequisites for IPv6/Dual-Stack Tests

The IPv6 and dual-stack networking tests have additional requirements:

**IPv6 Enabled on Cluster:**
- Worker nodes must have IPv6 addresses configured
- The test automatically detects IPv6 availability and skips gracefully if not present
- To verify IPv6 is enabled:
  ```bash
  kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.addresses[?(@.type=="InternalIP")].address}{"\n"}{end}'
  ```
  Look for addresses containing colons (`:`) which indicate IPv6

**Test Container with IPv6 Support:**
- The default test container (`quay.io/openshift-kni/cnf-tests:4.16`) includes ping6
- For ARM64 clusters or if you encounter issues, use an alternative:
  ```bash
  export ECO_SRIOV_TEST_CONTAINER="quay.io/ocp-edge-qe/eco-gotests-network-client:v4.15.2"
  ```

**Dual-Stack Configuration:**
- Both IPv4 and IPv6 must be enabled on the cluster
- Dual-stack tests validate both protocols work simultaneously
- Tests will skip gracefully if dual-stack is not available

## Test Stability and Readiness Checks

The tests include comprehensive stability checks before starting test execution:

### SR-IOV Operator Readiness
- Verifies SR-IOV operator pods are running in the operator namespace
- Confirms SR-IOV CRDs are available by checking `SriovNetwork` resources
- Validates that `SriovNetworkNodeState` objects have been created by the operator

### SR-IOV Node State Synchronization
- Waits for all worker nodes to have `SriovNetworkNodeState` objects populated by the operator
- Validates each node's `SyncStatus` is "Succeeded" (indicates operator has synced config to node)
- Retries if any node state is missing or not fully synced
- Treats empty node states as "not ready" to prevent premature test execution

### MachineConfigPool Stability
- Checks `MachineConfigPool` resources (if available in cluster schema)
- Verifies MCP `Updated=True` condition on worker pools
- Ensures MCP is not in `Degraded` or `Updating` state
- Gracefully falls back to SR-IOV node state sync if MCP check is unavailable
- Retries if MCP conditions indicate pending configuration updates

### Worker Node Readiness
- Verifies all worker nodes have `Ready` condition
- Checks for resource pressure conditions (memory, disk) that indicate instability
- Retries if any node is not ready or has resource pressure

**Default Timeouts:**
- Stability check timeout: 20 minutes (configurable)
- Polling interval: 30 seconds (configurable)
- Timeout is increased automatically if needed based on cluster load

These checks ensure tests only execute when the cluster is in a stable state and SR-IOV is fully operational.

## Running the Tests

### Basic test execution:
```bash
export GOSUMDB=sum.golang.org
export GOTOOLCHAIN=auto
go test ./tests/sriov/... -v
```

### Running only basic tests:
```bash
export GOSUMDB=sum.golang.org
export GOTOOLCHAIN=auto
go test ./tests/sriov/sriov_basic_test.go ./tests/sriov/helpers.go -v
```

### Running only reinstallation tests:
```bash
export GOSUMDB=sum.golang.org
export GOTOOLCHAIN=auto
go test ./tests/sriov/sriov_reinstall_test.go ./tests/sriov/helpers.go -v -timeout 90m
```

### Running only lifecycle tests:
```bash
export GOSUMDB=sum.golang.org
export GOTOOLCHAIN=auto
go test ./tests/sriov/sriov_lifecycle_test.go ./tests/sriov/helpers.go -v -timeout 90m
```

### Running only operator networking tests:
```bash
export GOSUMDB=sum.golang.org
export GOTOOLCHAIN=auto
go test ./tests/sriov/sriov_operator_networking_test.go ./tests/sriov/helpers.go -v -timeout 90m
```

### Running specific operator networking tests:
```bash
# Run only IPv4 networking test
go test ./tests/sriov/... -v -ginkgo.focus="ipv4_functionality"

# Run only IPv6 networking test (will skip if IPv6 not available)
go test ./tests/sriov/... -v -ginkgo.focus="ipv6_functionality"

# Run only dual-stack networking test (will skip if IPv6 not available)
go test ./tests/sriov/... -v -ginkgo.focus="dual_stack_functionality"
```

### Running specific reinstallation tests by name:
```bash
# Run only control plane validation test
go test ./tests/sriov/... -v -ginkgo.focus="control_plane_before_removal"

# Run only data plane validation test
go test ./tests/sriov/... -v -ginkgo.focus="data_plane_before_removal"

# Run full reinstallation test
go test ./tests/sriov/... -v -ginkgo.focus="reinstallation_functionality"
```

### With additional options:
```bash
export GOSUMDB=sum.golang.org
export GOTOOLCHAIN=auto
go test ./tests/sriov/... -v -ginkgo.v -timeout 60m
```

### Run specific tests by label:
```bash
export GOSUMDB=sum.golang.org
export GOTOOLCHAIN=auto

# Run all disruptive tests
go test ./tests/sriov/... -v -ginkgo.label-filter="disruptive" -timeout 90m

# Run only reinstallation tests
go test ./tests/sriov/... -v -ginkgo.label-filter="reinstall" -timeout 90m

# Run only lifecycle tests
go test ./tests/sriov/... -v -ginkgo.label-filter="lifecycle" -timeout 90m

# Run only operator networking tests
go test ./tests/sriov/... -v -ginkgo.label-filter="operator-networking" -timeout 90m

# Run basic tests only (exclude reinstall, lifecycle, and operator-networking)
go test ./tests/sriov/... -v -ginkgo.label-filter="basic" -timeout 60m
```

### Run with debugging options:
```bash
export GOSUMDB=sum.golang.org
export GOTOOLCHAIN=auto
go test ./tests/sriov/... -v -ginkgo.v -ginkgo.trace -timeout 60m
```

**Common Options:**
- `-v`: Verbose output
- `-ginkgo.v`: Ginkgo verbose output (shows detailed test progress)
- `-ginkgo.trace`: Include full stack trace when a failure occurs
- `-timeout 60m`: Sets test timeout to 60 minutes (adjust as needed)
- `-ginkgo.label-filter`: Filter tests by labels (e.g., `"Disruptive && Serial"`, `"!Serial"`)
- `-ginkgo.focus`: Run only tests matching the given regex (e.g., `-ginkgo.focus="DPDK"`)
- `-ginkgo.skip`: Skip tests matching the given regex
- `-ginkgo.keep-going`: Continue running tests even after a failure
- `-ginkgo.fail-fast`: Stop on first failure
- `-ginkgo.reportFile`: Generate test report to specified file (e.g., `-ginkgo.reportFile=test-report.json`)

**Note:** `GOTOOLCHAIN=auto` ensures Go uses the correct toolchain version as specified in `go.mod`. `GOSUMDB=sum.golang.org` enables checksum verification for module downloads.

## Test Data

The `testdata/` directory contains YAML templates for:
- SRIOV network configurations
- DPDK test pod specifications
- Network attachment definitions

## Troubleshooting Stability Checks

If tests fail waiting for stability, check the following:

### "No SR-IOV node states available yet"
- The SR-IOV operator is running but hasn't populated `SriovNetworkNodeState` objects
- **Solution:** Wait for the operator to complete initialization, or check operator logs:
  ```bash
  kubectl logs -n openshift-sriov-network-operator deployment/sriov-network-operator
  ```

### "SR-IOV node not yet synced"
- The operator has created node states but sync is still in progress
- **Solution:** Check individual node state status:
  ```bash
  kubectl get sriovnetworknodestates -n openshift-sriov-network-operator
  kubectl describe sriovnetworknodestate <node-name> -n openshift-sriov-network-operator
  ```

### "MachineConfigPool not yet updated" or "MachineConfigPool is degraded"
- Machine configuration updates are still in progress or have encountered errors
- **Solution:** Check MCP status:
  ```bash
  kubectl get mcp
  kubectl describe mcp worker
  ```

### "Worker node is not ready" or "Node has resource pressure"
- Nodes are experiencing issues or resource constraints
- **Solution:** Inspect node status:
  ```bash
  kubectl get nodes -o wide
  kubectl describe node <node-name>
  kubectl top node  # Check resource usage
  ```

### Increasing Timeout
If the cluster is healthy but needs more time, increase the stability check timeout by setting environment variables:
```bash
export SRIOV_STABILITY_TIMEOUT=1800  # 30 minutes (in seconds)
export SRIOV_STABILITY_INTERVAL=30   # Poll interval in seconds
```

## Notes

- Tests are marked as `[Disruptive]` and `[Serial]` as they modify cluster networking configuration and must run sequentially
- Some tests skip certain device types (e.g., x710, bcm devices) due to hardware limitations
- Tests clean up resources after completion
- DPDK tests require specific hardware support and may be skipped on unsupported platforms
- Comprehensive stability checks prevent test flakiness from races with operator reconciliation
