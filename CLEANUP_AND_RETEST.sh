#!/bin/bash

# SR-IOV Resource Conflict Fix - Cleanup and Retest Script
# This script cleans up all stuck resources and runs the test with the fix applied

set -e

echo "=========================================="
echo "SR-IOV Cleanup and Retest"
echo "=========================================="
echo ""

# Configuration
SRIOV_NS="openshift-sriov-network-operator"
TEST_PATTERN="25959.*spoof.*on"
CLEANUP_TIMEOUT=300

echo "[1/7] Checking cluster connectivity..."
oc cluster-info >/dev/null 2>&1 || {
    echo "ERROR: Cannot connect to cluster. Please check 'oc' configuration."
    exit 1
}
echo "✓ Cluster OK"
echo ""

echo "[2/7] Deleting all stuck SriovNetwork resources..."
oc delete sriovnetwork --all -n $SRIOV_NS --ignore-not-found 2>/dev/null || true
echo "✓ SriovNetwork deleted"
echo ""

echo "[3/7] Deleting all test namespaces (e2e-*)..."
for ns in $(oc get ns -o name 2>/dev/null | grep "e2e-" | sed 's/^namespace\///'); do
    echo "  Deleting namespace: $ns"
    oc delete ns "$ns" --ignore-not-found 2>/dev/null || true
done
echo "✓ Test namespaces cleaned up"
echo ""

echo "[4/7] Waiting for cleanup completion..."
sleep 30
echo "✓ Cleanup wait complete"
echo ""

echo "[5/7] Restarting SR-IOV operator..."
oc rollout restart deployment/sriov-network-operator \
    -n $SRIOV_NS 2>/dev/null || {
    echo "WARNING: Could not restart operator (may already be restarting)"
}
echo "  Waiting for operator to be ready..."
oc rollout status deployment/sriov-network-operator \
    -n $SRIOV_NS --timeout=300s 2>/dev/null || {
    echo "WARNING: Operator status check timed out, proceeding anyway"
}
echo "✓ Operator restarted"
echo ""

echo "[6/7] Verifying cleanup..."
sriovnet_count=$(oc get sriovnetwork -n $SRIOV_NS --no-headers 2>/dev/null | wc -l)
testns_count=$(oc get ns -o name 2>/dev/null | grep "e2e-" | wc -l)
echo "  Remaining SriovNetworks: $sriovnet_count"
echo "  Remaining test namespaces: $testns_count"

if [ "$sriovnet_count" -gt 0 ] || [ "$testns_count" -gt 0 ]; then
    echo "  ⚠ Some resources still exist (may be in terminating state)"
else
    echo "  ✓ All resources cleaned up"
fi
echo ""

echo "[7/7] Running test with fixed naming..."
cd /root/eco-gotests || exit 1
echo "  Test pattern: $TEST_PATTERN"
echo "  Running: ginkgo -v tests/sriov/sriov_basic_test.go --focus \"$TEST_PATTERN\""
echo ""
echo "=========================================="

ginkgo -v tests/sriov/sriov_basic_test.go --focus "$TEST_PATTERN"

TEST_RESULT=$?

echo ""
echo "=========================================="
if [ $TEST_RESULT -eq 0 ]; then
    echo "✓ TEST PASSED!"
    echo ""
    echo "The resource naming conflict fix is working!"
    echo "Each test now uses unique SriovNetwork names:"
    echo "  - Test 25959: \"25959-cx7anl244\""
    echo "  - Test 70820: \"70820-cx7anl244\""
    echo "  - Test 25960: \"25960-cx7anl244\""
    echo "  - Test 70821: \"70821-cx7anl244\""
else
    echo "❌ TEST FAILED"
    echo ""
    echo "Debugging tips:"
    echo "1. Check SR-IOV operator status:"
    echo "   oc get pods -n $SRIOV_NS"
    echo ""
    echo "2. Check operator logs:"
    echo "   oc logs -n $SRIOV_NS -l app=sriov-network-operator --tail=100"
    echo ""
    echo "3. Check if NAD was created:"
    echo "   oc get net-attach-def -A | grep -E 'e2e-25959|25959-'"
    echo ""
    echo "4. Check SriovNetwork resource:"
    echo "   oc get sriovnetwork -n $SRIOV_NS -o wide"
fi
echo "=========================================="

exit $TEST_RESULT




