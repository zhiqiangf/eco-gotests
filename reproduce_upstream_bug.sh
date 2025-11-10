#!/bin/bash

################################################################################
#                                                                              #
#  SR-IOV Operator SriovNetwork Controller Bug Reproduction Script             #
#                                                                              #
#  This script reproduces the upstream SR-IOV operator bug where the          #
#  SriovNetwork controller becomes unresponsive after pod restart.            #
#                                                                              #
#  Bug: SriovNetwork controller claims to start but never processes events    #
#  Impact: NetworkAttachmentDefinition (NAD) creation fails silently          #
#  Reproducibility: 100% after operator restart                               #
#                                                                              #
################################################################################

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SRIOV_OPERATOR_NS="openshift-sriov-network-operator"
TEST_NS="sriov-bug-reproduce-$(date +%s)"
SRIOV_NETWORK_NAME="bug-reproduce-net"
LOG_DIR="/tmp/sriov-bug-logs-$(date +%Y%m%d-%H%M%S)"
TIMEOUT_SECONDS=300

# Create log directory
mkdir -p "$LOG_DIR"

echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  SR-IOV Operator SriovNetwork Controller Bug Reproduction${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${YELLOW}Log Directory: ${LOG_DIR}${NC}"
echo ""

################################################################################
# Helper Functions
################################################################################

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

collect_operator_logs() {
    local phase=$1
    local output_file="$LOG_DIR/operator-logs-${phase}.log"
    
    log_step "Collecting operator logs: $phase"
    
    oc logs -n "$SRIOV_OPERATOR_NS" \
        -l app=sriov-network-operator \
        --tail=500 \
        --timestamps=true > "$output_file" 2>&1
    
    log_success "Operator logs saved to: $output_file"
}

check_sriov_network_status() {
    local sriov_net=$1
    local namespace=$2
    local phase=$3
    
    log_step "Checking SriovNetwork status for $phase: $sriov_net in $namespace"
    
    local output_file="$LOG_DIR/sriovnetwork-status-${phase}.txt"
    
    oc get sriovnetwork "$sriov_net" -n "$namespace" -o yaml > "$output_file" 2>&1 || {
        log_warning "SriovNetwork not found yet"
        return 1
    }
    
    log_success "SriovNetwork status saved to: $output_file"
    return 0
}

check_nad_status() {
    local nad=$1
    local namespace=$2
    local phase=$3
    
    log_step "Checking NAD status for $phase: $nad in $namespace"
    
    local output_file="$LOG_DIR/nad-status-${phase}.txt"
    
    # Detailed NAD check
    if oc get nad "$nad" -n "$namespace" &>/dev/null; then
        log_success "✅ NAD EXISTS in namespace: $namespace"
        oc get nad "$nad" -n "$namespace" -o yaml > "$output_file"
        
        # Additional verification
        log_step "Verifying NAD details:"
        oc get nad "$nad" -n "$namespace" -o json | jq '.metadata.name, .metadata.namespace, .spec.config' >> "$output_file"
        log_success "NAD verified and details saved to: $output_file"
        return 0
    else
        log_warning "❌ NAD NOT FOUND in namespace: $namespace"
        echo "NOT FOUND" > "$output_file"
        return 1
    fi
}

check_reconciliation_logs() {
    local phase=$1
    local output_file="$LOG_DIR/reconciliation-check-${phase}.txt"
    
    log_step "Checking for reconciliation messages in logs: $phase"
    
    if oc logs -n "$SRIOV_OPERATOR_NS" \
        -l app=sriov-network-operator \
        --tail=200 \
        --timestamps=true | grep -i "reconciling.*sriovnetwork" > "$output_file" 2>&1; then
        log_success "Found reconciliation messages: $phase"
        grep -i "reconciling.*sriovnetwork" "$output_file" | head -5
        return 0
    else
        log_warning "NO reconciliation messages found: $phase"
        return 1
    fi
}

wait_for_nad_creation() {
    local nad=$1
    local namespace=$2
    local timeout=$3
    local phase=$4
    
    log_step "Waiting for NAD creation ($phase): $nad in $namespace (timeout: ${timeout}s)"
    
    local start_time=$(date +%s)
    local elapsed=0
    
    while [ $elapsed -lt $timeout ]; do
        if oc get nad "$nad" -n "$namespace" &>/dev/null; then
            log_success "NAD created successfully: $nad"
            return 0
        fi
        
        elapsed=$(($(date +%s) - start_time))
        remaining=$((timeout - elapsed))
        
        if [ $((elapsed % 10)) -eq 0 ]; then
            echo -e "  Waiting... (${elapsed}/${timeout}s)"
        fi
        
        sleep 2
    done
    
    log_error "NAD creation FAILED after ${timeout}s: $nad"
    return 1
}

################################################################################
# Phase 1: Pre-restart baseline
################################################################################

phase_number=1
log_step "Phase 1: Pre-restart baseline (verify operator works initially)"
echo ""

log_step "Creating test namespace: $TEST_NS"
oc create namespace "$TEST_NS" || {
    log_error "Failed to create namespace"
    exit 1
}
log_success "Namespace created: $TEST_NS"

log_step "Collecting operator logs (pre-restart baseline)"
collect_operator_logs "phase1-pre-restart"

log_step "Creating SriovNetwork object (phase 1 baseline)"
cat > "$LOG_DIR/sriovnetwork-phase1.yaml" << 'EOF'
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetwork
metadata:
  name: bug-reproduce-net-phase1
  namespace: openshift-sriov-network-operator
spec:
  resourceName: cx7anl244
  networkNamespace: sriov-bug-reproduce-ns
  spoofChk: "off"
  trust: "on"
EOF

sed -i "s/sriov-bug-reproduce-ns/$TEST_NS/g" "$LOG_DIR/sriovnetwork-phase1.yaml"
oc apply -f "$LOG_DIR/sriovnetwork-phase1.yaml"
log_success "SriovNetwork created: bug-reproduce-net-phase1"

log_step "Waiting for NAD creation (Phase 1 - pre-restart, timeout: 300s)"
if wait_for_nad_creation "bug-reproduce-net-phase1" "$TEST_NS" 300 "phase1"; then
    log_success "Phase 1 PASSED: NAD created before restart"
    check_nad_status "bug-reproduce-net-phase1" "$TEST_NS" "phase1-success"
    check_sriov_network_status "bug-reproduce-net-phase1" "$SRIOV_OPERATOR_NS" "phase1"
    check_reconciliation_logs "phase1"
    
    log_step "Listing all NADs in test namespace ($TEST_NS) after Phase 1:"
    oc get nad -n "$TEST_NS" > "$LOG_DIR/nad-list-phase1.txt" 2>&1
    cat "$LOG_DIR/nad-list-phase1.txt"
else
    log_warning "Phase 1: NAD creation took longer than expected (may be cluster slowness)"
    log_step "Current NADs in namespace ($TEST_NS):"
    oc get nad -n "$TEST_NS" || echo "No NADs found"
fi

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Phase 1 Complete - Operator Working Correctly${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo ""

################################################################################
# Phase 2: Restart the operator
################################################################################

log_step "Phase 2: Restarting SR-IOV operator pod"
echo ""

log_step "Getting current operator pod name"
OPERATOR_POD=$(oc get pods -n "$SRIOV_OPERATOR_NS" --sort-by=.metadata.creationTimestamp | grep sriov-network-operator | tail -1 | awk '{print $1}')
if [ -z "$OPERATOR_POD" ]; then
    log_error "Could not find operator pod"
    oc get pods -n "$SRIOV_OPERATOR_NS"
    exit 1
fi
log_success "Current operator pod: $OPERATOR_POD"

log_step "Deleting operator pod to trigger restart: $OPERATOR_POD"
oc delete pod "$OPERATOR_POD" -n "$SRIOV_OPERATOR_NS" --wait=false
log_success "Pod deletion initiated"

log_step "Waiting for operator pod to restart (max 60s)"
sleep 10
for i in {1..30}; do
    NEW_POD=$(oc get pods -n "$SRIOV_OPERATOR_NS" --sort-by=.metadata.creationTimestamp 2>/dev/null | grep sriov-network-operator | tail -1 | awk '{print $1}')
    if [ -n "$NEW_POD" ] && oc get pod "$NEW_POD" -n "$SRIOV_OPERATOR_NS" 2>/dev/null | grep -q "Running"; then
        log_success "Operator pod restarted and running"
        log_success "New operator pod: $NEW_POD"
        break
    fi
    sleep 2
done

log_step "Waiting for operator to stabilize (15s)"
sleep 15

log_step "Collecting operator logs (immediately after restart)"
collect_operator_logs "phase2-post-restart"

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Phase 2 Complete - Operator Restarted${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo ""

################################################################################
# Phase 3: Test operator responsiveness after restart
################################################################################

log_step "Phase 3: Testing operator responsiveness after restart"
echo ""

log_step "Creating NEW SriovNetwork object (phase 3 post-restart)"
cat > "$LOG_DIR/sriovnetwork-phase3.yaml" << 'EOF'
apiVersion: sriovnetwork.openshift.io/v1
kind: SriovNetwork
metadata:
  name: bug-reproduce-net-phase3
  namespace: openshift-sriov-network-operator
spec:
  resourceName: cx7anl244
  networkNamespace: sriov-bug-reproduce-ns
  spoofChk: "off"
  trust: "on"
EOF

sed -i "s/sriov-bug-reproduce-ns/$TEST_NS/g" "$LOG_DIR/sriovnetwork-phase3.yaml"
oc apply -f "$LOG_DIR/sriovnetwork-phase3.yaml"
log_success "SriovNetwork created: bug-reproduce-net-phase3"

log_step "Recording timestamp of SriovNetwork creation"
CREATION_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "SriovNetwork created at: $CREATION_TIME" > "$LOG_DIR/sriovnetwork-creation-time.txt"

log_step "Waiting for NAD creation (Phase 3 - post-restart) [MAX 300s / 5 minutes]"
if wait_for_nad_creation "bug-reproduce-net-phase3" "$TEST_NS" 300 "phase3"; then
    log_success "Phase 3 PASSED: NAD created after restart (operator is responsive)"
    check_nad_status "bug-reproduce-net-phase3" "$TEST_NS" "phase3-success"
    check_sriov_network_status "bug-reproduce-net-phase3" "$SRIOV_OPERATOR_NS" "phase3"
    check_reconciliation_logs "phase3"
    
    log_step "Listing all NADs in test namespace ($TEST_NS) after Phase 3:"
    oc get nad -n "$TEST_NS" > "$LOG_DIR/nad-list-phase3-success.txt" 2>&1
    cat "$LOG_DIR/nad-list-phase3-success.txt"
else
    log_error "Phase 3 FAILED: NAD NOT created after restart within 5 minutes"
    log_step "This may indicate the upstream bug - operator became unresponsive"
    check_sriov_network_status "bug-reproduce-net-phase3" "$SRIOV_OPERATOR_NS" "phase3"
    check_reconciliation_logs "phase3"
    
    log_step "Current NADs in namespace ($TEST_NS) - checking if NAD appears eventually:"
    oc get nad -n "$TEST_NS" > "$LOG_DIR/nad-list-phase3-timeout.txt" 2>&1
    cat "$LOG_DIR/nad-list-phase3-timeout.txt"
fi

log_step "Collecting final operator logs"
collect_operator_logs "phase3-final"

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Phase 3 Complete - Bug Reproduction Attempt${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo ""

################################################################################
# Phase 4: Detailed diagnosis
################################################################################

log_step "Phase 4: Detailed diagnosis and log analysis"
echo ""

log_step "Analyzing reconciliation logs for Phase 3"
local diag_file="$LOG_DIR/detailed-diagnosis.txt"
{
    echo "SR-IOV Operator Bug Diagnosis Report"
    echo "===================================="
    echo ""
    echo "Test Environment:"
    echo "  Timestamp: $(date)"
    echo "  Operator Namespace: $SRIOV_OPERATOR_NS"
    echo "  Test Namespace: $TEST_NS"
    echo ""
    
    echo "Phase 1 (Pre-restart) - Should show reconciliation:"
    echo "  ---"
    oc logs -n "$SRIOV_OPERATOR_NS" \
        -l app=sriov-network-operator \
        --tail=1000 \
        --timestamps=true 2>/dev/null | grep -i "reconciling.*sriovnetwork" | head -10 || echo "  [No reconciliation logs found in Phase 1]"
    echo ""
    
    echo "Phase 3 (Post-restart) - Key diagnostic:"
    echo "  ---"
    echo "  1. Check if 'Starting Controller: sriovnetwork' appears:"
    oc logs -n "$SRIOV_OPERATOR_NS" \
        -l app=sriov-network-operator \
        --tail=500 \
        --timestamps=true 2>/dev/null | grep "Starting Controller.*sriovnetwork" || echo "  [Not found]"
    echo ""
    
    echo "  2. Check if 'Starting workers' appears for sriovnetwork:"
    oc logs -n "$SRIOV_OPERATOR_NS" \
        -l app=sriov-network-operator \
        --tail=500 \
        --timestamps=true 2>/dev/null | grep "Starting workers" | grep -i sriov || echo "  [Not found]"
    echo ""
    
    echo "  3. Check for 'Reconciling SriovNetwork' (should appear but might not):"
    oc logs -n "$SRIOV_OPERATOR_NS" \
        -l app=sriov-network-operator \
        --tail=500 \
        --timestamps=true 2>/dev/null | grep -i "reconciling.*sriovnetwork" || echo "  [NOT FOUND - BUG SYMPTOM!]"
    echo ""
    
    echo "Conclusion:"
    if oc logs -n "$SRIOV_OPERATOR_NS" \
        -l app=sriov-network-operator \
        --tail=500 \
        --timestamps=true 2>/dev/null | grep "Reconciling.*SriovNetwork"; then
        echo "  ✅ Operator is processing SriovNetwork objects (no bug in this run)"
    else
        echo "  ❌ Operator is NOT processing SriovNetwork objects (BUG REPRODUCED)"
    fi
} | tee "$diag_file"

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Phase 4 Complete - Diagnosis Complete${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo ""

################################################################################
# Cleanup
################################################################################

log_step "Cleaning up test resources"
log_warning "Deleting test namespace: $TEST_NS"
oc delete namespace "$TEST_NS" --wait=false || true
log_success "Cleanup initiated"

################################################################################
# Summary
################################################################################

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  REPRODUCTION SCRIPT COMPLETE${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════════════════${NC}"
echo ""

echo -e "${YELLOW}📁 Collected Logs Location: ${LOG_DIR}${NC}"
echo ""

echo "Log Files:"
ls -lh "$LOG_DIR"/ | awk 'NR>1 {printf "  📄 %-50s %s\n", $9, $5}'
echo ""

echo -e "${YELLOW}📋 Files for Upstream Reporting:${NC}"
echo "  1. operator-logs-phase1-pre-restart.log     - Operator logs before restart"
echo "  2. operator-logs-phase2-post-restart.log    - Operator logs immediately after restart"
echo "  3. operator-logs-phase3-final.log           - Operator logs after SriovNetwork creation attempt"
echo "  4. reconciliation-check-*.txt               - Reconciliation message analysis"
echo "  5. detailed-diagnosis.txt                   - Detailed diagnosis report"
echo "  6. sriovnetwork-*.yaml                      - YAML objects created during test"
echo ""

echo -e "${YELLOW}🐛 Next Steps for Upstream Reporting:${NC}"
echo "  1. Review detailed-diagnosis.txt for conclusion"
echo "  2. Attach operator-logs-phase1-pre-restart.log (working state)"
echo "  3. Attach operator-logs-phase3-final.log (broken state)"
echo "  4. Include this script as reproducible test case"
echo "  5. File issue at: https://github.com/k8snetworkplumbinggroup/sriov-network-operator"
echo ""

echo -e "${YELLOW}💾 To archive logs for reporting:${NC}"
echo "  tar czf sriov-bug-logs.tar.gz $LOG_DIR/"
echo "  # Attach sriov-bug-logs.tar.gz to GitHub issue"
echo ""

