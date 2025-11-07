# What to Expect After the Update

## 🎯 The Main Change: Automatic Cleanup Before Tests

When you run the SRIOV tests, here's what will happen:

### Timeline

```
Start Test Run
    │
    ├─► BeforeSuite Hook Executes
    │   │
    │   ├─ [NEW] cleanupLeftoverResources()  ← AUTOMATIC CLEANUP
    │   │         └─ Finds & removes leftover namespaces
    │   │         └─ Finds & removes leftover networks
    │   │         └─ Frees up VF resources
    │   │
    │   ├─ Normal setup
    │   ├─ Verify SRIOV operator
    │   └─ Pull test images
    │
    ├─► All 9 Tests Run
    │   └─ Clean, fresh state
    │
    └─► Complete!
```

## 📋 Expected Log Output

When you run tests, you'll see these new log lines (if there are leftover resources):

```
STEP: BeforeSuite [setup]
STEP: Cleaning up leftover resources from previous test runs
STEP: Cleaning up leftover test namespaces from previous runs
"level"=0 "msg"="Removing leftover test namespace" "namespace"="e2e-25959-cx7anl244"
"level"=0 "msg"="Removing leftover test namespace" "namespace"="e2e-70821-cx7anl244"
STEP: Cleaning up leftover SR-IOV networks from previous runs
"level"=0 "msg"="Removing leftover SR-IOV network" "network"="25959-cx7anl244"
"level"=0 "msg"="Removing leftover SR-IOV network" "network"="70821-cx7anl244"
"level"=0 "msg"="Cleanup of leftover resources completed"
STEP: Creating test namespace with privileged labels
...
[PASSED] BeforeSuite [setup]
```

Or if there are no leftover resources:

```
STEP: BeforeSuite [setup]
STEP: Cleaning up leftover resources from previous test runs
STEP: Cleaning up leftover test namespaces from previous runs
STEP: Cleaning up leftover SR-IOV networks from previous runs
"level"=0 "msg"="Cleanup of leftover resources completed"
STEP: Creating test namespace with privileged labels
...
[PASSED] BeforeSuite [setup]
```

## 🔄 Practical Scenarios

### Scenario 1: Everything Works Perfectly
```
$ GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m

[sig-networking] SDN sriov-legacy
  ✓ BeforeSuite
  ✓ Test 25959: SR-IOV VF with spoof checking enabled
  ✓ Test 70820: SR-IOV VF with spoof checking disabled
  ✓ Test 25960: SR-IOV VF with trust disabled
  ✓ Test 70821: SR-IOV VF with trust enabled
  ... (all tests pass)

PASSED
```

### Scenario 2: You Interrupt Tests with Ctrl+C
```
$ GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m

[sig-networking] SDN sriov-legacy
  ✓ BeforeSuite
  ✓ Test 25959: PASSED
  ✓ Test 70820: PASSED
  ○ Test 25960: Running...
    ^ User presses Ctrl+C here

Test interrupted! Leftover resources remain:
  - e2e-25959-cx7anl244 namespace
  - e2e-70820-e810xxv namespace
  - 25959-cx7anl244 network
  - 70820-e810xxv network
  - VF resources still allocated on nodes
```

### Scenario 3: You Run Tests Again Immediately
```
$ GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m

[sig-networking] SDN sriov-legacy
  STEP: BeforeSuite [setup]
  STEP: Cleaning up leftover resources from previous test runs
  "msg"="Removing leftover test namespace" "namespace"="e2e-25959-cx7anl244"
  "msg"="Removing leftover test namespace" "namespace"="e2e-70820-e810xxv"
  "msg"="Removing leftover SR-IOV network" "network"="25959-cx7anl244"
  "msg"="Removing leftover SR-IOV network" "network"="70820-e810xxv"
  "msg"="Cleanup of leftover resources completed"
  
  ✓ BeforeSuite  (with auto cleanup!)
  ✓ Test 25959: PASSED
  ✓ Test 70820: PASSED
  ✓ Test 25960: PASSED (this time it completes!)
  ... (all tests pass)

PASSED  ✓
```

### Scenario 4: Multiple Ctrl+C Interruptions
```
Run 1: Interrupt after test 2 (leaves resources)
  └─ e2e-25959-cx7anl244, e2e-70820-e810xxv

Run 2: Interrupt after test 5 (leaves more resources)
  └─ e2e-25959-cx7anl244, e2e-70820-e810xxv (still there)
  └─ e2e-25963-e810xxv, e2e-25961-cx7anl244 (new ones)

Run 3: Clean start - cleanup finds ALL 4 leftover namespaces
  STEP: Cleaning up leftover resources...
  "msg"="Removing leftover test namespace" "namespace"="e2e-25959-cx7anl244"
  "msg"="Removing leftover test namespace" "namespace"="e2e-70820-e810xxv"
  "msg"="Removing leftover test namespace" "namespace"="e2e-25963-e810xxv"
  "msg"="Removing leftover test namespace" "namespace"="e2e-25961-cx7anl244"
  └─ All cleaned up!

  ✓ Tests run successfully
```

## ⚡ Timing Expectations

| Scenario | Time |
|----------|------|
| **First test run (clean)** | ~40-50 minutes |
| **Cleanup only (no leftover)** | ~10-15 seconds |
| **Cleanup 1 namespace** | ~30-60 seconds |
| **Cleanup 4 namespaces** | ~2-4 minutes |
| **Cleanup with force delete** | +30-60 seconds per namespace |
| **Full cleanup + tests** | ~42-54 minutes (includes cleanup time) |

## 🔍 How to Verify It's Working

### Check 1: Look for cleanup log messages
```bash
# Run tests and grep for cleanup messages
GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m 2>&1 | grep -i "cleanup\|removing"
```

Expected output (if leftover resources exist):
```
Cleaning up leftover resources
Removing leftover test namespace: e2e-25959-cx7anl244
Removing leftover SR-IOV network: 25959-cx7anl244
Cleanup of leftover resources completed
```

### Check 2: Interrupt tests and run again
```bash
# Run test
GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m

# Press Ctrl+C to interrupt

# Run again immediately
GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m

# Should complete cleanup and then pass
```

### Check 3: Verify no "Insufficient resources" error
```bash
# If you see this error, cleanup didn't work:
# "Insufficient openshift.io/cx7anl244 resources"

# If you DON'T see this error, cleanup worked! ✅
```

### Check 4: Verify resources are freed
```bash
# Check if test namespaces exist
oc get ns | grep "e2e-"

# After cleanup, should return nothing (no output)
```

## 🎯 Success Indicators

You'll know the cleanup is working when:

✅ **No "Insufficient resources" errors** after interrupting tests  
✅ **Leftover namespaces automatically deleted** on next run  
✅ **Cleanup log messages appear** in test output  
✅ **Tests run successfully** even after interruptions  
✅ **VF resources freed** for next test run  

## ⚠️ What If Cleanup Fails?

### Scenario: Cleanup shows errors
```
"msg"="Failed to delete leftover namespace" "namespace"="e2e-25959" "error"="timeout"
"msg"="Cleanup of leftover resources completed"
```

**What this means:** Namespace deletion timed out, but cleanup continued  
**What happens:** Tests may fail with resource errors  
**Solution:** Manual cleanup needed

```bash
# Check stuck namespaces
oc get ns | grep e2e-

# Manually delete
oc delete namespace e2e-25959-cx7anl244 --grace-period=0 --force

# Retry tests
GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m
```

### Scenario: API error during cleanup
```
"msg"="Failed to list namespaces for cleanup" "error"="connection refused"
"msg"="Cleanup of leftover resources completed"
```

**What this means:** Could not connect to cluster  
**What happens:** Cleanup skipped, tests run without cleanup  
**Solution:** Verify kubeconfig is set up correctly

```bash
export KUBECONFIG=/root/dev-scripts/ocp/sriov/auth/kubeconfig
GOTOOLCHAIN=auto go test ./tests/sriov/... -v -ginkgo.v -timeout 60m
```

## 📊 Before vs After Comparison

### Before This Update
```
Test interrupted (Ctrl+C)
    │
    ├─ Leftover namespace: e2e-25959-cx7anl244
    ├─ Leftover namespace: e2e-70821-cx7anl244
    ├─ Leftover networks: 25959-cx7anl244, 70821-cx7anl244
    └─ VF resources: still allocated
    
Next run:
    │
    ├─ ❌ Error: "Insufficient openshift.io/cx7anl244"
    ├─ ❌ Pod stuck in Pending
    ├─ ❌ Need manual cleanup
    └─ ❌ Manual commands:
        oc delete namespace e2e-25959-cx7anl244
        oc delete namespace e2e-70821-cx7anl244
        oc delete sriovnetwork 25959-cx7anl244 -n openshift-sriov-network-operator
        oc delete sriovnetwork 70821-cx7anl244 -n openshift-sriov-network-operator
        (wait 2-3 minutes...)
    
    ✅ Finally works (after cleanup)
```

### After This Update
```
Test interrupted (Ctrl+C)
    │
    ├─ Leftover namespace: e2e-25959-cx7anl244
    ├─ Leftover namespace: e2e-70821-cx7anl244
    ├─ Leftover networks: 25959-cx7anl244, 70821-cx7anl244
    └─ VF resources: still allocated
    
Next run:
    │
    ├─ ✅ Cleanup runs automatically
    ├─ Finds leftover namespaces
    ├─ Finds leftover networks
    ├─ Deletes all resources
    ├─ Frees VF resources
    └─ ✅ Tests run successfully!
```

## 🚀 What to Do Now

1. **Just use it!** Cleanup is automatic
2. **Run tests normally** - cleanup happens transparently
3. **Feel free to interrupt** - cleanup handles it
4. **Check logs** - cleanup messages appear in test output
5. **Enjoy!** No more manual cleanup 🎉

## 📝 Summary

| Aspect | Details |
|--------|---------|
| **What changed** | Automatic cleanup before tests run |
| **How to use** | Nothing! Automatic, just run tests |
| **When it runs** | Before any tests in BeforeSuite |
| **What it cleans** | Test namespaces + SR-IOV networks |
| **Cleanup time** | 2-4 minutes for full cleanup |
| **Success indicator** | Logs show "Cleanup of leftover resources completed" |
| **Failure handling** | Graceful - cleanup errors don't block tests |
| **Benefits** | No manual cleanup needed after interruptions |

**Bottom line:** Run your tests as normal. If interrupted, just run again. Cleanup happens automatically! 🎯


