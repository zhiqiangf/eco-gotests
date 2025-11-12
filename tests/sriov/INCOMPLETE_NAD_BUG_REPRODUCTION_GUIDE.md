# SR-IOV Operator Incomplete NAD Configuration Bug - Reproduction Guide

**Document Date**: 2025-11-12  
**Status**: READY FOR EXECUTION  
**Objective**: Reproduce the Incomplete NAD bug and collect evidence for upstream reporting

---

## Quick Start

```bash
# From the eco-gotests directory
cd tests/sriov

# Run the reproduction script
./reproduce_incomplete_nad_bug.sh

# Output will be saved to: /tmp/incomplete_nad_bug_<timestamp>/
```

---

## Prerequisites

1. **Active OpenShift Cluster**: Must be connected via `oc` CLI
2. **SR-IOV Operator**: Must be installed and running
3. **SR-IOV Capable Hardware**: At least one node with SR-IOV devices
4. **Permissions**: Must be able to create namespaces and resources
5. **Environment**: Either set `KUBECONFIG` or have valid `~/.kube/config`

### Verify Prerequisites

```bash
# Check cluster connectivity
oc cluster-info

# Check SR-IOV operator
oc get deployment -A | grep -i sriov

# Check SR-IOV devices
oc get sriovnetworkdevices -A

# Check available nodes
oc get nodes
```

---

## What the Script Does

### Phase 1: Setup
- Creates output directory for logs
- Verifies SR-IOV operator is deployed
- Creates test namespace
- Labels namespace for easy cleanup

### Phase 2: Create SriovNetwork
- Defines SriovNetwork resource
- Applies it to the cluster
- Monitors for creation

### Phase 3: Capture NAD Configuration
- **Waits up to 60 seconds** for NAD to be created
- Captures NAD in YAML format
- Captures NAD in JSON format
- **Extracts CNI config** from NAD spec
- **Analyzes** for missing `resourceName` and `pciAddress` fields
- **Confirms bug** if fields are missing

### Phase 4: Create Test Pod
- Creates a test pod with SR-IOV network annotation
- Pod will attempt to attach to the SR-IOV network
- Pod will fail because of incomplete NAD config

### Phase 5: Capture Diagnostics
- Pod status and events
- Pod failure reason and CNI error messages
- Operator logs
- Network state information

### Phase 6: Generate Report
- Creates comprehensive bug report
- Generates analysis of findings
- Archives all logs to tarball

### Phase 7: Cleanup (Optional)
- Deletes test pod
- Deletes SriovNetwork
- Deletes test namespace
- Use `--skip-cleanup` to preserve resources for investigation

---

## Output Files

The script generates the following files:

```
/tmp/incomplete_nad_bug_<timestamp>/
├── 01_cluster_info.txt              # Cluster and operator status
├── 02_nad_configuration.yaml        # NAD resource (YAML)
├── 02_nad_configuration.json        # NAD resource (JSON)
├── 02_nad_cni_config.json           # Extracted CNI config (KEY FILE)
├── 02_nad_analysis.txt              # Analysis of missing fields (KEY FILE)
├── 03_pod_status.yaml               # Test pod detailed status
├── 03_pod_events.txt                # Pod events showing CNI failure
├── 03_bug_reproduced.txt            # Confirmation: Bug successfully reproduced
├── 04_operator_logs.txt             # SR-IOV operator pod logs
├── 04_operator_logs_analysis.txt    # Analysis of operator logs
├── 05_network_state.txt             # Overall network state
├── BUG_REPORT.md                    # Comprehensive bug report (KEY FILE)
└── incomplete_nad_bug_logs_*.tar.gz # Compressed archive of all files
```

### Key Files for Bug Reporting
1. **02_nad_cni_config.json** - Shows incomplete config
2. **02_nad_analysis.txt** - Confirms missing fields
3. **03_bug_reproduced.txt** - Proves bug reproduction
4. **BUG_REPORT.md** - Complete report
5. **incomplete_nad_bug_logs_*.tar.gz** - Everything in one file

---

## Usage Examples

### Basic Run
```bash
./reproduce_incomplete_nad_bug.sh
```
Runs to completion and cleans up resources.

### Preserve Resources for Investigation
```bash
./reproduce_incomplete_nad_bug.sh --skip-cleanup
```
Leaves test resources so you can investigate manually:
```bash
# Inspect the created NAD
oc get networkattachmentdefinition -n reproduce-nad-bug-*
oc describe networkattachmentdefinition -n reproduce-nad-bug-* -l sriov-test=true

# Check the test pod
oc describe pod test-pod-sriov -n reproduce-nad-bug-*
oc get events -n reproduce-nad-bug-*
```

### Custom Output Directory
```bash
./reproduce_incomplete_nad_bug.sh --output-dir /var/logs/sriov-bug
```
Saves outputs to specified directory.

### Combine Options
```bash
./reproduce_incomplete_nad_bug.sh --skip-cleanup --output-dir /tmp/sriov-investigation
```

---

## Expected Output

### Success (Bug Reproduced)
```
[INFO] Output directory: /tmp/incomplete_nad_bug_1762972707
[SUCCESS] SR-IOV operator found
[SUCCESS] Namespace ready
[SUCCESS] SriovNetwork created successfully
[SUCCESS] NAD created after 12s
[SUCCESS] NAD configuration captured to ...
[ERROR] BUG CONFIRMED: Found 'VF pci addr is required' error in pod events
[SUCCESS] Bug reproduction successful!
[SUCCESS] Logs archived to: /tmp/incomplete_nad_bug_logs_20251112_134522.tar.gz
```

### Analysis Output
```
=== NAD Configuration Analysis ===

Expected Fields in CNI Config:
  - cniVersion: ✓
  - name: ✓
  - type: ✓
  - resourceName: ? (CRITICAL - should be present)
  - pciAddress: ? (CRITICAL - should be present)

Actual CNI Config:
{
  "cniVersion": "1.0.0",
  "name": "reproduce-nad-test",
  "type": "sriov",
  "vlan": 0,
  "vlanQoS": 0,
  "capabilities": {"mac": true, "ips": true},
  "logLevel": "debug",
  "ipam": {"type": "static"}
}

Missing Fields Check:
  ❌ resourceName: MISSING (BUG CONFIRMED)
  ❌ pciAddress: MISSING (BUG CONFIRMED)
```

---

## Understanding the Bug Evidence

### Phase 1: NAD Creation
```bash
$ oc get networkattachmentdefinition -n reproduce-nad-bug-*
NAME                      AGE
reproduce-nad-test        1m
```
✅ NAD is created successfully.

### Phase 2: NAD Configuration
```json
{
  "cniVersion": "1.0.0",
  "name": "reproduce-nad-test",
  "type": "sriov",
  "vlan": 0,
  "vlanQoS": 0,
  "ipam": {"type": "static"}
}
```
❌ Config is incomplete - missing `resourceName` and `pciAddress`.

### Phase 3: Pod Failure
```
Warning  FailedCreatePodSandBox  pod/test-pod-sriov
Message: ... SRIOV-CNI failed to load netconf: LoadConf(): VF pci addr is required
```
❌ Pod fails because CNI config is incomplete.

---

## Manual Investigation Steps

If you run with `--skip-cleanup`, you can investigate further:

### 1. Check NAD Content
```bash
NAMESPACE=$(oc get ns -l sriov-test=true -o jsonpath='{.items[0].metadata.name}')
oc get networkattachmentdefinition -n $NAMESPACE -o jsonpath='{.items[0].spec.config}' | jq .
```

### 2. Check Pod Logs
```bash
oc logs test-pod-sriov -n $NAMESPACE -c test-container || echo "Container not started due to CNI failure"
```

### 3. Check Operator Logs for NAD Creation
```bash
oc logs -n openshift-sriov-network-operator -l app=sriov-network-operator --tail=100 | grep -i "nad\|networkattachment\|reconcil"
```

### 4. Check Why Fields Are Missing
```bash
# Look for any error in operator logs
oc logs -n openshift-sriov-network-operator -l app=sriov-network-operator --tail=500 | grep -i "error\|warn" | head -20
```

---

## For Upstream Bug Reporting

### Files to Include
1. Archive file: `incomplete_nad_bug_logs_*.tar.gz`
2. Or individual files:
   - `02_nad_cni_config.json`
   - `02_nad_analysis.txt`
   - `03_pod_events.txt`
   - `BUG_REPORT.md`
   - `04_operator_logs.txt`

### Issue Template
```markdown
## Title
SR-IOV Operator: NetworkAttachmentDefinition missing resourceName and pciAddress fields

## Description
When an SR-IOV operator creates a NetworkAttachmentDefinition for a SriovNetwork,
the resulting NAD has incomplete CNI configuration, missing critical fields:
- resourceName (should be "openshift.io/{resourceName}")
- pciAddress (should be VF PCI address)

This causes pod attachment to fail with: "VF pci addr is required"

## Reproduction
See attached logs from reproduction script:
- reproduce_incomplete_nad_bug.sh
- Output directory: incomplete_nad_bug_logs_*.tar.gz

## Expected Behavior
NAD should contain:
- resourceName field from SriovNetwork spec
- pciAddress field determined by operator querying node

## Actual Behavior
NAD is created but with incomplete config, causing pod attachment failures.

## Logs and Evidence
See attached files in tar archive.
```

---

## Troubleshooting

### Script Fails: "SR-IOV operator not found"
**Problem**: Operator not deployed or wrong namespace  
**Solution**:
```bash
# Find where operator is deployed
oc get deployment -A | grep -i sriov

# If found in different namespace, edit script or update operator installation
# Default expects: openshift-sriov-network-operator
```

### Script Fails: "Missing kubeconfig"
**Problem**: Not connected to cluster  
**Solution**:
```bash
# Set kubeconfig
export KUBECONFIG=/path/to/kubeconfig

# Or use oc login
oc login https://your-cluster-api:6443
```

### NAD Not Created
**Problem**: Operator is not reconciling SriovNetwork  
**Solution**:
```bash
# Check operator logs
oc logs -n openshift-sriov-network-operator -l app=sriov-network-operator --tail=100

# Check SriovNetwork status
oc describe sriovnetwork -A

# This might indicate OCPBUGS-64886 instead of this bug
```

### Pod Not Failing
**Problem**: Pod might succeed if hardware configuration is different  
**Solution**: This is OK - the script will still capture the NAD config regardless of pod status.

---

## Related Issues

- **OCPBUGS-64886**: NAD not created at all (different issue)
- **This Bug**: NAD created but incomplete (NEW)

---

## Script Parameters

```bash
./reproduce_incomplete_nad_bug.sh [OPTIONS]

Options:
  --skip-cleanup      Keep test resources after script finishes
  --output-dir PATH   Directory to save logs (default: /tmp/incomplete_nad_bug_*)
  --help              Show help message
```

---

## Performance Notes

- **Total Runtime**: 2-3 minutes (includes timeouts)
- **Key Timeouts**:
  - NAD creation wait: 60 seconds
  - Pod event capture: 5 seconds
  - Cleanup: ~10 seconds

---

## Next Steps After Reproduction

1. **Archive Logs**: Script automatically creates `.tar.gz`
2. **Review Evidence**: Check `BUG_REPORT.md` and analysis files
3. **File Upstream Issue**: Use template above with attached logs
4. **Reference**: Include link to `DEEP_DIVE_INCOMPLETE_NAD_BUG.md`
5. **Share**: Send tar archive and deep dive analysis to operator team

---

## See Also

- `DEEP_DIVE_INCOMPLETE_NAD_BUG.md` - Detailed technical analysis
- `UPSTREAM_OPERATOR_BUG_INCOMPLETE_NAD.md` - Original bug report
- `tests/sriov/helpers.go` - Test workarounds (WORKAROUND_* functions)
- `sriov_operator_networking_test.go` - Test that discovers this bug

