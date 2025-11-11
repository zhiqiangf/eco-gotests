# SR-IOV Operator - Controller Architecture Analysis

**Purpose:** Technical deep-dive into the SriovNetwork controller reconciliation flow

---

## Expected Controller Flow

Based on the operator documentation and Kubernetes controller patterns, here's what SHOULD happen:

### Phase 1: Controller Registration

**File:** `cmd/manager/main.go` (or similar entry point)

```go
// Expected: SriovNetwork controller must be registered
if err := (&controllers.SriovNetworkReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    log.Error(err, "unable to create controller", "controller", "SriovNetwork")
    os.Exit(1)
}
```

**What to verify:**
- [ ] SriovNetworkReconciler exists in `controllers/` directory
- [ ] Controller is instantiated in main.go
- [ ] Controller's SetupWithManager is called
- [ ] If this is missing or commented out → **THIS IS THE BUG**

---

### Phase 2: SriovNetwork CRD Watched

**Expected behavior:**
- Manager sets up informer to watch `SriovNetwork` resources
- Any create/update/delete event triggers queue
- Queue sends work to reconciler

```go
// In SetupWithManager (should exist in SriovNetworkReconciler)
func (r *SriovNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&sriovnetworkv1.SriovNetwork{}).
        Complete(r)
}
```

**What to verify:**
- [ ] SriovNetworkReconciler has SetupWithManager method
- [ ] SetupWithManager calls `For(&sriovnetworkv1.SriovNetwork{})`
- [ ] If watch is not set up → **CRs won't trigger reconciliation**

---

### Phase 3: Reconciliation Triggered

**Expected behavior:**
- When SriovNetwork CR is created/updated
- Kubernetes API server stores it in etcd
- Operator's watch/informer detects change
- Reconcile() method is called

```go
// Expected Reconcile signature
func (r *SriovNetworkReconciler) Reconcile(
    ctx context.Context,
    req ctrl.Request,
) (ctrl.Result, error) {
    log.Info("Reconciling SriovNetwork", "name", req.Name, "namespace", req.Namespace)
    
    // 1. Fetch the SriovNetwork resource
    // 2. Validate it
    // 3. Create NetworkAttachmentDefinition
    // 4. Update status
    // 5. Return result
}
```

**What to verify:**
- [ ] Reconcile method exists on SriovNetworkReconciler
- [ ] Logs show reconciliation attempts
- [ ] If Reconcile is never called → **Watch not working**

---

### Phase 4: NetworkAttachmentDefinition Creation

**Expected behavior:**
During reconciliation, operator should:

```go
func (r *SriovNetworkReconciler) createNAD(
    ctx context.Context,
    sriovNet *sriovnetworkv1.SriovNetwork,
) error {
    // 1. Get target namespace from sriovNet.Spec.NetworkNamespace
    targetNs := sriovNet.Spec.NetworkNamespace
    
    // 2. Build NAD object
    nad := &nadv1.NetworkAttachmentDefinition{
        ObjectMeta: metav1.ObjectMeta{
            Name:      sriovNet.Name,
            Namespace: targetNs,
        },
        Spec: nadv1.NetworkAttachmentDefinitionSpec{
            Config: buildCNIConfig(sriovNet),
        },
    }
    
    // 3. Create NAD
    if err := r.Client.Create(ctx, nad); err != nil {
        log.Error(err, "failed to create NAD", "namespace", targetNs, "name", sriovNet.Name)
        // Update status to show error
        sriovNet.Status.State = "Failed"
        sriovNet.Status.Error = err.Error()
        r.Client.Status().Update(ctx, sriovNet)
        return err
    }
    
    // 4. Update status to show success
    sriovNet.Status.State = "Ready"
    r.Client.Status().Update(ctx, sriovNet)
    return nil
}
```

**What to verify:**
- [ ] Reconcile method attempts to create NAD
- [ ] Logs show NAD creation attempts (success or failure)
- [ ] SriovNetwork status is updated with result
- [ ] If no NAD creation attempts → **Reconciliation isn't progressing**

---

### Phase 5: Error Handling

**Expected behavior:**
All errors should be visible somewhere:

```go
// Option 1: Status Update (BEST PRACTICE)
// Error should be recorded in SriovNetwork.Status.Conditions

// Option 2: Operator Logs (MINIMUM)
// Error should appear in operator pod logs

// Option 3: Kubernetes Events (GOOD PRACTICE)
// Error event should be created in operator namespace
```

**What to verify:**
- [ ] SriovNetwork status shows error condition
- [ ] Operator logs contain error messages
- [ ] Kubernetes events show what went wrong
- [ ] If all three are missing → **Silent failure bug**

---

## Bug Detection: The Six-Point Checklist

### 1. Is the Controller Registered?

```bash
# Get operator pod
POD=$(oc get pods -n openshift-sriov-network-operator -l app=sriov-network-operator -o jsonpath='{.items[0].metadata.name}')

# Check for SetupWithManager logs
oc logs -n openshift-sriov-network-operator $POD | grep -i "setup"

# Look for "unable to create controller" errors
oc logs -n openshift-sriov-network-operator $POD | grep -i "controller.*sriovnetwork"
```

**Bug indicator:** No controller setup logs at startup

---

### 2. Is the Watch Running?

```bash
# Check if controller is watching SriovNetwork
oc logs -n openshift-sriov-network-operator $POD | grep -i "watch\|informer"

# Create a test SriovNetwork and check for any reaction
oc create -n openshift-sriov-network-operator -f test-sriovnetwork.yaml

# Check for reconciliation logs
oc logs -n openshift-sriov-network-operator $POD --tail=50 | grep -i "sriovnetwork\|reconcil"
```

**Bug indicator:** Creating SriovNetwork produces no logs

---

### 3. Does Reconciliation Start?

```bash
# Look for "Reconciling SriovNetwork" or similar log messages
oc logs -n openshift-sriov-network-operator $POD | grep -i "reconciling\|sriovnetwork"

# Check for work queue processing
oc logs -n openshift-sriov-network-operator $POD | grep -i "workqueue"

# Look for any mention of fetching SriovNetwork
oc logs -n openshift-sriov-network-operator $POD | grep -i "get.*sriovnetwork"
```

**Bug indicator:** No reconciliation start logs

---

### 4. Does NAD Creation Attempt Happen?

```bash
# Look for NetworkAttachmentDefinition creation logs
oc logs -n openshift-sriov-network-operator $POD | grep -i "nad\|networkattachmentdef\|create.*network"

# Look for any namespace-related errors
oc logs -n openshift-sriov-network-operator $POD | grep -i "namespace.*forbidden\|terminating"

# Look for RBAC errors
oc logs -n openshift-sriov-network-operator $POD | grep -i "forbidden\|rbac\|permission"
```

**Bug indicator:** No NAD creation attempts logged

---

### 5. Is Status Being Updated?

```bash
# Check if SriovNetwork status is being updated
oc get sriovnetwork -A -o wide

# Check status conditions
oc get sriovnetwork <name> -n <ns> -o yaml | grep -A 10 "status:"

# Look for any error conditions
oc describe sriovnetwork <name> -n <ns>
```

**Bug indicator:** Status remains empty/pending indefinitely

---

### 6. Are Errors Visible?

```bash
# Check for error events
oc get events -n openshift-sriov-network-operator | grep -i sriovnetwork

# Check for error status messages
oc get sriovnetwork -A -o jsonpath='{.items[*].status.error}' 2>/dev/null

# Check operator pod termination logs
oc logs -n openshift-sriov-network-operator $POD --previous 2>/dev/null | head -50
```

**Bug indicator:** No visibility into what's failing

---

## The Silent Failure Problem

The most insidious form of this bug is when:

```
1. Controller IS registered ✅
2. Watch IS running ✅
3. Reconciliation STARTS ✅
4. NAD creation attempt HAPPENS ✅
5. BUT: Request fails at RBAC layer ❌
6. AND: Error is not logged/updated ❌

Result: Bug appears to be in Phase 4 (NAD creation)
Actual: Bug is in error handling (Phase 5)
```

### Why This Happens

Kubernetes client-go library may suppress some errors in certain configurations:

```go
// Possible problematic code:
if err := r.Client.Create(ctx, nad); err != nil {
    // If this is commented out or uses log.V(4):
    log.V(4).Error(err, "failed to create NAD")  // ← Only visible at high verbosity!
}
```

**Verification:**
```bash
# Check operator log verbosity
oc get deployment sriov-network-operator -n openshift-sriov-network-operator -o yaml | grep -i "log-level\|v=\|verbosity"

# If using low verbosity, errors may not be visible
# Workaround: increase verbosity
oc set env deployment/sriov-network-operator \
  -n openshift-sriov-network-operator \
  OPERATOR_LOG_LEVEL=debug
```

---

## Admission Webhook Interference

The admission webhook can silently intercept and modify SriovNetwork processing:

### Expected Webhook Flow

```
SriovNetwork Created
    ↓
Admission Webhook Called (ValidatingAdmissionWebhook)
    ↓
    ├─→ Webhook denies request (Fail policy)
    │   └─→ User sees error immediately ✓
    │
    └─→ Webhook approves and possibly mutates
        ↓
        Stored in etcd ✓
        ↓
        Watch/Informer detects change
        ↓
        Reconciler called ✓
```

### Broken Webhook Flow

```
SriovNetwork Created
    ↓
Admission Webhook Called (ValidatingAdmissionWebhook)
    ↓
    ├─→ Webhook fails/times out (Ignore policy)
    │   └─→ Request is ACCEPTED anyway
    │   └─→ CR stored in etcd ✓
    │   └─→ BUT: Watch event may be suppressed
    │   └─→ OR: Watch event is delivered to dead handler
    │   └─→ Reconciler NEVER called ❌
    │
    └─→ Result: Silent failure - CR exists but isn't reconciled
```

**Verification:**
```bash
# Check webhook configuration
oc get validatingwebhookconfiguration -A | grep sriov

oc describe validatingwebhookconfiguration sriov-network-operator

# Look for this section:
# - failurePolicy: Ignore  (← This can cause silent failures!)
# - failurePolicy: Fail    (← This is safer)
```

---

## RBAC Permission Check

The operator needs these permissions for SriovNetwork reconciliation:

```yaml
# Required permissions
- apiGroups: ["sriovnetwork.openshift.io"]
  resources: ["sriovnetworks"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# For NAD creation
- apiGroups: ["k8s.cni.cncf.io"]
  resources: ["network-attachment-definitions"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# For status updates
- apiGroups: ["sriovnetwork.openshift.io"]
  resources: ["sriovnetworks/status"]
  verbs: ["get", "update", "patch"]
```

**Verification:**
```bash
# Get operator's service account
SA=$(oc get deployment sriov-network-operator -n openshift-sriov-network-operator \
  -o jsonpath='{.spec.template.spec.serviceAccountName}')

# Get associated role
oc get rolebinding -n openshift-sriov-network-operator | grep $SA
oc get clusterrolebinding | grep $SA

# Check actual permissions
oc describe clusterrole sriov-network-operator

# Test specific permission
oc auth can-i create network-attachment-definitions --as=system:serviceaccount:openshift-sriov-network-operator:$SA --all-namespaces
```

**Bug indicator:**
```
no - User "system:serviceaccount:openshift-sriov-network-operator:default" cannot create network-attachment-definitions
```

---

## Namespace Lifecycle Race Condition

### The Race Condition

```
Timeline:
0ms  - Test creates namespace "test-ns"
10ms - Test creates SriovNetwork CR
20ms - Operator starts reconciliation (fetch CR)
25ms - Test cleanup DELETES namespace "test-ns" ← RACE!
30ms - Operator tries to create NAD in "test-ns"
35ms - API Server rejects: "namespace is terminating"
40ms - NAD creation fails silently (error not propagated)
...
9600ms - Test timeout waiting for pods
```

### How to Detect

```bash
# 1. Check if namespace termination is happening during test
oc get namespace <test-ns> -w

# 2. Look for these error patterns in logs
oc logs -n openshift-sriov-network-operator $POD | grep -i "terminating\|forbidden"

# 3. Check namespace phase during testing
oc get namespace -A -o wide | grep -i "active\|terminating"

# 4. Check for pending finalizers
oc get namespace <test-ns> -o yaml | grep -A 10 "finalizers"
```

### Prevention

```bash
# Add finalizer to prevent premature termination
oc patch namespace <test-ns> --type json -p '[{"op":"add","path":"/metadata.finalizers","value":["test.finalizer"]}]'

# Or use explicit wait
oc wait --for=condition=NADReady sriovnetwork <name> -n <ns> --timeout=5m
```

---

## Debugging Commands Cheat Sheet

```bash
#!/bin/bash

NS="openshift-sriov-network-operator"

# 1. Get operator pod
POD=$(oc get pods -n $NS -l app=sriov-network-operator -o jsonpath='{.items[0].metadata.name}')
echo "Operator Pod: $POD"

# 2. Full operator logs (last 1000 lines)
echo "=== Operator Logs ==="
oc logs -n $NS $POD --tail=1000 | tail -100

# 3. Search for SriovNetwork reconciliation
echo "=== SriovNetwork Reconciliation Logs ==="
oc logs -n $NS $POD | grep -i "sriovnetwork"

# 4. Check for NAD creation attempts
echo "=== NAD Creation Attempts ==="
oc logs -n $NS $POD | grep -i "networkattachmentdef\|nad"

# 5. Check for errors
echo "=== Errors ==="
oc logs -n $NS $POD | grep -i "error\|failed"

# 6. Check webhook status
echo "=== Webhooks ==="
oc get validatingwebhookconfiguration -A | grep sriov
oc get mutatingwebhookconfiguration -A | grep sriov

# 7. Check RBAC permissions
echo "=== RBAC ==="
oc describe clusterrole sriov-network-operator | grep -A 10 "network-attachment"

# 8. Check SriovNetwork resources
echo "=== SriovNetwork Resources ==="
oc get sriovnetwork -A -o wide

# 9. Check if NADs were created
echo "=== NADs ==="
oc get networkattachmentdefinition -A | grep sriov

# 10. Check for events
echo "=== Events ==="
oc get events -n $NS -A | grep -i sriov
```

---

## Conclusion: Most Likely Bugs

Based on architecture analysis, the most likely bugs are:

| Likelihood | Bug | How to Detect | Fix |
|---|---|---|---|
| HIGH | Admission webhook rejecting CRs | Disable webhook and test | Update webhook, document unsupported HW |
| HIGH | RBAC missing NAD creation permission | Check auth can-i | Add permission to ClusterRole |
| MEDIUM | Namespace termination race condition | Check namespace phase | Add finalizer or better synchronization |
| MEDIUM | Controller not registered | Check startup logs | Fix main.go controller setup |
| LOW | Silent error handling | Enable debug logging | Update error handling |

---

## What to File as a Bug Report

When reporting to maintainers, include:

```markdown
## Issue: SriovNetwork CRs not being reconciled

**Symptoms:**
- [ ] SriovNetwork objects created successfully
- [ ] Operator pod is running
- [ ] No reconciliation logs appear in operator logs
- [ ] NetworkAttachmentDefinition not created
- [ ] SriovNetwork status not updated

**Evidence needed:**
1. Operator logs around SriovNetwork creation
2. Output of: `oc get sriovnetwork -A -o yaml`
3. Output of: `oc get networkattachmentdefinition -A -o yaml`
4. Output of: `oc auth can-i create network-attachment-definitions ...`
5. Output of: `oc get validatingwebhookconfiguration -o yaml`
6. Full operator pod logs: `oc logs deployment/sriov-network-operator -n openshift-sriov-network-operator --tail=1000`

**Reproduction steps:**
1. Install sriov-network-operator version X
2. Run these commands:
   ```bash
   # Create SriovNetwork
   oc create -f sriovnetwork.yaml
   # Check if NAD is created
   oc get networkattachmentdefinition
   ```
3. Observe: NAD not created
```

