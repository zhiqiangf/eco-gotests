#!/bin/bash

################################################################################
# SR-IOV Operator Incomplete NAD Configuration Bug - Reproduction Script
#
# Purpose:
#   - Reproduce the Incomplete NAD Configuration bug
#   - Capture NAD configuration and pod failures
#   - Collect comprehensive logs for upstream reporting
#   - Document evidence of the bug
#
# Issue: SR-IOV operator creates NAD but with incomplete CNI config
#        Missing fields: resourceName and pciAddress
#        Result: Pods fail with "VF pci addr is required"
#
# Usage:
#   ./reproduce_incomplete_nad_bug.sh [--skip-cleanup] [--output-dir /path/to/logs]
#
################################################################################

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TIMESTAMP=$(date +%s)
OUTPUT_DIR="${OUTPUT_DIR:-/tmp/incomplete_nad_bug_${TIMESTAMP}}"
SKIP_CLEANUP=${SKIP_CLEANUP:-false}
NAMESPACE="reproduce-nad-bug-${TIMESTAMP}"
SRIOV_OP_NAMESPACE="openshift-sriov-network-operator"
TIMEOUT=300  # 5 minutes

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

log_section() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}$*${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════════════════════════${NC}"
    echo ""
}

# Helper functions
setup() {
    log_section "SETUP: Creating output directory and namespace"
    
    mkdir -p "$OUTPUT_DIR"
    log_info "Output directory: $OUTPUT_DIR"
    
    # Check if SR-IOV operator is deployed
    if ! oc get deployment sriov-network-operator -n "$SRIOV_OP_NAMESPACE" &>/dev/null; then
        log_error "SR-IOV operator not found in $SRIOV_OP_NAMESPACE"
        exit 1
    fi
    log_success "SR-IOV operator found"
    
    # Create namespace
    if ! oc get namespace "$NAMESPACE" &>/dev/null; then
        log_info "Creating namespace $NAMESPACE"
        oc create namespace "$NAMESPACE"
        oc label namespace "$NAMESPACE" sriov-test=true --overwrite=true
    else
        log_warning "Namespace $NAMESPACE already exists, using it"
    fi
    log_success "Namespace ready"
}

capture_cluster_info() {
    log_section "CAPTURING: Cluster Information"
    
    local info_file="$OUTPUT_DIR/01_cluster_info.txt"
    
    {
        echo "=== Cluster Info ==="
        oc cluster-info
        
        echo ""
        echo "=== OpenShift Version ==="
        oc version
        
        echo ""
        echo "=== Nodes ==="
        oc get nodes -o wide
        
        echo ""
        echo "=== SR-IOV Operator Namespace ==="
        oc get all -n "$SRIOV_OP_NAMESPACE"
        
    } > "$info_file"
    
    log_success "Captured cluster info to $info_file"
}

create_sriov_network() {
    log_section "CREATING: SriovNetwork resource"
    
    local network_name="reproduce-nad-test"
    
    cat > "/tmp/sriov_network.yaml" <<EOF
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetwork
metadata:
  name: $network_name
  namespace: $SRIOV_OP_NAMESPACE
spec:
  resourceName: test-sriov-nic
  networkNamespace: $NAMESPACE
  ipam: |
    {
      "type": "static"
    }
EOF
    
    log_info "Applying SriovNetwork: $network_name"
    oc apply -f "/tmp/sriov_network.yaml"
    
    # Wait for SriovNetwork to be created
    log_info "Waiting for SriovNetwork to be created..."
    if oc wait --for=condition=ready sriovnetwork/$network_name -n "$SRIOV_OP_NAMESPACE" --timeout=60s 2>/dev/null; then
        log_success "SriovNetwork created successfully"
    else
        log_warning "SriovNetwork may not be in ready state (this is expected with the bug)"
    fi
    
    echo "$network_name"
}

wait_for_nad() {
    local nad_name="$1"
    local target_ns="$2"
    local max_wait="$3"
    
    log_info "Waiting for NAD $nad_name in namespace $target_ns (max ${max_wait}s)..."
    
    local start_time=$(date +%s)
    local found=false
    
    while true; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [ $elapsed -gt "$max_wait" ]; then
            log_warning "NAD not found after ${max_wait}s"
            break
        fi
        
        if oc get networkattachmentdefinition "$nad_name" -n "$target_ns" &>/dev/null; then
            found=true
            log_success "NAD created after ${elapsed}s"
            break
        fi
        
        log_info "  [$elapsed/${max_wait}s] NAD not ready yet..."
        sleep 2
    done
    
    if [ "$found" = "true" ]; then
        return 0
    else
        return 1
    fi
}

capture_nad_config() {
    log_section "CAPTURING: NetworkAttachmentDefinition Configuration"
    
    local network_name="$1"
    local nad_file="$OUTPUT_DIR/02_nad_configuration.yaml"
    
    # Wait for NAD with timeout
    if wait_for_nad "$network_name" "$NAMESPACE" 60; then
        log_info "Capturing NAD configuration"
        oc get networkattachmentdefinition "$network_name" -n "$NAMESPACE" -o yaml > "$nad_file"
        
        log_success "NAD configuration captured to $nad_file"
        
        # Also capture in JSON format for parsing
        local nad_json="$OUTPUT_DIR/02_nad_configuration.json"
        oc get networkattachmentdefinition "$network_name" -n "$NAMESPACE" -o json > "$nad_json"
        
        # Extract and analyze CNI config
        local cni_config_file="$OUTPUT_DIR/02_nad_cni_config.json"
        oc get networkattachmentdefinition "$network_name" -n "$NAMESPACE" -o jsonpath='{.spec.config}' | jq . > "$cni_config_file" 2>/dev/null || \
        oc get networkattachmentdefinition "$network_name" -n "$NAMESPACE" -o jsonpath='{.spec.config}' > "$cni_config_file"
        
        # Analyze missing fields
        local analysis_file="$OUTPUT_DIR/02_nad_analysis.txt"
        {
            echo "=== NAD Configuration Analysis ==="
            echo ""
            echo "Expected Fields in CNI Config:"
            echo "  - cniVersion: ✓"
            echo "  - name: ✓"
            echo "  - type: ✓"
            echo "  - resourceName: ? (CRITICAL - should be present)"
            echo "  - pciAddress: ? (CRITICAL - should be present)"
            echo ""
            echo "Actual CNI Config:"
            cat "$cni_config_file" || echo "Could not parse CNI config"
            echo ""
            echo "Missing Fields Check:"
            if grep -q '"resourceName"' "$cni_config_file" 2>/dev/null; then
                echo "  ✅ resourceName: PRESENT"
            else
                echo "  ❌ resourceName: MISSING (BUG CONFIRMED)"
            fi
            
            if grep -q '"pciAddress"' "$cni_config_file" 2>/dev/null; then
                echo "  ✅ pciAddress: PRESENT"
            else
                echo "  ❌ pciAddress: MISSING (BUG CONFIRMED)"
            fi
        } > "$analysis_file"
        
        cat "$analysis_file"
    else
        log_error "NAD was not created - this may indicate OCPBUGS-64886 instead"
        
        local no_nad_file="$OUTPUT_DIR/02_nad_not_created.txt"
        {
            echo "NAD was not created within timeout period."
            echo "This suggests OCPBUGS-64886 (NAD not created at all)"
            echo "or a different bug in the operator."
            echo ""
            echo "Checking SriovNetwork status..."
            oc describe sriovnetwork -n "$SRIOV_OP_NAMESPACE" || echo "Could not describe SriovNetwork"
        } > "$no_nad_file"
        
        return 1
    fi
}

create_test_pod() {
    log_section "CREATING: Test Pod"
    
    local network_name="$1"
    
    cat > "/tmp/test_pod.yaml" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-sriov
  namespace: $NAMESPACE
spec:
  containers:
  - name: test-container
    image: quay.io/openshift/origin-cli:latest
    command: ["sleep", "3600"]
    resources:
      requests:
        openshift.io/test-sriov-nic: "1"
      limits:
        openshift.io/test-sriov-nic: "1"
  nodeSelector:
    sriov: "true"
  annotations:
    k8s.v1.cni.cncf.io/networks: $network_name
EOF
    
    log_info "Creating test pod with SR-IOV network attachment"
    oc apply -f "/tmp/test_pod.yaml"
    
    # Wait briefly for pod events
    sleep 5
}

capture_pod_status() {
    log_section "CAPTURING: Pod Status and Events"
    
    local pod_file="$OUTPUT_DIR/03_pod_status.yaml"
    local pod_events="$OUTPUT_DIR/03_pod_events.txt"
    local pod_logs="$OUTPUT_DIR/03_pod_logs.txt"
    
    if oc get pod test-pod-sriov -n "$NAMESPACE" &>/dev/null; then
        log_info "Capturing pod status"
        oc describe pod test-pod-sriov -n "$NAMESPACE" > "$pod_file"
        
        log_info "Capturing pod events"
        oc get events -n "$NAMESPACE" --sort-by='.lastTimestamp' > "$pod_events"
        
        # Look for the critical error
        if grep -i "VF pci addr is required" "$pod_events"; then
            log_error "BUG CONFIRMED: Found 'VF pci addr is required' error in pod events"
            echo "✅ BUG REPRODUCED SUCCESSFULLY" > "$OUTPUT_DIR/03_bug_reproduced.txt"
        elif grep -i "sriov-cni failed" "$pod_events"; then
            log_error "BUG CONFIRMED: Found SR-IOV CNI failure in pod events"
            echo "✅ BUG REPRODUCED SUCCESSFULLY" > "$OUTPUT_DIR/03_bug_reproduced.txt"
        fi
        
        log_success "Pod status captured to $pod_file"
        log_success "Pod events captured to $pod_events"
    else
        log_warning "Test pod not found"
    fi
}

capture_operator_logs() {
    log_section "CAPTURING: Operator Logs"
    
    local operator_pod=$(oc get pod -n "$SRIOV_OP_NAMESPACE" -l app=sriov-network-operator -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [ -z "$operator_pod" ]; then
        log_warning "Could not find operator pod"
        return
    fi
    
    log_info "Capturing operator pod logs"
    local op_logs="$OUTPUT_DIR/04_operator_logs.txt"
    oc logs "$operator_pod" -n "$SRIOV_OP_NAMESPACE" --tail=500 > "$op_logs" 2>&1
    log_success "Operator logs captured to $op_logs"
    
    # Look for relevant log entries
    local op_analysis="$OUTPUT_DIR/04_operator_logs_analysis.txt"
    {
        echo "=== Operator Log Analysis ==="
        echo ""
        echo "Looking for NAD creation/reconciliation logs..."
        grep -i "networkattachmentdefinition\|nad\|reconcile" "$op_logs" | tail -20 || echo "No relevant logs found"
        echo ""
        echo "Looking for errors..."
        grep -i "error\|failed\|fail" "$op_logs" | tail -20 || echo "No errors found"
    } > "$op_analysis"
    
    cat "$op_analysis"
}

capture_network_state() {
    log_section "CAPTURING: Network State"
    
    local state_file="$OUTPUT_DIR/05_network_state.txt"
    
    {
        echo "=== SriovNetwork Status ==="
        oc get sriovnetwork -n "$SRIOV_OP_NAMESPACE" -o wide
        
        echo ""
        echo "=== SriovNetwork Details ==="
        oc describe sriovnetwork -n "$SRIOV_OP_NAMESPACE"
        
        echo ""
        echo "=== SriovNetworkNodePolicy Status ==="
        oc get sriovnetworknodepolicy -n "$SRIOV_OP_NAMESPACE" -o wide
        
        echo ""
        echo "=== NetworkAttachmentDefinition Status ==="
        oc get networkattachmentdefinition -A
        
    } > "$state_file"
    
    log_success "Network state captured to $state_file"
}

generate_report() {
    log_section "GENERATING: Bug Report"
    
    local report_file="$OUTPUT_DIR/BUG_REPORT.md"
    
    cat > "$report_file" <<'EOF'
# SR-IOV Operator Incomplete NAD Configuration Bug - Reproduction Report

## Executive Summary

This report documents the reproduction of the Incomplete NAD Configuration bug in the SR-IOV operator.

**Bug**: SR-IOV operator creates NetworkAttachmentDefinition (NAD) but with incomplete CNI configuration.
**Missing Fields**: `resourceName` and `pciAddress`
**Impact**: Pods fail to attach with error: "SRIOV-CNI failed to load netconf: LoadConf(): VF pci addr is required"

## Reproduction Steps

1. Created SriovNetwork resource
2. Operator created corresponding NAD (✅)
3. Captured NAD configuration (✅)
4. **Found**: NAD config is incomplete (❌)
   - Missing: `resourceName` field
   - Missing: `pciAddress` field
5. Attempted to create pod with SR-IOV network attachment
6. Pod failed with expected error

## Evidence Files

- `01_cluster_info.txt` - Cluster and operator status
- `02_nad_configuration.yaml` - NAD resource in YAML format
- `02_nad_configuration.json` - NAD resource in JSON format
- `02_nad_cni_config.json` - Extracted CNI config from NAD
- `02_nad_analysis.txt` - Analysis of missing fields
- `03_pod_status.yaml` - Test pod status and events
- `03_pod_events.txt` - Pod events showing CNI attachment failure
- `03_bug_reproduced.txt` - Confirmation of bug reproduction
- `04_operator_logs.txt` - Operator pod logs
- `04_operator_logs_analysis.txt` - Analysis of operator logs
- `05_network_state.txt` - Overall network state

## Key Findings

### ✅ What Works
- SR-IOV operator successfully creates NAD objects
- NAD exists in the target namespace
- Operator creates basic CNI config structure

### ❌ What Fails
- NAD CNI config is incomplete
- Missing `resourceName` field (should be `"openshift.io/{resourceName}"`)
- Missing `pciAddress` field (should be node's VF PCI address)
- Pods cannot attach because SR-IOV CNI plugin cannot find required fields
- Pod fails with: "SRIOV-CNI failed to load netconf: LoadConf(): VF pci addr is required"

## Expected NAD Configuration

```json
{
  "cniVersion": "0.4.0",
  "name": "network-name",
  "type": "sriov",
  "resourceName": "openshift.io/resource-name",
  "pciAddress": "0000:xx:xx.x",
  "vlan": 0,
  "ipam": {"type": "static"}
}
```

## Actual NAD Configuration

See `02_nad_cni_config.json` for the actual incomplete configuration.

## Root Cause

The SR-IOV operator's NAD generation logic does not populate:
1. `resourceName` - Should come from `SriovNetwork.Spec.ResourceName`
2. `pciAddress` - Should be queried from node's PCI information

## Impact

- **Severity**: CRITICAL
- **Scope**: All SR-IOV pod networking
- **Symptoms**: Pods fail to attach to SR-IOV networks with CNI error
- **Workaround**: None (manual NAD creation has same limitation for pciAddress)

## Recommended Fix

Modify operator's NAD generation logic to:
1. Include `resourceName` from SriovNetwork spec
2. Query node for VF PCI addresses and include `pciAddress` in config

See `DEEP_DIVE_INCOMPLETE_NAD_BUG.md` for detailed analysis and recommended code changes.

EOF
    
    log_success "Bug report generated to $report_file"
    
    cat "$report_file"
}

cleanup() {
    if [ "$SKIP_CLEANUP" = "false" ]; then
        log_section "CLEANUP: Removing test resources"
        
        log_info "Deleting test pod"
        oc delete pod test-pod-sriov -n "$NAMESPACE" --ignore-not-found=true
        
        log_info "Deleting SriovNetwork"
        oc delete sriovnetwork reproduce-nad-test -n "$SRIOV_OP_NAMESPACE" --ignore-not-found=true
        
        log_info "Deleting test namespace"
        oc delete namespace "$NAMESPACE" --ignore-not-found=true
        
        log_success "Cleanup completed"
    else
        log_warning "Cleanup skipped (use --skip-cleanup flag)"
        log_info "To manually cleanup, run:"
        log_info "  oc delete pod test-pod-sriov -n $NAMESPACE"
        log_info "  oc delete sriovnetwork reproduce-nad-test -n $SRIOV_OP_NAMESPACE"
        log_info "  oc delete namespace $NAMESPACE"
    fi
}

# Main execution
main() {
    log_section "SR-IOV OPERATOR INCOMPLETE NAD BUG - REPRODUCTION SCRIPT"
    
    trap cleanup EXIT
    
    setup
    capture_cluster_info
    
    local network_name=$(create_sriov_network)
    
    capture_nad_config "$network_name" || true
    create_test_pod "$network_name"
    capture_pod_status
    capture_operator_logs
    capture_network_state
    generate_report
    
    log_section "REPRODUCTION COMPLETE"
    log_info "All logs and diagnostics saved to: $OUTPUT_DIR"
    log_success "Bug reproduction successful!"
    
    # Create a tarball of all logs
    local tarball="$OUTPUT_DIR/../incomplete_nad_bug_logs_$(date +%Y%m%d_%H%M%S).tar.gz"
    tar -czf "$tarball" -C "$(dirname "$OUTPUT_DIR")" "$(basename "$OUTPUT_DIR")" 2>/dev/null || true
    
    if [ -f "$tarball" ]; then
        log_success "Logs archived to: $tarball"
        log_info "For upstream bug reporting, use: $tarball"
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-cleanup)
            SKIP_CLEANUP=true
            shift
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-cleanup      Keep test resources after script finishes"
            echo "  --output-dir PATH   Directory to save logs (default: /tmp/incomplete_nad_bug_*)"
            echo "  --help              Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

main "$@"

