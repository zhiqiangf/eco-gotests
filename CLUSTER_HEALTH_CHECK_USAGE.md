# Cluster Health Check Script - Usage Guide

## Overview

The `cluster_health_check.sh` script provides automated, comprehensive cluster health verification for SR-IOV test readiness. It performs 10+ critical and informational checks and generates formatted reports.

## Quick Start

```bash
# Basic health check
./cluster_health_check.sh

# Verbose output
./cluster_health_check.sh --verbose

# Generate JSON report
./cluster_health_check.sh --output json > report.json

# Generate HTML report
./cluster_health_check.sh --output html > report.html
```

## Features

### Checks Performed

#### Critical Checks (Must Pass)
- ✅ Kubernetes API server responsiveness
- ✅ All nodes in Ready state
- ✅ All SR-IOV operator pods running
- ✅ Multus CNI deployed
- ✅ OLM operator running

#### Important Checks (Should Pass)
- ✅ Machine Config Pools stable and updated
- ✅ SR-IOV CSV in Succeeded phase
- ✅ SR-IOV policies and networks configured

#### Informational Checks
- ✅ No orphaned test namespaces
- ✅ Sufficient cluster resources
- ✅ Kubernetes version compatibility

### Output Formats

#### Text Format (Default)
```bash
./cluster_health_check.sh
```

Produces a formatted report with:
- Color-coded status indicators
- Summary statistics
- Overall verdict
- Cluster information

#### JSON Format
```bash
./cluster_health_check.sh --output json
```

Output:
```json
{
  "timestamp": "2025-11-11T10:30:45Z",
  "cluster": {
    "context": "...",
    "api_server": "..."
  },
  "summary": {
    "total_checks": 10,
    "passed": 10,
    "failed": 0,
    "warnings": 0
  },
  "status": "READY",
  "ready_for_testing": true
}
```

#### HTML Format
```bash
./cluster_health_check.sh --output html > report.html
```

Generates a professional HTML report with:
- Visual summary statistics
- Color-coded results
- Cluster information
- Responsive design

### Verbose Mode

Enable detailed logging:
```bash
./cluster_health_check.sh --verbose
```

Displays:
- Individual metric values
- Resource counts
- Diagnostic information
- Enhanced troubleshooting details

## Checks in Detail

### 1. Kubernetes API Server
**Purpose:** Verify cluster API is responsive  
**Command:** `oc api-resources`  
**Status:** Critical

### 2. Node Status
**Purpose:** Ensure all nodes are ready  
**Checks:**
- Total nodes
- Ready nodes
- Node roles (master/worker/sriov)

**Command:** `oc get nodes`  
**Status:** Critical

### 3. SR-IOV Operator
**Purpose:** Verify operator is running  
**Checks:**
- sriov-network-operator pod
- sriov-device-plugin pod
- sriov-network-config-daemon pods (one per node)

**Command:** `oc get pods -n openshift-sriov-network-operator`  
**Status:** Critical

### 4. Multus CNI
**Purpose:** Verify network plugin deployment  
**Checks:**
- Multus pod count
- Pod running status

**Command:** `oc get pods -n openshift-multus`  
**Status:** Critical

### 5. Machine Config Pools
**Purpose:** Check system stability  
**Checks:**
- All MCPs updated
- No degraded nodes
- Pool status

**Command:** `oc get mcp`  
**Status:** Important

### 6. OLM Operator
**Purpose:** Verify lifecycle management  
**Checks:**
- OLM operator running
- Subscription management

**Command:** `oc get pods -n openshift-operator-lifecycle-manager`  
**Status:** Critical

### 7. SR-IOV CSV
**Purpose:** Check operator health  
**Checks:**
- CSV phase (should be "Succeeded")
- Operator readiness

**Command:** `oc get csv -n openshift-sriov-network-operator`  
**Status:** Important

### 8. Orphaned Namespaces
**Purpose:** Detect test artifacts  
**Checks:**
- No e2e-* namespaces
- No test-* namespaces

**Command:** `oc get ns`  
**Status:** Informational

### 9. SR-IOV Resources
**Purpose:** Verify configuration  
**Checks:**
- SR-IOV policies
- SR-IOV networks
- Resource count

**Command:** `oc get sriovnetworknodepolicy`, `oc get sriovnetwork`  
**Status:** Informational

### 10. Cluster Resources
**Purpose:** Check utilization  
**Checks:**
- Total pod count
- Resource availability

**Command:** `oc get pods -A`  
**Status:** Informational

### 11. Kubernetes Version
**Purpose:** Version compatibility  
**Checks:**
- Kubernetes version

**Command:** `oc version`  
**Status:** Informational

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All critical checks passed - Ready for testing |
| 1 | One or more critical checks failed - Not ready |

## Usage Examples

### Automated Monitoring

```bash
# Run every 5 minutes
while true; do
    echo "=== $(date) ===" >> health_check.log
    ./cluster_health_check.sh >> health_check.log 2>&1
    sleep 300
done
```

### CI/CD Integration

```bash
# Pre-test verification
./cluster_health_check.sh || {
    echo "Cluster not ready for tests"
    exit 1
}

# Run tests
ginkgo -v ./tests/sriov/
```

### Reporting

```bash
# Generate daily reports
./cluster_health_check.sh --output html > reports/health_$(date +%Y%m%d).html

# Create JSON for log aggregation
./cluster_health_check.sh --output json | jq . > metrics.json
```

### Troubleshooting

```bash
# Verbose diagnostics
./cluster_health_check.sh --verbose 2>&1 | tee debug.log

# Check specific namespace
oc get pods -n openshift-sriov-network-operator -o wide

# Monitor real-time
watch './cluster_health_check.sh'
```

## Output Examples

### Passing Health Check

```
════════════════════════════════════════════════════════════════════════════════

📊 SUMMARY
════════════════════════════════════════════════════════════════════════════════

Total Checks:           11
Passed:                 11 ✅
Failed:                 0 ❌
Warnings:               0 ⚠️

Overall Status:         ✅ HEALTHY

════════════════════════════════════════════════════════════════════════════════

🎯 VERDICT
════════════════════════════════════════════════════════════════════════════════

✅ READY FOR FULL TEST SUITE EXECUTION

All critical checks passed. Cluster is in excellent condition for running
the complete SR-IOV test suite.

Recommended Action: Proceed with test execution
```

### Failing Health Check

```
📊 SUMMARY
════════════════════════════════════════════════════════════════════════════════

Total Checks:           11
Passed:                 9 ✅
Failed:                 2 ❌
Warnings:               0 ⚠️

Overall Status:         ❌ ISSUES DETECTED

════════════════════════════════════════════════════════════════════════════════

🎯 VERDICT
════════════════════════════════════════════════════════════════════════════════

❌ NOT READY FOR TESTING

Critical issues detected. Address failures before running tests.

Recommended Action: Review failures above and take corrective action
```

## Troubleshooting

### Script Fails to Connect

**Problem:** "Unable to connect to cluster"  
**Solution:** Verify kubeconfig
```bash
export KUBECONFIG=/path/to/kubeconfig
oc cluster-info
```

### Some Nodes Not Ready

**Problem:** Script shows "Not all nodes Ready"  
**Solution:** Debug node status
```bash
./cluster_health_check.sh --verbose
oc describe node <node-name>
```

### SR-IOV Operator Pods Down

**Problem:** "Not all SR-IOV operator pods Running"  
**Solution:** Check operator logs
```bash
oc logs -n openshift-sriov-network-operator -f deployment/sriov-network-operator
```

### Orphaned Test Namespaces

**Problem:** "Found orphaned test namespaces"  
**Solution:** Clean up
```bash
oc delete ns $(oc get ns --no-headers | grep -E "e2e-|test-" | awk '{print $1}')
```

## Performance Considerations

- **Runtime:** ~5-10 seconds for full check
- **Network:** Minimal bandwidth usage
- **API Load:** Low (read-only operations)
- **Impact:** Safe to run frequently

## Integration Examples

### With Test Runner

```bash
#!/bin/bash
set -e

# Check cluster health
./cluster_health_check.sh || exit 1

# Run SR-IOV tests
export GOTOOLCHAIN=auto
ginkgo -v ./tests/sriov/

# Generate final report
./cluster_health_check.sh --output html > test_health_final.html
```

### Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: cluster-health-check
spec:
  schedule: "*/5 * * * *"  # Every 5 minutes
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: health-check
            image: image-with-oc-installed:latest
            command:
            - /path/to/cluster_health_check.sh
            - --output
            - json
          restartPolicy: OnFailure
```

## Contributing

To add new checks:

1. Create function: `check_new_feature()`
2. Add to `run_all_checks()`
3. Update documentation
4. Test with `--verbose` flag

## License

MIT License - Free to use and modify

## Support

For issues or suggestions, contact the development team.

---

**Version:** 1.0  
**Last Updated:** November 11, 2025  
**Tested On:** OpenShift 4.21, Kubernetes 1.34.1
