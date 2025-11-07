# SR-IOV Test Suite - Cursor Context

## Project Overview
**Project**: eco-gotests SR-IOV Testing Framework  
**Path**: `/root/eco-gotests/tests/sriov/`  
**Framework**: Ginkgo v2 + eco-goinfra  
**Environment**: OpenShift/Kubernetes with SR-IOV networking  
**Primary Goal**: Test SR-IOV VF configurations and networking scenarios  

---

## Core Technologies

### Testing Framework
- **Ginkgo v2**: BDD-style Go testing framework
- **Gomega**: Assertion library for Ginkgo
- **GinkgoLogr**: Structured logging in tests

### Kubernetes/OpenShift APIs
- **eco-goinfra**: High-level Go API for cluster resources
- **OpenShift API**: OpenShift-specific resources (SriovNetwork, SriovNetworkNodePolicy)
- **controller-runtime**: Dynamic client for Kubernetes resources

### SR-IOV Specific
- **SR-IOV Operator**: Manages SR-IOV device configuration
- **SriovNetworkNodePolicy**: Defines VF configuration per node
- **SriovNetwork**: Network attachment definition for VFs
- **NetworkAttachmentDefinition (NAD)**: CNI plugin configuration

---

## Required Imports for SR-IOV Tests

### Essential Imports
```go
import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/rh-ecosystem-edge/eco-goinfra/pkg/clients"
    "github.com/rh-ecosystem-edge/eco-goinfra/pkg/sriov"
    "github.com/rh-ecosystem-edge/eco-goinfra/pkg/namespace"
    "github.com/rh-ecosystem-edge/eco-goinfra/pkg/pod"
    "github.com/rh-ecosystem-edge/eco-goinfra/pkg/nodes"
    "github.com/rh-ecosystem-edge/eco-goinfra/pkg/nad"
    corev1 "k8s.io/api/core/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"
)
```

---

## Code Patterns & Best Practices

### 1. Test Structure
```go
It("Author:ID-TestName [Disruptive] [Serial]", func() {
    // Loop through test data (devices)
    for _, device := range testData {
        device := device  // Capture for goroutine
        
        func() {
            // Create resources
            defer func() {
                // Cleanup resources
            }()
            
            // Test logic
        }()
    }
})
```

### 2. Logging Pattern
```go
GinkgoLogr.Info("operation description", "key1", value1, "key2", value2)
```

### 3. Error Handling
- Always check for nil before accessing fields
- Use `Expect(err).NotTo(HaveOccurred())` for assertions
- Provide context in error messages

### 4. Resource Cleanup
- Use `defer` for cleanup operations
- Always delete resources: namespaces, networks, pods
- Increase timeouts for SR-IOV cleanup (120s for namespaces, 60s for pods)

### 5. YAML Templates
- Always quote string fields: `spoofChk: "{{ .spoolchk }}"` not `spoofChk: {{ .spoolchk }}`
- Use proper templating syntax for conditionals: `{{- if .field }}`
- Verify YAML syntax with YAMLLint

---

## Key Functions & Helpers

### Initialization
- `initVF()` - Initialize SR-IOV VFs on a device
- `initDpdkVF()` - Initialize DPDK-specific VF configuration
- `verifyWorkerNodesReady()` - Check node stability before tests

### Verification
- `verifyVFResourcesAvailable()` - Check if VF resources are advertised
- `verifyInterfaceReady()` - Verify pod network interface is UP
- `checkInterfaceCarrier()` - Check for NO-CARRIER status
- `IsSriovDeployed()` - Verify SR-IOV operator is running
- `WaitForSriovAndMCPStable()` - Poll for cluster stability

### Cleanup
- `rmSriovPolicy()` - Delete SR-IOV policy
- `rmSriovNetwork()` - Delete SR-IOV network
- `cleanupLeftoverResources()` - Pre-test cleanup for failed runs
- `CleanAllNetworksByTargetNamespace()` - Clean namespace-specific networks

### Diagnostics
- `collectSriovClusterDiagnostics()` - Gather cluster info on failure
- `collectPodDiagnostics()` - Gather pod-specific info on failure
- `logOcCommand()` - Log equivalent `oc` commands for debugging

---

## Common Pitfalls & Solutions

### ❌ Pitfall 1: Node Name is Empty
**Problem**: Pod scheduled but `pod.Definition.Spec.NodeName` is empty  
**Solution**: Call `pod.Pull()` after `WaitUntilReady()` to refresh object

### ❌ Pitfall 2: YAML Parser Rejects Values
**Problem**: YAML 1.1 parses `on`/`off` as booleans  
**Solution**: Quote all string values: `"on"` not `on`

### ❌ Pitfall 3: Premature Success
**Problem**: Test passes but operator still rolling out config  
**Solution**: Check `SriovNetworkNodeState.Status.SyncStatus == "Succeeded"`

### ❌ Pitfall 4: Resource Accumulation
**Problem**: Previous test runs leave namespaces/networks  
**Solution**: Call `cleanupLeftoverResources()` in `BeforeSuite`

### ❌ Pitfall 5: Pod Inaccessible During Cleanup
**Problem**: Pod termination causes "use of closed network connection"  
**Solution**: Gracefully handle network errors in helper functions

### ❌ Pitfall 6: Vendor Inconsistency
**Problem**: go.mod and vendor/modules.txt have different versions  
**Solution**: Keep versions synchronized, use `go mod vendor` when needed

---

## Test Data Structure

```go
type deviceConfig struct {
    Name          string  // e.g., "cx7anl244"
    DeviceID      string  // e.g., "1021"
    Vendor        string  // e.g., "15b3"
    InterfaceName string  // e.g., "ens2f0np0"
}
```

### Available Test Devices
- `e810xxv`, `e810c`, `x710` (Intel)
- `bcm57414`, `bcm57508` (Broadcom)
- `e810back` (Backup device)

---

## Test Cases (9 Total)

1. **25959**: SR-IOV VF with spoof checking enabled
2. **70820**: SR-IOV VF with spoof checking disabled
3. **25960**: SR-IOV VF with trust disabled
4. **70821**: SR-IOV VF with trust enabled
5. **25963**: SR-IOV VF with VLAN and rate limiting
6. **25961**: SR-IOV VF with auto link state
7. **71006**: SR-IOV VF with enabled link state
8. **69646**: MTU configuration for SR-IOV
9. **69582**: DPDK SR-IOV VF functionality

---

## Environment Variables

```bash
# Device configuration
export SRIOV_DEVICES="name:deviceid:vendor:interface"
# Example: export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0"

# Multi-device example
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0,cx6dxanl244:a2d6:15b3:ens7f0np0"

# Custom container image (for multi-arch support)
export ECO_SRIOV_TEST_CONTAINER="quay.io/ocp-edge-qe/eco-gotests-network-client:v4.15.2"

# kubeconfig for cluster access
export KUBECONFIG="/path/to/kubeconfig"
```

---

## Running Tests

### Build
```bash
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0"
GOSUMDB=sum.golang.org GOTOOLCHAIN=auto go build ./tests/sriov/...
```

### Run All Tests
```bash
export SRIOV_DEVICES="cx7anl244:1021:15b3:ens2f0np0"
GOSUMDB=sum.golang.org GOTOOLCHAIN=auto go test ./tests/sriov/... -v -timeout 90m
```

### Run Single Test
```bash
GOSUMDB=sum.golang.org GOTOOLCHAIN=auto go test ./tests/sriov/... -v -timeout 60m -ginkgo.focus "25959"
```

---

## File Structure

```
tests/sriov/
├── sriov_basic_test.go       # 9 test cases
├── helpers.go                # 30+ helper functions
├── README.md                 # Documentation
└── testdata/
    ├── networking/sriov/
    │   ├── sriovnetwork-template.yaml
    │   └── sriovnetwork-whereabouts-template.yaml
    └── testpods/
        └── test-pod.yaml
```

---

## Debugging Commands

```bash
# Check operator status
oc get pods -n openshift-sriov-network-operator

# Check SR-IOV policies
oc get sriovnetworknodepolicy -n openshift-sriov-network-operator

# Check node state
oc get sriovnetworknodestate -n openshift-sriov-network-operator

# Check worker nodes
oc get nodes -l node-role.kubernetes.io/worker -o wide

# Check SR-IOV networks
oc get sriovnetwork -n openshift-sriov-network-operator

# Check NetworkAttachmentDefinitions
oc get networkattachmentdefinition -A

# Describe pod for scheduling issues
oc describe pod <pod-name> -n <namespace>

# Check pod logs
oc logs <pod-name> -n <namespace>
```

---

## Important Notes

⚠️ **[Serial]** tag: Tests must run sequentially (not in parallel)  
⚠️ **[Disruptive]** tag: Tests modify cluster state (e.g., node reboots)  
⚠️ **Timeout**: VF initialization can take 30+ minutes  
⚠️ **Node Stability**: Always verify nodes are Ready before running tests  
⚠️ **Resource Cleanup**: Always use `defer` for cleanup in tests  

---

## Quick Reference: Common Issues & Fixes

| Issue | Solution |
|-------|----------|
| Pod stuck in Pending | Check `oc describe pod`, verify VF resources available |
| NAD creation timeout | Check operator logs: `oc logs -n openshift-sriov-network-operator` |
| Node not ready | Run `verifyWorkerNodesReady()` before test |
| Vendor mismatch | Run `go mod vendor` to sync |
| YAML parsing error | Quote string values: `"{{ .value }}"` |
| Pod inaccessible | Handle network errors gracefully in cleanup |

---

**Last Updated**: November 7, 2025  
**Maintained By**: eco-gotests team  
**Related Docs**: README.md, SRIOV_TEST_FLOWCHART.md

