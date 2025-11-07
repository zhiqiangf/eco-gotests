# Enhanced `oc` Command Logging for SRIOV Tests

## Overview

Added comprehensive `oc` command logging throughout the SRIOV test helper functions to make troubleshooting easier when errors occur. Each test step now logs the equivalent `oc` command that can be manually executed for verification.

## Changes Made

### 1. **SRIOV Network Creation & Verification** (Lines 495-499)
Added `oc` commands to verify network and policy creation:
```bash
oc get sriovnetwork <network-name> -n openshift-sriov-network-operator -o yaml
oc describe sriovnetwork <network-name> -n openshift-sriov-network-operator
oc get sriovnetworknodepolicy <resource-name> -n openshift-sriov-network-operator -o yaml
oc describe sriovnetworknodepolicy <resource-name> -n openshift-sriov-network-operator
```

### 2. **NetworkAttachmentDefinition (NAD) Waiting** (Lines 518-522)
Added commands to verify NAD creation:
```bash
oc get networkattachmentdefinition <nad-name> -n <target-namespace> -o yaml
oc get networkattachmentdefinition -n <target-namespace>
oc get sriovnetwork <network-name> -n openshift-sriov-network-operator -o json
oc get sriovnetworknodepolicy <resource-name> -n openshift-sriov-network-operator -o json
```

### 3. **VF Resource Verification** (Lines 537-540)
Added commands to check node resources:
```bash
oc get nodes -o wide
oc describe nodes
oc get nodes -o json | jq ".items[].status.allocatable"
```

### 4. **Client Pod Creation & Readiness** (Lines 696-699)
Added commands for client pod diagnostics:
```bash
oc get pod client -n <namespace> -o yaml
oc describe pod client -n <namespace>
oc get events -n <namespace> --field-selector involvedObject.name=client
```

### 5. **Server Pod Creation & Readiness** (Lines 716-719)
Added commands for server pod diagnostics:
```bash
oc get pod server -n <namespace> -o yaml
oc describe pod server -n <namespace>
oc get events -n <namespace> --field-selector involvedObject.name=server
```

### 6. **Connectivity Testing** (Lines 770-774)
Added commands for connectivity test verification:
```bash
oc exec client -n <namespace> -- ping -c 3 192.168.1.11
oc get pod client -n <namespace> -o wide
oc get pod server -n <namespace> -o wide
oc describe pod client -n <namespace>
oc describe pod server -n <namespace>
```

### 7. **Network Cleanup** (Lines 553-555)
Added commands for cleanup verification:
```bash
oc delete sriovnetwork <network-name> -n openshift-sriov-network-operator
oc get sriovnetwork <network-name> -n openshift-sriov-network-operator -o yaml
oc get sriovnetwork -n openshift-sriov-network-operator -o wide
```

## Benefits

### 🎯 **For Troubleshooting**
- Each test step logs the equivalent `oc` command
- Users can immediately reproduce issues manually
- No need to guess what commands to run
- Clear audit trail of what was checked

### 🔍 **For Understanding**
- Shows exactly what the test is doing at each step
- Educational - users learn `oc` commands by example
- Better documentation of test flow

### 📊 **For Debugging**
- Commands are in logs, can be copy-pasted directly
- Full YAML and JSON output available in logs
- Event logs capture problems at each stage
- Multiple angles on the same resource

## Log Output Example

When you run a test, you'll now see logs like:

```
STEP: Creating SRIOV network 70821-cx7anl244
  "Equivalent oc command" "command"="oc get sriovnetwork 70821-cx7anl244 -n openshift-sriov-network-operator -o yaml"
  
STEP: Verifying SRIOV policy exists for resource cx7anl244
  "Equivalent oc command" "command"="oc get sriovnetworknodepolicy cx7anl244 -n openshift-sriov-network-operator -o yaml"
  "Equivalent oc command" "command"="oc describe sriovnetworknodepolicy cx7anl244 -n openshift-sriov-network-operator"
  
STEP: Waiting for NetworkAttachmentDefinition 70821-cx7anl244
  "Equivalent oc command" "command"="oc get networkattachmentdefinition 70821-cx7anl244 -n e2e-70821-cx7anl244 -o yaml"
  "Equivalent oc command" "command"="oc get networkattachmentdefinition -n e2e-70821-cx7anl244"
  
STEP: Waiting for client pod to be ready
  "Equivalent oc command" "command"="oc get pod client -n e2e-70821-cx7anl244 -o yaml"
  "Equivalent oc command" "command"="oc describe pod client -n e2e-70821-cx7anl244"
  "Equivalent oc command" "command"="oc get events -n e2e-70821-cx7anl244 --field-selector involvedObject.name=client"
  
STEP: Testing connectivity between pods
  "Equivalent oc command" "command"="oc exec client -n e2e-70821-cx7anl244 -- ping -c 3 192.168.1.11"
  "Equivalent oc command" "command"="oc get pod client -n e2e-70821-cx7anl244 -o wide"
```

## Key Test Steps Now with OC Logging

### ✅ Covered Steps
1. **SRIOV Policy Creation** - Get, describe policy
2. **SRIOV Network Creation** - Get, describe network
3. **NAD Creation** - Get NADs in namespace
4. **VF Resource Check** - Get nodes, check allocatable
5. **Pod Creation** - Get, describe pods
6. **Pod Readiness** - Get events, pod status
7. **Connectivity Test** - Exec ping, get pod status
8. **Cleanup** - Delete, get final status

### 🔄 How to Use When Troubleshooting

1. **Run test and capture logs**:
   ```bash
   go test ./tests/sriov/... -v -ginkgo.v 2>&1 | tee test-logs.txt
   ```

2. **Find the failing step in logs**:
   ```bash
   grep -A 5 "FAILED\|ERROR" test-logs.txt
   ```

3. **Find the `oc` command logged just before the failure**:
   ```bash
   grep -B 2 "FAILED" test-logs.txt | grep "command"
   ```

4. **Copy and run the command**:
   ```bash
   export KUBECONFIG=/root/dev-scripts/ocp/sriov/auth/kubeconfig
   oc get sriovnetwork 70821-cx7anl244 -n openshift-sriov-network-operator -o yaml
   ```

5. **Analyze the output manually**

## Files Modified

- **`tests/sriov/helpers.go`**
  - Line 495-499: Network verification
  - Line 518-522: NAD verification
  - Line 537-540: Resource verification
  - Line 696-699: Client pod diagnostics
  - Line 716-719: Server pod diagnostics
  - Line 770-774: Connectivity testing
  - Line 553-555: Cleanup verification

## Integration with Existing Logging

The new `oc` command logging:
- ✅ Uses existing `logOcCommand()` function
- ✅ Maintains consistent format
- ✅ Works with Ginkgo logging framework
- ✅ Compatible with junit reports
- ✅ No impact on test execution time

## Verification

- ✅ Code compiles without errors
- ✅ No linting errors
- ✅ Backward compatible
- ✅ Tested with multi-card setup (CX7 + Bluefield-2)

## Future Enhancements

Could add `oc` command logging for:
- VF initialization step
- Node stability checks
- SRIOV operator status
- DaemonSet status
- MachineConfig status
- And more...


