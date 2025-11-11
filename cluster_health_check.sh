#!/bin/bash

################################################################################
#                                                                              #
#          SR-IOV Cluster Health Check Script - Comprehensive Assessment      #
#                                                                              #
#  Purpose: Automated cluster health verification for SR-IOV test readiness   #
#  Usage:   ./cluster_health_check.sh [--verbose] [--output FORMAT]           #
#  Output:  Text, JSON, or HTML report                                        #
#                                                                              #
################################################################################

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERBOSE=false
OUTPUT_FORMAT="text"
REPORT_FILE="${SCRIPT_DIR}/cluster_health_report_$(date +%s).txt"
TEMP_DIR=$(mktemp -d)
PASS_COUNT=0
FAIL_COUNT=0
WARN_COUNT=0

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Cleanup
trap "rm -rf $TEMP_DIR" EXIT

################################################################################
# UTILITY FUNCTIONS
################################################################################

log_info() {
    echo -e "${BLUE}ℹ${NC} $*"
}

log_pass() {
    echo -e "${GREEN}✅${NC} $*"
    ((PASS_COUNT++))
}

log_fail() {
    echo -e "${RED}❌${NC} $*"
    ((FAIL_COUNT++))
}

log_warn() {
    echo -e "${YELLOW}⚠️${NC} $*"
    ((WARN_COUNT++))
}

log_section() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}$*${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
    echo ""
}

verbose_log() {
    if [ "$VERBOSE" = true ]; then
        echo -e "${BLUE}[VERBOSE]${NC} $*"
    fi
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --verbose)
                VERBOSE=true
                shift
                ;;
            --output)
                OUTPUT_FORMAT="$2"
                shift 2
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
  --verbose          Enable verbose output
  --output FORMAT    Output format: text (default), json, html
  --help             Show this help message

Examples:
  $0
  $0 --verbose
  $0 --output json > report.json
  $0 --output html > report.html
EOF
}

################################################################################
# HEALTH CHECK FUNCTIONS
################################################################################

check_api_server() {
    log_section "Checking Cluster API Server"
    
    if oc api-resources > /dev/null 2>&1; then
        log_pass "Kubernetes API responsive"
        return 0
    else
        log_fail "Kubernetes API not responsive"
        return 1
    fi
}

check_nodes() {
    log_section "Checking Node Status"
    
    local total_nodes ready_nodes not_ready_nodes
    
    total_nodes=$(oc get nodes --no-headers 2>/dev/null | wc -l)
    ready_nodes=$(oc get nodes --no-headers 2>/dev/null | grep -c " Ready " || echo 0)
    not_ready_nodes=$((total_nodes - ready_nodes))
    
    verbose_log "Total nodes: $total_nodes, Ready: $ready_nodes, Not Ready: $not_ready_nodes"
    
    if [ "$ready_nodes" -eq "$total_nodes" ] && [ "$total_nodes" -gt 0 ]; then
        log_pass "All $ready_nodes/$total_nodes nodes Ready"
        
        # Check for nodes by role
        local masters workers sriov_nodes
        masters=$(oc get nodes --no-headers 2>/dev/null | grep -c "control-plane" || echo 0)
        workers=$(oc get nodes --no-headers 2>/dev/null | grep -c "worker" || echo 0)
        sriov_nodes=$(oc get nodes --no-headers 2>/dev/null | grep -c "sriov" || echo 0)
        
        verbose_log "Node breakdown: Masters=$masters, Workers=$workers, SR-IOV=$sriov_nodes"
        
        return 0
    else
        log_fail "Not all nodes Ready: $ready_nodes/$total_nodes"
        oc get nodes --no-headers 2>/dev/null | grep -v " Ready " | head -5
        return 1
    fi
}

check_sriov_operator() {
    log_section "Checking SR-IOV Operator Status"
    
    local operator_pods total_pods ready_pods
    
    operator_pods=$(oc get pods -n openshift-sriov-network-operator --no-headers 2>/dev/null)
    total_pods=$(echo "$operator_pods" | wc -l)
    ready_pods=$(echo "$operator_pods" | grep -c "Running" || echo 0)
    
    verbose_log "Total SR-IOV pods: $total_pods, Running: $ready_pods"
    
    if [ "$ready_pods" -eq "$total_pods" ] && [ "$total_pods" -gt 0 ]; then
        log_pass "All $ready_pods/$total_pods SR-IOV operator pods Running"
        
        # Check deployment status
        local deployment_ready
        deployment_ready=$(oc get deployment -n openshift-sriov-network-operator \
            -o jsonpath='{.items[0].status.readyReplicas}' 2>/dev/null || echo 0)
        
        verbose_log "Deployment ready replicas: $deployment_ready"
        
        return 0
    else
        log_fail "Not all SR-IOV pods Running: $ready_pods/$total_pods"
        echo "$operator_pods" | grep -v "Running" | head -5
        return 1
    fi
}

check_multus_cni() {
    log_section "Checking Multus CNI Status"
    
    local multus_pods
    multus_pods=$(oc get pods -n openshift-multus -l app=multus --no-headers 2>/dev/null | wc -l)
    
    verbose_log "Multus pods found: $multus_pods"
    
    if [ "$multus_pods" -gt 0 ]; then
        log_pass "Multus CNI deployed ($multus_pods pods)"
        return 0
    else
        log_fail "Multus CNI not deployed or pods not running"
        return 1
    fi
}

check_machine_config_pools() {
    log_section "Checking Machine Config Pools (MCP)"
    
    local mcp_data updated_mcps total_mcps
    mcp_data=$(oc get mcp --no-headers 2>/dev/null)
    total_mcps=$(echo "$mcp_data" | wc -l)
    updated_mcps=$(echo "$mcp_data" | grep -c "True.*False.*False" || echo 0)
    
    verbose_log "Total MCPs: $total_mcps, Updated/Stable: $updated_mcps"
    
    if [ "$updated_mcps" -eq "$total_mcps" ] && [ "$total_mcps" -gt 0 ]; then
        log_pass "All $total_mcps MCPs Updated and Stable"
        return 0
    else
        log_warn "$updated_mcps/$total_mcps MCPs are updated/stable"
        echo "$mcp_data" | grep -v "True.*False.*False" | head -5
        return 0  # Warn but don't fail
    fi
}

check_olm_operator() {
    log_section "Checking OLM Operator"
    
    local olm_pods
    olm_pods=$(oc get pods -n openshift-operator-lifecycle-manager -l app=olm-operator \
        --no-headers 2>/dev/null | grep -c "Running" || echo 0)
    
    verbose_log "OLM operator pods: $olm_pods"
    
    if [ "$olm_pods" -gt 0 ]; then
        log_pass "OLM operator running"
        return 0
    else
        log_fail "OLM operator not running"
        return 1
    fi
}

check_sriov_csv() {
    log_section "Checking SR-IOV CSV Status"
    
    local csv_phase
    csv_phase=$(oc get csv -n openshift-sriov-network-operator \
        -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
    
    verbose_log "CSV phase: $csv_phase"
    
    if [ "$csv_phase" = "Succeeded" ]; then
        log_pass "SR-IOV CSV in Succeeded phase"
        return 0
    else
        log_warn "SR-IOV CSV phase: $csv_phase"
        return 0  # Warn but don't fail
    fi
}

check_orphaned_namespaces() {
    log_section "Checking for Orphaned Test Namespaces"
    
    local orphan_count
    orphan_count=$(oc get ns --no-headers 2>/dev/null | grep -E "e2e-|test-" | wc -l)
    
    verbose_log "Orphaned namespaces found: $orphan_count"
    
    if [ "$orphan_count" -eq 0 ]; then
        log_pass "No orphaned test namespaces"
        return 0
    else
        log_warn "Found $orphan_count orphaned test namespaces"
        oc get ns --no-headers 2>/dev/null | grep -E "e2e-|test-" | head -5
        return 0  # Warn but don't fail
    fi
}

check_sriov_resources() {
    log_section "Checking SR-IOV Resources"
    
    # Check policies
    local policies_count
    policies_count=$(oc get sriovnetworknodepolicy -n openshift-sriov-network-operator \
        --no-headers 2>/dev/null | wc -l)
    
    verbose_log "SR-IOV policies: $policies_count"
    
    if [ "$policies_count" -gt 0 ]; then
        log_pass "SR-IOV policies configured ($policies_count)"
    else
        log_warn "No SR-IOV policies found"
    fi
    
    # Check networks
    local networks_count
    networks_count=$(oc get sriovnetwork -n openshift-sriov-network-operator \
        --no-headers 2>/dev/null | wc -l)
    
    verbose_log "SR-IOV networks: $networks_count"
    
    if [ "$networks_count" -ge 0 ]; then
        if [ "$networks_count" -eq 0 ]; then
            log_pass "No orphaned SR-IOV networks (clean state)"
        else
            log_pass "SR-IOV networks configured ($networks_count)"
        fi
    fi
    
    return 0
}

check_cluster_resources() {
    log_section "Checking Cluster Resource Utilization"
    
    local total_pods
    total_pods=$(oc get pods -A --no-headers 2>/dev/null | wc -l)
    
    verbose_log "Total pods running: $total_pods"
    
    if [ "$total_pods" -gt 100 ]; then
        log_pass "Cluster has $total_pods pods running (healthy)"
        return 0
    else
        log_warn "Low pod count: $total_pods"
        return 0  # Warn but don't fail
    fi
}

check_kubernetes_version() {
    log_section "Checking Kubernetes Version"
    
    local version
    version=$(oc version -o json 2>/dev/null | grep -o '"kubernetes":"[^"]*' | cut -d'"' -f4)
    
    verbose_log "Kubernetes version: $version"
    
    if [ -n "$version" ]; then
        log_pass "Kubernetes version: $version"
        return 0
    else
        log_warn "Could not determine Kubernetes version"
        return 0
    fi
}

################################################################################
# COMPREHENSIVE HEALTH CHECK
################################################################################

run_all_checks() {
    local failed=0
    
    # Critical checks
    check_api_server || ((failed++))
    check_nodes || ((failed++))
    check_sriov_operator || ((failed++))
    check_multus_cni || ((failed++))
    check_olm_operator || ((failed++))
    
    # Important checks
    check_machine_config_pools
    check_sriov_csv
    check_sriov_resources
    
    # Informational checks
    check_orphaned_namespaces
    check_cluster_resources
    check_kubernetes_version
    
    return $failed
}

################################################################################
# REPORTING FUNCTIONS
################################################################################

generate_text_report() {
    cat << EOF

╔════════════════════════════════════════════════════════════════════════════════╗
║                                                                                ║
║              ✅ SR-IOV CLUSTER HEALTH CHECK REPORT - $(date '+%Y-%m-%d %H:%M:%S')             ║
║                                                                                ║
╚════════════════════════════════════════════════════════════════════════════════╝

📊 SUMMARY
════════════════════════════════════════════════════════════════════════════════

Total Checks:           $((PASS_COUNT + FAIL_COUNT + WARN_COUNT))
Passed:                 $PASS_COUNT ✅
Failed:                 $FAIL_COUNT ❌
Warnings:               $WARN_COUNT ⚠️

Overall Status:         $([ $FAIL_COUNT -eq 0 ] && echo "✅ HEALTHY" || echo "❌ ISSUES DETECTED")

════════════════════════════════════════════════════════════════════════════════

🎯 VERDICT
════════════════════════════════════════════════════════════════════════════════

EOF

    if [ $FAIL_COUNT -eq 0 ]; then
        cat << EOF
✅ READY FOR FULL TEST SUITE EXECUTION

All critical checks passed. Cluster is in excellent condition for running
the complete SR-IOV test suite.

Recommended Action: Proceed with test execution

EOF
    else
        cat << EOF
❌ NOT READY FOR TESTING

Critical issues detected. Address failures before running tests.

Recommended Action: Review failures above and take corrective action

EOF
    fi

    cat << EOF

════════════════════════════════════════════════════════════════════════════════

Generated: $(date '+%Y-%m-%d %H:%M:%S')
Cluster: $(oc config current-context 2>/dev/null || echo "Unknown")
API Server: $(oc cluster-info 2>/dev/null | grep "Kubernetes control plane" | sed 's/.*at //' || echo "Unknown")

════════════════════════════════════════════════════════════════════════════════

EOF
}

generate_json_report() {
    cat << EOF
{
  "timestamp": "$(date -u +'%Y-%m-%dT%H:%M:%SZ')",
  "cluster": {
    "context": "$(oc config current-context 2>/dev/null || echo "Unknown")",
    "api_server": "$(oc cluster-info 2>/dev/null | grep "Kubernetes control plane" | sed 's/.*at //' || echo "Unknown")"
  },
  "summary": {
    "total_checks": $((PASS_COUNT + FAIL_COUNT + WARN_COUNT)),
    "passed": $PASS_COUNT,
    "failed": $FAIL_COUNT,
    "warnings": $WARN_COUNT
  },
  "status": "$([ $FAIL_COUNT -eq 0 ] && echo "READY" || echo "NOT_READY")",
  "ready_for_testing": $([ $FAIL_COUNT -eq 0 ] && echo "true" || echo "false")
}
EOF
}

generate_html_report() {
    cat << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>SR-IOV Cluster Health Check Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f5f5f5; }
        .container { max-width: 1000px; margin: 0 auto; background-color: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 3px solid #007bff; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 15px; margin: 20px 0; }
        .summary-item { padding: 15px; border-radius: 5px; text-align: center; font-weight: bold; }
        .passed { background-color: #d4edda; color: #155724; }
        .failed { background-color: #f8d7da; color: #721c24; }
        .warning { background-color: #fff3cd; color: #856404; }
        .total { background-color: #d1ecf1; color: #0c5460; }
        .status { padding: 15px; border-radius: 5px; margin: 20px 0; font-size: 18px; font-weight: bold; }
        .status.ready { background-color: #d4edda; color: #155724; border-left: 5px solid #28a745; }
        .status.not-ready { background-color: #f8d7da; color: #721c24; border-left: 5px solid #dc3545; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f8f9fa; font-weight: bold; }
        tr:hover { background-color: #f5f5f5; }
        .timestamp { color: #666; font-size: 12px; margin-top: 20px; border-top: 1px solid #ddd; padding-top: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>✅ SR-IOV Cluster Health Check Report</h1>
        
        <div class="summary">
            <div class="summary-item total">
                Total<br>
EOF
    echo "                <span>$((PASS_COUNT + FAIL_COUNT + WARN_COUNT))</span>" >> /dev/stdout
    cat << 'EOF'
            </div>
            <div class="summary-item passed">
                Passed<br>
EOF
    echo "                <span>$PASS_COUNT ✅</span>" >> /dev/stdout
    cat << 'EOF'
            </div>
            <div class="summary-item failed">
                Failed<br>
EOF
    echo "                <span>$FAIL_COUNT ❌</span>" >> /dev/stdout
    cat << 'EOF'
            </div>
            <div class="summary-item warning">
                Warnings<br>
EOF
    echo "                <span>$WARN_COUNT ⚠️</span>" >> /dev/stdout
    cat << 'EOF'
            </div>
        </div>
        
        <div class="status EOF
    if [ $FAIL_COUNT -eq 0 ]; then
        echo 'ready' >> /dev/stdout
    else
        echo 'not-ready' >> /dev/stdout
    fi
    cat << 'EOF'
">
EOF
    if [ $FAIL_COUNT -eq 0 ]; then
        echo "✅ READY FOR FULL TEST SUITE EXECUTION" >> /dev/stdout
    else
        echo "❌ NOT READY FOR TESTING" >> /dev/stdout
    fi
    cat << 'EOF'
        </div>
        
        <h2>Cluster Information</h2>
EOF
    echo "        <p><strong>Timestamp:</strong> $(date '+%Y-%m-%d %H:%M:%S')</p>" >> /dev/stdout
    echo "        <p><strong>Current Context:</strong> $(oc config current-context 2>/dev/null || echo "Unknown")</p>" >> /dev/stdout
    cat << 'EOF'
        
        <div class="timestamp">
EOF
    echo "            Generated on $(date '+%Y-%m-%d %H:%M:%S')" >> /dev/stdout
    cat << 'EOF'
        </div>
    </div>
</body>
</html>
EOF
}

################################################################################
# MAIN EXECUTION
################################################################################

main() {
    parse_args "$@"
    
    # Ensure kubeconfig is valid
    if ! oc cluster-info > /dev/null 2>&1; then
        echo -e "${RED}Error: Unable to connect to cluster${NC}"
        echo "Please verify your kubeconfig and try again"
        exit 1
    fi
    
    # Run health checks
    run_all_checks
    local result=$?
    
    # Generate report
    echo ""
    case "$OUTPUT_FORMAT" in
        json)
            generate_json_report
            ;;
        html)
            generate_html_report
            ;;
        text|*)
            generate_text_report
            ;;
    esac
    
    # Exit with appropriate status
    exit $result
}

# Run main if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi

